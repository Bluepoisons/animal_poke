// Package handlers 隐私、安全报告、审计查询、商业化。
package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/models"
	"animalpoke/backend/internal/repo"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PrivacyHandler 授权/导出/删除。
type PrivacyHandler struct {
	deviceRepo     *repo.DeviceRepo
	animalRepo     *repo.AnimalRepo
	inferenceRepo  *repo.InferenceRepo
	auditRepo      *repo.AuditLogRepo
	accountRepo    *repo.AccountRepo
	db             *gorm.DB
	consentVersion string
}

// NewPrivacyHandler 构造。
func NewPrivacyHandler(db *gorm.DB, device *repo.DeviceRepo, animal *repo.AnimalRepo, inf *repo.InferenceRepo, audit *repo.AuditLogRepo) *PrivacyHandler {
	return NewPrivacyHandlerFull(db, device, animal, inf, audit, nil)
}

// NewPrivacyHandlerFull 构造（含账号级删除/导出所需 AccountRepo）。
func NewPrivacyHandlerFull(db *gorm.DB, device *repo.DeviceRepo, animal *repo.AnimalRepo, inf *repo.InferenceRepo, audit *repo.AuditLogRepo, account *repo.AccountRepo) *PrivacyHandler {
	return &PrivacyHandler{
		deviceRepo: device, animalRepo: animal, inferenceRepo: inf, auditRepo: audit, accountRepo: account, db: db,
		consentVersion: "v1",
	}
}

type consentRequest struct {
	Version string `json:"version" binding:"required"`
	Scope   string `json:"scope"`
	Revoke  bool   `json:"revoke"`
}

// deleteDataRequest AP-077：scope=device（默认）|account；账号级需 reauth_password 或 reauth_token（AP-079）。
type deleteDataRequest struct {
	Scope          string `json:"scope"` // device|account
	ReauthPassword string `json:"reauth_password"`
	ReauthToken    string `json:"reauth_token"` // 短期 reauth 令牌（AP-079）
	Confirm        string `json:"confirm"`      // account 删除需 "DELETE"
}

type exportDataRequest struct {
	Scope string `json:"scope"` // device|account
}

var allowedConsentScopes = map[string]struct{}{
	"photo": {}, "location": {}, "precise_location": {},
}

func normalizeConsentScope(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "photo,location", nil
	}
	parts := strings.Split(raw, ",")
	seen := map[string]struct{}{}
	var out []string
	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s == "" {
			continue
		}
		if _, ok := allowedConsentScopes[s]; !ok {
			return "", errInvalidScope
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	if len(out) == 0 {
		return "photo,location", nil
	}
	return strings.Join(out, ","), nil
}

var errInvalidScope = errScope{}

type errScope struct{}

func (errScope) Error() string { return "invalid scope" }

// PutConsent POST /privacy/consent
func (h *PrivacyHandler) PutConsent(c *gin.Context) {
	var req consentRequest
	if err := middleware.BindStrictJSON(c, &req); err != nil {
		middleware.WriteBindError(c, err)
		return
	}
	if h.consentVersion != "" && req.Version != h.consentVersion {
		// 仅接受当前服务端版本；升级时客户端需重弹
		middleware.WriteError(c, http.StatusBadRequest, "unsupported_consent_version", "unsupported consent version", false, map[string]any{
			"required_version": h.consentVersion,
		})
		return
	}
	deviceID := middleware.GetDeviceID(c)
	scope, err := normalizeConsentScope(req.Scope)
	if err != nil {
		middleware.WriteError(c, http.StatusBadRequest, "invalid_scope", "invalid scope", false, map[string]any{
			"allowed": []string{"photo", "location", "precise_location"},
		})
		return
	}
	if err := h.deviceRepo.UpdateConsent(deviceID, req.Version, scope, req.Revoke); err != nil {
		// DB 不可用 fail-closed
		middleware.WriteError(c, http.StatusServiceUnavailable, "db_unavailable", "update failed", true, nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "version": req.Version, "scope": scope, "revoked": req.Revoke})
}

// ExportData POST /privacy/export
// 默认完整导出（分页循环 ListByDevice 直至空）；可选 ?cursor=<after_id> 仅返回一页便于大包分片。
// 导出始终脱敏精确坐标；安全报告仅元数据（不含 payload 密钥/原文）。
func (h *PrivacyHandler) ExportData(c *gin.Context) {
	deviceID := middleware.GetDeviceID(c)
	accountID := middleware.GetAccountID(c)
	scope := "device"
	if c.Request != nil && c.Request.Body != nil && c.Request.ContentLength != 0 {
		var req exportDataRequest
		if err := middleware.BindStrictJSON(c, &req); err != nil {
			middleware.WriteBindError(c, err)
			return
		}
		if requested := strings.ToLower(strings.TrimSpace(req.Scope)); requested != "" {
			scope = requested
		}
	}
	if scope != "device" && scope != "account" {
		middleware.WriteError(c, http.StatusBadRequest, "bad_request", "invalid scope", false, nil)
		return
	}
	if scope == "account" && accountID == "" {
		middleware.WriteError(c, http.StatusForbidden, "account_required", "account required", false, nil)
		return
	}

	pageOnly := false
	var afterID uint
	if v := strings.TrimSpace(c.Query("cursor")); v != "" {
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			middleware.WriteError(c, http.StatusBadRequest, "invalid_cursor", "invalid cursor", false, nil)
			return
		}
		afterID = uint(n)
		pageOnly = true
	}

	reqID := uuid.NewString()
	dr := models.DataRequest{
		RequestID: reqID, DeviceID: deviceID, Type: "export", Status: "processing",
		RequestedAt: time.Now().UTC(),
	}
	if err := h.db.Create(&dr).Error; err != nil {
		middleware.WriteError(c, http.StatusInternalServerError, "create_request_failed", "create request failed", true, nil)
		return
	}

	var activeDevices []models.Device
	if scope == "account" {
		var err error
		activeDevices, err = listActiveAccountDevices(h.db, accountID)
		if err != nil {
			h.failExport(c, &dr, "export devices failed", err)
			return
		}
	} else if dev, err := h.deviceRepo.Find(deviceID); err == nil {
		activeDevices = []models.Device{*dev}
	}
	deviceIDs := make([]string, 0, len(activeDevices))
	for _, device := range activeDevices {
		deviceIDs = append(deviceIDs, device.DeviceID)
	}

	animalQuery := h.db.Model(&models.Animal{}).Where("deleted_at IS NULL")
	if scope == "account" {
		animalQuery = animalQuery.Where("account_id = ?", accountID)
	} else {
		animalQuery = scopeRowsByDeviceAndAccount(animalQuery, deviceID, accountID)
	}
	animals, nextCursor, err := collectExportAnimals(animalQuery, afterID, pageOnly)
	if err != nil {
		now := time.Now().UTC()
		_ = h.db.Model(&dr).Updates(map[string]interface{}{
			"status": "failed", "error_msg": err.Error(), "completed_at": now,
		})
		middleware.WriteError(c, http.StatusInternalServerError, "export_failed", "export animals failed", true, nil)
		return
	}

	// 脱敏精确坐标
	for i := range animals {
		animals[i].PreciseLat = nil
		animals[i].PreciseLng = nil
		animals[i].PreciseExpiresAt = nil
	}

	dev, _ := h.deviceRepo.Find(deviceID)
	consent := gin.H{}
	if dev != nil {
		consent = gin.H{
			"version":    dev.ConsentVersion,
			"scope":      dev.ConsentScope,
			"consent_at": dev.ConsentAt,
			"revoked_at": dev.ConsentRevoked,
		}
	}

	// 安全报告：仅 count + 元数据，不含 payload
	var secRows []models.SecurityReport
	securityQuery := h.db.Select("report_id", "risk_score", "created_at")
	if scope == "account" {
		securityQuery = whereDeviceIDs(securityQuery, deviceIDs)
	} else {
		securityQuery = securityQuery.Where("device_id = ?", deviceID)
	}
	if err := securityQuery.Order("id asc").Find(&secRows).Error; err != nil {
		h.failExport(c, &dr, "export security reports failed", err)
		return
	}
	secMeta := make([]gin.H, 0, len(secRows))
	for _, s := range secRows {
		secMeta = append(secMeta, gin.H{
			"report_id":  s.ReportID,
			"risk_score": s.RiskScore,
			"created_at": s.CreatedAt,
		})
	}

	// data_requests 历史：不含 payload，避免循环嵌套与体积膨胀
	var reqRows []models.DataRequest
	requestQuery := h.db.Select("request_id", "type", "status", "requested_at", "completed_at", "created_at")
	if scope == "account" {
		requestQuery = whereDeviceIDs(requestQuery, deviceIDs)
	} else {
		requestQuery = requestQuery.Where("device_id = ?", deviceID)
	}
	if err := requestQuery.Order("id asc").Find(&reqRows).Error; err != nil {
		h.failExport(c, &dr, "export data requests failed", err)
		return
	}
	reqHist := make([]gin.H, 0, len(reqRows))
	for _, r := range reqRows {
		reqHist = append(reqHist, gin.H{
			"request_id":   r.RequestID,
			"type":         r.Type,
			"status":       r.Status,
			"requested_at": r.RequestedAt,
			"completed_at": r.CompletedAt,
			"created_at":   r.CreatedAt,
		})
	}

	orderQuery := h.db.Model(&models.Order{})
	entitlementQuery := h.db.Model(&models.Entitlement{})
	if scope == "account" {
		orderQuery = orderQuery.Where("account_id = ?", accountID)
		entitlementQuery = entitlementQuery.Where("account_id = ?", accountID)
	} else {
		orderQuery = scopeRowsByDeviceAndAccount(orderQuery, deviceID, accountID)
		entitlementQuery = scopeRowsByDeviceAndAccount(entitlementQuery, deviceID, accountID)
	}
	orders, err := collectExportRows[models.Order](orderQuery)
	if err != nil {
		h.failExport(c, &dr, "export orders failed", err)
		return
	}
	entitlements, err := collectExportRows[models.Entitlement](entitlementQuery)
	if err != nil {
		h.failExport(c, &dr, "export entitlements failed", err)
		return
	}
	adventureQuery := h.db.Model(&models.AdventureRun{})
	if scope == "account" {
		adventureQuery = adventureQuery.Where("account_id = ? OR owner_key = ?", accountID, repo.OwnerKey(accountID, ""))
	} else {
		adventureQuery = scopeRowsByDeviceAndAccount(adventureQuery, deviceID, accountID)
	}
	adventureRows, err := collectExportRows[models.AdventureRun](adventureQuery)
	if err != nil {
		now := time.Now().UTC()
		_ = h.db.Model(&dr).Updates(map[string]interface{}{
			"status": "failed", "error_msg": err.Error(), "completed_at": now,
		})
		middleware.WriteError(c, http.StatusInternalServerError, "export_failed", "export adventures failed", true, nil)
		return
	}
	adventures := make([]gin.H, 0, len(adventureRows))
	for _, run := range adventureRows {
		adventures = append(adventures, gin.H{
			"run_id":             run.RunID,
			"animal_uuid":        run.AnimalUUID,
			"theme":              run.Theme,
			"title":              run.Title,
			"status":             run.Status,
			"story":              json.RawMessage(run.ResultJSON),
			"selected_choice_id": run.SelectedChoiceID,
			"outcome":            run.Outcome,
			"souvenir_name":      run.SouvenirName,
			"bond_delta":         run.BondDelta,
			"created_at":         run.CreatedAt,
			"completed_at":       run.CompletedAt,
		})
	}

	ownerKey := repo.OwnerKey("", deviceID)
	eventQuery := h.db.Model(&models.GrowthEvent{})
	auditQuery := h.db.Model(&models.GrowthResetAudit{})
	if scope == "account" {
		ownerKey = repo.OwnerKey(accountID, "")
		eventQuery = eventQuery.Where("account_id = ? OR owner_key = ?", accountID, ownerKey)
		auditQuery = auditQuery.Where("account_id = ? OR owner_key = ?", accountID, ownerKey)
	} else {
		eventQuery = scopeRowsByDeviceAndAccount(eventQuery, deviceID, accountID)
		auditQuery = scopeRowsByDeviceAndAccount(auditQuery, deviceID, accountID)
	}
	growthEvents, err := collectExportRows[models.GrowthEvent](eventQuery)
	if err != nil {
		h.failExport(c, &dr, "export growth events failed", err)
		return
	}
	researcherTracks, err := collectExportRows[models.ResearcherTrack](h.db.Model(&models.ResearcherTrack{}).Where("owner_key = ?", ownerKey))
	if err != nil {
		h.failExport(c, &dr, "export researcher tracks failed", err)
		return
	}
	companions, err := collectExportRows[models.CompanionProfile](h.db.Model(&models.CompanionProfile{}).Where("owner_key = ?", ownerKey))
	if err != nil {
		h.failExport(c, &dr, "export companion profiles failed", err)
		return
	}
	companionNodes, err := collectExportRows[models.CompanionMemoryNode](h.db.Model(&models.CompanionMemoryNode{}).Where("owner_key = ?", ownerKey))
	if err != nil {
		h.failExport(c, &dr, "export companion nodes failed", err)
		return
	}
	growthAudits, err := collectExportRows[models.GrowthResetAudit](auditQuery)
	if err != nil {
		h.failExport(c, &dr, "export growth reset audits failed", err)
		return
	}

	tokenVersion := 0
	disabled := false
	var createdAt interface{}
	if dev != nil {
		tokenVersion = dev.TokenVersion
		disabled = dev.Disabled
		createdAt = dev.CreatedAt
	}

	deviceItems := make([]gin.H, 0, len(activeDevices))
	for _, device := range activeDevices {
		deviceItems = append(deviceItems, gin.H{
			"device_id":     device.DeviceID,
			"token_version": device.TokenVersion,
			"disabled":      device.Disabled,
			"created_at":    device.CreatedAt,
			"consent": gin.H{
				"version":    device.ConsentVersion,
				"scope":      device.ConsentScope,
				"consent_at": device.ConsentAt,
				"revoked_at": device.ConsentRevoked,
			},
		})
	}

	payloadObj := gin.H{
		"scope": scope,
		"device": gin.H{
			"device_id":     deviceID,
			"token_version": tokenVersion,
			"disabled":      disabled,
			"created_at":    createdAt,
		},
		"consent": consent,
		"animals": animals,
		"security_reports": gin.H{
			"count": len(secRows),
			"items": secMeta,
		},
		"data_requests": reqHist,
		"orders":        orders,
		"entitlements":  entitlements,
		"adventures":    adventures,
		"growth": gin.H{
			"events":            growthEvents,
			"researcher_tracks": researcherTracks,
			"companions":        companions,
			"companion_nodes":   companionNodes,
			"reset_audits":      growthAudits,
		},
	}
	if scope == "account" {
		payloadObj["devices"] = deviceItems
	}
	if pageOnly {
		payloadObj["next_cursor"] = nextCursor
		payloadObj["page_only"] = true
	}

	payload, _ := json.Marshal(payloadObj)
	now := time.Now().UTC()
	_ = h.db.Model(&dr).Updates(map[string]interface{}{
		"status": "completed", "payload": string(payload), "completed_at": now,
	})
	c.JSON(http.StatusOK, gin.H{"request_id": reqID, "status": "completed", "data": json.RawMessage(payload)})
}

// collectExportAnimals 分页拉取动物；pageOnly 时只取一页并返回 next_cursor（0 表示无更多）。
func collectExportAnimals(query *gorm.DB, afterID uint, pageOnly bool) ([]models.Animal, uint, error) {
	const pageSize = 200
	var all []models.Animal
	cur := afterID
	for {
		var batch []models.Animal
		q := query
		if cur > 0 {
			q = q.Where("id > ?", cur)
		}
		if err := q.Order("id asc").Limit(pageSize).Find(&batch).Error; err != nil {
			return nil, 0, err
		}
		if len(batch) == 0 {
			return all, 0, nil
		}
		all = append(all, batch...)
		cur = batch[len(batch)-1].ID
		if pageOnly {
			next := uint(0)
			if len(batch) == pageSize {
				next = cur
			}
			return all, next, nil
		}
		if len(batch) < pageSize {
			return all, 0, nil
		}
	}
}

func scopeRowsByDeviceAndAccount(query *gorm.DB, deviceID, accountID string) *gorm.DB {
	query = query.Where("device_id = ?", deviceID)
	if strings.TrimSpace(accountID) == "" {
		return query.Where("account_id = '' OR account_id IS NULL")
	}
	return query.Where("account_id = ? OR account_id = '' OR account_id IS NULL", accountID)
}

func whereDeviceIDs(query *gorm.DB, deviceIDs []string) *gorm.DB {
	if len(deviceIDs) == 0 {
		return query.Where("1 = 0")
	}
	return query.Where("device_id IN ?", deviceIDs)
}

func listActiveAccountDevices(db *gorm.DB, accountID string) ([]models.Device, error) {
	var devices []models.Device
	err := db.Model(&models.Device{}).
		Joins("JOIN device_accounts ON device_accounts.device_id = devices.device_id").
		Where("device_accounts.account_id = ? AND device_accounts.status = ? AND devices.account_id = ?", accountID, "active", accountID).
		Order("devices.id asc").
		Find(&devices).Error
	return devices, err
}

func collectExportRows[T any](query *gorm.DB) ([]T, error) {
	const batchSize = 200
	all := make([]T, 0)
	batch := make([]T, 0, batchSize)
	err := query.Order("id asc").FindInBatches(&batch, batchSize, func(_ *gorm.DB, _ int) error {
		all = append(all, batch...)
		return nil
	}).Error
	return all, err
}

func (h *PrivacyHandler) failExport(c *gin.Context, request *models.DataRequest, message string, err error) {
	now := time.Now().UTC()
	_ = h.db.Model(request).Updates(map[string]interface{}{
		"status": "failed", "error_msg": err.Error(), "completed_at": now,
	})
	middleware.WriteError(c, http.StatusInternalServerError, "export_failed", message, true, nil)
}

// DeleteData POST /privacy/delete
// scope=device（默认）：仅本设备数据。
// scope=account：注销整个账号（AP-077），要求已绑定账号 + reauth_password + confirm=DELETE；
// 覆盖账号下所有设备、绑定、收藏、报告；订单法律保留并匿名化 account/device 关联；完成后撤销全设备 token。
func (h *PrivacyHandler) DeleteData(c *gin.Context) {
	deviceID := middleware.GetDeviceID(c)
	var req deleteDataRequest
	_ = c.ShouldBindJSON(&req) // body optional for backward compat
	scope := strings.ToLower(strings.TrimSpace(req.Scope))
	if scope == "" {
		scope = "device"
	}
	if scope != "device" && scope != "account" {
		middleware.WriteError(c, http.StatusBadRequest, "bad_request", "invalid scope", false, nil)
		return
	}

	reqID := uuid.NewString()
	dr := models.DataRequest{
		RequestID: reqID, DeviceID: deviceID, Type: "delete", Status: "processing",
		RequestedAt: time.Now().UTC(),
	}
	if err := h.db.Create(&dr).Error; err != nil {
		middleware.WriteError(c, http.StatusInternalServerError, "create_request_failed", "create request failed", true, nil)
		return
	}

	var err error
	if scope == "account" {
		err = h.deleteAccountScope(c, deviceID, req)
	} else {
		err = h.deleteDeviceScope(deviceID, middleware.GetAccountID(c))
	}

	now := time.Now().UTC()
	status := "completed"
	errMsg := ""
	if err != nil {
		status = "failed"
		errMsg = err.Error()
		slog.Error("删除失败", "err", err, "scope", scope)
	}
	updates := map[string]interface{}{"status": status, "completed_at": now}
	if errMsg != "" {
		updates["error_msg"] = errMsg
	}
	_ = h.db.Model(&dr).Updates(updates)

	if err != nil {
		// map known errors
		msg := err.Error()
		code := http.StatusInternalServerError
		reason := "delete_failed"
		retryable := true
		if strings.Contains(msg, "reauth") || strings.Contains(msg, "confirm") || strings.Contains(msg, "account required") {
			code = http.StatusForbidden
			reason = "reauth_required"
			retryable = false
		}
		middleware.WriteError(c, code, reason, msg, retryable, map[string]any{
			"status":     status,
			"request_id": reqID,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"request_id": reqID, "status": status, "scope": scope})
}

func (h *PrivacyHandler) deleteDeviceScope(deviceID, accountID string) error {
	return h.db.Transaction(func(tx *gorm.DB) error {
		if err := deleteGeneratedDataByDevice(tx, deviceID, accountID); err != nil {
			return err
		}
		if err := softDeleteAnimalsByDeviceScope(tx, deviceID, accountID); err != nil {
			return err
		}
		ar := h.animalRepo.WithTx(tx)
		if err := ar.ClearExpiredPreciseLocation(time.Now().UTC()); err != nil {
			return err
		}
		if h.inferenceRepo != nil {
			if err := h.inferenceRepo.WithTx(tx).SoftDeleteByDevice(deviceID); err != nil {
				return err
			}
		}
		if err := tx.Where("device_id = ?", deviceID).Delete(&models.SecurityReport{}).Error; err != nil {
			return err
		}
		if err := tx.Where("device_id = ?", deviceID).Delete(&models.ModerationReport{}).Error; err != nil {
			// table may be empty / ok
			_ = err
		}
		if err := tx.Model(&models.DataRequest{}).
			Where("device_id = ? AND type = ?", deviceID, "export").
			Updates(map[string]interface{}{"payload": ""}).Error; err != nil {
			return err
		}
		entitlementQuery := scopeRowsByDeviceAndAccount(tx.Model(&models.Entitlement{}), deviceID, accountID)
		if err := entitlementQuery.Update("active", false).Error; err != nil {
			return err
		}
		drRepo := h.deviceRepo.WithTx(tx)
		if err := drRepo.UpdateConsent(deviceID, "", "", true); err != nil {
			return err
		}
		// 吊销 refresh family（AP-078）
		refreshQuery := tx.Model(&models.RefreshToken{}).
			Where("device_id = ? AND status IN ?", deviceID, []string{"active", "rotated"})
		if accountID != "" {
			refreshQuery = refreshQuery.Where("account_id = ?", accountID)
		}
		if err := refreshQuery.
			Updates(map[string]interface{}{
				"status":     "revoked",
				"revoked_at": time.Now().UTC(),
			}).Error; err != nil {
			return err
		}
		bindingQuery := tx.Model(&models.DeviceAccount{}).Where("device_id = ?", deviceID)
		if accountID != "" {
			bindingQuery = bindingQuery.Where("account_id = ?", accountID)
		}
		if err := bindingQuery.
			Updates(map[string]interface{}{
				"refresh_token_hash": "",
				"refresh_expires_at": nil,
			}).Error; err != nil {
			return err
		}
		// 吊销已有 Token（使旧 JWT 失效）；不 Disable 设备以便用户可重新授权注册。
		if err := drRepo.BumpTokenVersion(deviceID); err != nil {
			return err
		}
		return nil
	})
}

func deleteGeneratedDataByDevice(tx *gorm.DB, deviceID, accountID string) error {
	deviceOwner := repo.OwnerKey("", deviceID)
	if err := tx.Where("owner_key = ?", deviceOwner).Delete(&models.CompanionMemoryNode{}).Error; err != nil {
		return err
	}
	for _, model := range []any{&models.ResearcherTrack{}, &models.CompanionProfile{}} {
		if err := tx.Where("owner_key = ?", deviceOwner).Delete(model).Error; err != nil {
			return err
		}
	}
	for _, model := range []any{&models.AdventureRun{}, &models.GrowthEvent{}, &models.GrowthResetAudit{}} {
		if err := scopeRowsByDeviceAndAccount(tx, deviceID, accountID).Delete(model).Error; err != nil {
			return err
		}
	}
	if err := tx.Where("device_id = ?", deviceID).Delete(&models.IdempotencyRecord{}).Error; err != nil {
		return err
	}
	return nil
}

func softDeleteAnimalsByDeviceScope(tx *gorm.DB, deviceID, accountID string) error {
	now := time.Now().UTC()
	return scopeRowsByDeviceAndAccount(tx.Model(&models.Animal{}), deviceID, accountID).
		Where("deleted_at IS NULL").
		Updates(map[string]interface{}{
			"deleted_at":         now,
			"server_version":     gorm.Expr("? + id", now.UnixNano()),
			"precise_lat":        nil,
			"precise_lng":        nil,
			"precise_expires_at": nil,
		}).Error
}

func deleteGeneratedDataByAccount(tx *gorm.DB, accountID string) error {
	ownerKey := repo.OwnerKey(accountID, "")
	if err := tx.Where("owner_key = ?", ownerKey).Delete(&models.CompanionMemoryNode{}).Error; err != nil {
		return err
	}
	if err := tx.Where("account_id = ? OR owner_key = ?", accountID, ownerKey).Delete(&models.AdventureRun{}).Error; err != nil {
		return err
	}
	for _, model := range []any{
		&models.GrowthEvent{},
		&models.ResearcherTrack{},
		&models.CompanionProfile{},
		&models.GrowthResetAudit{},
	} {
		if err := tx.Where("account_id = ? OR owner_key = ?", accountID, ownerKey).Delete(model).Error; err != nil {
			return err
		}
	}
	return nil
}

func (h *PrivacyHandler) deleteAccountScope(c *gin.Context, deviceID string, req deleteDataRequest) error {
	if h.accountRepo == nil {
		return errPrivacy("account repo unavailable")
	}
	if strings.TrimSpace(req.Confirm) != "DELETE" {
		return errPrivacy("confirm must be DELETE")
	}
	dev, err := h.deviceRepo.Find(deviceID)
	if err != nil || dev.AccountID == "" {
		return errPrivacy("account required")
	}
	accountID := dev.AccountID
	// re-auth: 已验证邮箱密码 或 短期 reauth_token（AP-079）
	ok := false
	if strings.TrimSpace(req.ReauthToken) != "" {
		if _, perr := h.accountRepo.PeekSecurityToken(strings.TrimSpace(req.ReauthToken), models.SecurityPurposeReauth, accountID); perr == nil {
			ok = true
		}
	}
	if !ok {
		if strings.TrimSpace(req.ReauthPassword) == "" {
			return errPrivacy("reauth_password required")
		}
		bindings, err := h.accountRepo.ListBindings(accountID)
		if err != nil {
			return err
		}
		for _, b := range bindings {
			if b.Provider == "email" && b.Verified && h.accountRepo.VerifyBindingCredential(&b, req.ReauthPassword) {
				ok = true
				break
			}
		}
	}
	if !ok {
		return errPrivacy("reauth failed")
	}

	return h.db.Transaction(func(tx *gorm.DB) error {
		ar := h.accountRepo.WithTx(tx)
		// Domain data is account-owned. Never scan it by historical device_id.
		animalRepo := h.animalRepo.WithTx(tx)
		if err := animalRepo.SoftDeleteByAccount(accountID); err != nil {
			return err
		}
		if err := deleteGeneratedDataByAccount(tx, accountID); err != nil {
			return err
		}
		if err := tx.Model(&models.Entitlement{}).Where("account_id = ?", accountID).Update("active", false).Error; err != nil {
			return err
		}

		// Only active devices still owned by this account may have credentials revoked.
		devs, err := listActiveAccountDevices(tx, accountID)
		if err != nil {
			return err
		}
		for _, d := range devs {
			if err := tx.Model(&models.DataRequest{}).Where("device_id = ? AND type = ?", d.DeviceID, "export").
				Updates(map[string]interface{}{"payload": ""}).Error; err != nil {
				return err
			}
			if err := tx.Where("device_id = ?", d.DeviceID).Delete(&models.IdempotencyRecord{}).Error; err != nil {
				return err
			}
			// revoke device
			if err := ar.RevokeDevice(accountID, d.DeviceID); err != nil {
				return err
			}
			dr := h.deviceRepo.WithTx(tx)
			if err := dr.UpdateConsent(d.DeviceID, "", "", true); err != nil {
				return err
			}
		}
		// anonymize orders (legal retain)
		anon := "anon:" + accountID[:8]
		if err := tx.Model(&models.Order{}).Where("account_id = ?", accountID).
			Updates(map[string]interface{}{"account_id": anon, "device_id": anon}).Error; err != nil {
			return err
		}
		// disable account + remove bindings
		if err := tx.Model(&models.Account{}).Where("account_id = ?", accountID).Update("status", "deleted").Error; err != nil {
			return err
		}
		if err := tx.Where("account_id = ?", accountID).Delete(&models.AccountBinding{}).Error; err != nil {
			return err
		}
		if err := tx.Where("account_id = ?", accountID).Delete(&models.DeviceAccount{}).Error; err != nil {
			return err
		}
		return nil
	})
}

type privacyError string

func (e privacyError) Error() string { return string(e) }
func errPrivacy(msg string) error    { return privacyError(msg) }

// GetDataRequest GET /privacy/requests/:id
func (h *PrivacyHandler) GetDataRequest(c *gin.Context) {
	id := c.Param("id")
	var dr models.DataRequest
	if err := h.db.Where("request_id = ? AND device_id = ?", id, middleware.GetDeviceID(c)).First(&dr).Error; err != nil {
		middleware.WriteError(c, http.StatusNotFound, "not_found", "not found", false, nil)
		return
	}
	c.JSON(http.StatusOK, dr)
}

// SecurityHandler 安全报告。
// Nonce 使用 SharedCounter.SetNX（Redis SET NX EX 或内存 TTL map），禁止无界 map。
// Fail-closed：共享存储错误时返回 503，避免跨 Pod 重放穿透；已知重放返回 409。
type SecurityHandler struct {
	db        *gorm.DB
	auditRepo *repo.AuditLogRepo
	nonces    middleware.SharedCounter
}

// NewSecurityHandler 构造；counter 为 nil 时使用进程内 MemorySharedCounter。
func NewSecurityHandler(db *gorm.DB, audit *repo.AuditLogRepo, counter middleware.SharedCounter) *SecurityHandler {
	if counter == nil {
		counter = middleware.NewMemorySharedCounter()
	}
	return &SecurityHandler{db: db, auditRepo: audit, nonces: counter}
}

type securityReportRequest struct {
	Nonce   string                 `json:"nonce" binding:"required"`
	Payload map[string]interface{} `json:"payload"`
}

// Report POST /security/report
// Nonce 策略：SET NX EX 5m；fail-closed on store error。
func (h *SecurityHandler) Report(c *gin.Context) {
	var req securityReportRequest
	if err := middleware.BindStrictJSON(c, &req); err != nil {
		middleware.WriteBindError(c, err)
		return
	}
	if len(req.Nonce) > 64 {
		middleware.WriteError(c, http.StatusBadRequest, "nonce_too_long", "nonce too long", false, nil)
		return
	}
	// 重放检查：SharedCounter.SetNX（Redis 或内存 TTL）
	nonceKey := middleware.KeyPrefixNonce + req.Nonce
	ok, err := h.nonces.SetNX(c.Request.Context(), nonceKey, 5*time.Minute)
	if err != nil {
		// fail-closed：无法确认唯一性时拒绝
		middleware.WriteError(c, http.StatusServiceUnavailable, "nonce_store_error", "nonce store unavailable", true, nil)
		return
	}
	if !ok {
		middleware.ObserveNonceReplay()
		middleware.WriteError(c, http.StatusConflict, "replay", "nonce replay", false, nil)
		return
	}

	deviceID := middleware.GetDeviceID(c)
	// 服务端重算风险：不信任客户端 score
	risk := 0
	if v, ok := req.Payload["client_skew_ms"].(float64); ok && (v > 300000 || v < -300000) {
		risk += 40
	}
	if v, ok := req.Payload["debugger"].(bool); ok && v {
		risk += 50
	}
	payload, _ := json.Marshal(req.Payload)
	if len(payload) > 16<<10 {
		middleware.WriteError(c, http.StatusBadRequest, "payload_too_large", "payload too large", false, nil)
		return
	}
	report := models.SecurityReport{
		ReportID: uuid.NewString(), DeviceID: deviceID, Nonce: req.Nonce,
		Payload: string(payload), RiskScore: risk,
	}
	if err := h.db.Create(&report).Error; err != nil {
		middleware.WriteError(c, http.StatusInternalServerError, "save_failed", "save failed", true, nil)
		return
	}
	if risk >= 40 && h.auditRepo != nil {
		_ = h.auditRepo.Create(&models.AuditLog{
			DeviceID: deviceID, Type: "security", Message: "security_report",
			Metadata: string(payload), RiskScore: risk, Status: "open",
		})
	}
	// 失败不默认为安全：返回 recalculated risk
	c.JSON(http.StatusOK, gin.H{"status": "accepted", "risk_score": risk, "safe": risk < 40})
}

// AuditHandler 管理员审计查询。
type AuditHandler struct {
	auditRepo *repo.AuditLogRepo
}

// NewAuditHandler 构造。
func NewAuditHandler(audit *repo.AuditLogRepo) *AuditHandler {
	return &AuditHandler{auditRepo: audit}
}

// List GET /admin/audit/logs
func (h *AuditHandler) List(c *gin.Context) {
	deviceID := c.Query("device_id")
	logType := c.Query("type")
	status := c.Query("status")
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	logs, total, err := h.auditRepo.Query(deviceID, logType, status, nil, nil, offset, limit)
	if err != nil {
		middleware.WriteError(c, http.StatusInternalServerError, "query_failed", "query failed", true, nil)
		return
	}
	// 管理员查询本身记审计（真实 actor，禁止写死 admin）
	actorID := "unknown"
	if a := middleware.GetAdminActor(c); a != nil && a.ActorID != "" {
		actorID = a.ActorID
	}
	_ = h.auditRepo.Create(&models.AuditLog{
		DeviceID: actorID, Type: "admin", Message: "audit_query",
		Metadata: deviceID + "|" + logType + "|sid=" + adminSessionID(c) + "|rid=" + middleware.GetRequestID(c),
		Status:   "closed",
	})
	c.JSON(http.StatusOK, gin.H{"items": logs, "total": total, "actor": actorID, "request_id": middleware.GetRequestID(c)})
}

// Ack POST /admin/audit/logs/:id/ack
func (h *AuditHandler) Ack(c *gin.Context) {
	id64, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	actorID := "unknown"
	if a := middleware.GetAdminActor(c); a != nil && a.ActorID != "" {
		actorID = a.ActorID
	}
	if err := h.auditRepo.Ack(uint(id64), actorID); err != nil {
		middleware.WriteError(c, http.StatusInternalServerError, "ack_failed", "ack failed", true, nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ack", "acked_by": actorID, "request_id": middleware.GetRequestID(c)})
}

func adminSessionID(c *gin.Context) string {
	if a := middleware.GetAdminActor(c); a != nil {
		return a.SessionID
	}
	return ""
}

// CommerceHandler 订单与权益。
//
// 安全策略（AP-012）：
//   - COMMERCE_ENABLED=false 或 production 且未开启 COMMERCE_STORE_VERIFY 时，
//     create/fulfill/admin-refund 返回 501/503，reason_code=commerce_not_ready 或 fulfill_disabled。
//   - 商品仅允许目录内 active 产品，禁止请求路径自动造品。
//   - 幂等作用域为 (device_id, idempotency_key)。
//   - 履约：短回执拒绝；非 production 仅 mock；production 需商店验签开关。
//   - 设备 JWT 不可退款；退款走管理员/平台 webhook。
type CommerceHandler struct {
	db          *gorm.DB
	production  bool
	enabled     bool
	storeVerify bool
}

// CommerceOptions 商业化运行时开关。
type CommerceOptions struct {
	Production  bool
	Enabled     bool
	StoreVerify bool
}

// NewCommerceHandler 构造（兼容旧调用：仅 db 时按开发默认开启）。
func NewCommerceHandler(db *gorm.DB) *CommerceHandler {
	return NewCommerceHandlerWithOptions(db, CommerceOptions{Enabled: true})
}

// NewCommerceHandlerWithOptions 带配置构造。
func NewCommerceHandlerWithOptions(db *gorm.DB, opts CommerceOptions) *CommerceHandler {
	return &CommerceHandler{
		db:          db,
		production:  opts.Production,
		enabled:     opts.Enabled,
		storeVerify: opts.StoreVerify,
	}
}

const minReceiptLen = 8

type createOrderRequest struct {
	ProductID      string `json:"product_id" binding:"required"`
	IdempotencyKey string `json:"idempotency_key" binding:"required"`
	Platform       string `json:"platform"`
}

// commerceGate 检查商业化是否可用。不可用时写入响应并返回 true。
// op: create|fulfill|refund
func (h *CommerceHandler) commerceGate(c *gin.Context, op string) bool {
	if !h.enabled {
		middleware.WriteError(c, http.StatusNotImplemented, "commerce_not_ready", "commerce is disabled", false, map[string]any{
			"detail": "COMMERCE_ENABLED is false; production defaults to disabled until store verification is ready",
		})
		return true
	}
	// production 在未开启真实商店验签前关闭写路径
	if h.production && !h.storeVerify {
		code := "commerce_not_ready"
		if op == "fulfill" {
			code = "fulfill_disabled"
		}
		middleware.WriteError(c, http.StatusServiceUnavailable, code, "commerce not ready in production", true, map[string]any{
			"detail": "set COMMERCE_STORE_VERIFY=true only after Apple/Google server-side receipt verification is integrated",
		})
		return true
	}
	return false
}

// CreateOrder POST /commerce/orders
func (h *CommerceHandler) CreateOrder(c *gin.Context) {
	if h.commerceGate(c, "create") {
		return
	}
	var req createOrderRequest
	if err := middleware.BindStrictJSON(c, &req); err != nil {
		middleware.WriteBindError(c, err)
		return
	}
	deviceID := middleware.GetDeviceID(c)
	accountID := middleware.GetAccountID(c)
	var existing models.Order
	if err := h.db.Where("device_id = ? AND idempotency_key = ?", deviceID, req.IdempotencyKey).First(&existing).Error; err == nil {
		c.JSON(http.StatusOK, existing)
		return
	}

	// 仅 active 目录商品；禁止请求时自动创建
	var product models.Product
	if err := h.db.Where("product_id = ? AND active = ?", req.ProductID, true).First(&product).Error; err != nil {
		middleware.WriteError(c, http.StatusNotFound, "product_not_found", "product not found", false, nil)
		return
	}

	platform := strings.TrimSpace(req.Platform)
	if platform == "" {
		platform = "mock"
	}
	if h.production && platform == "mock" {
		middleware.WriteError(c, http.StatusBadRequest, "platform_not_allowed", "mock platform not allowed in production", false, nil)
		return
	}
	if !h.production && platform != "mock" {
		// 非生产仅允许 mock，避免误连沙盒回执到开发环境以外的路径
		middleware.WriteError(c, http.StatusBadRequest, "platform_not_allowed", "only mock platform allowed outside production", false, nil)
		return
	}

	order := models.Order{
		OrderID: uuid.NewString(), DeviceID: deviceID, AccountID: accountID, ProductID: product.ProductID,
		Status: "created", Platform: platform, AmountCents: product.PriceCents,
		Currency: product.Currency, IdempotencyKey: req.IdempotencyKey,
	}
	if err := h.db.Create(&order).Error; err != nil {
		// 并发幂等：再查一次
		if isDuplicate(err) {
			if err2 := h.db.Where("device_id = ? AND idempotency_key = ?", deviceID, req.IdempotencyKey).First(&existing).Error; err2 == nil {
				c.JSON(http.StatusOK, existing)
				return
			}
		}
		middleware.WriteError(c, http.StatusInternalServerError, "create_failed", "create order failed", true, nil)
		return
	}
	c.JSON(http.StatusCreated, order)
}

type fulfillRequest struct {
	OrderID string `json:"order_id" binding:"required"`
	Receipt string `json:"receipt" binding:"required"`
}

// FulfillOrder POST /commerce/orders/fulfill — 校验回执并幂等发放权益。
func (h *CommerceHandler) FulfillOrder(c *gin.Context) {
	if h.commerceGate(c, "fulfill") {
		return
	}
	var req fulfillRequest
	if err := middleware.BindStrictJSON(c, &req); err != nil {
		middleware.WriteBindError(c, err)
		return
	}
	deviceID := middleware.GetDeviceID(c)
	accountID := middleware.GetAccountID(c)
	if len(strings.TrimSpace(req.Receipt)) < minReceiptLen {
		middleware.WriteError(c, http.StatusBadRequest, "receipt_too_short", "receipt too short", false, nil)
		return
	}
	sum := sha256.Sum256([]byte(req.Receipt))
	receiptHash := hex.EncodeToString(sum[:])

	// 伪造/重复回执：同一 receipt_hash 只能履约一次
	var byReceipt models.Order
	if err := h.db.Where("receipt_hash = ?", receiptHash).First(&byReceipt).Error; err == nil {
		if byReceipt.Status == "fulfilled" {
			// 仅本设备可见已履约；跨设备不泄露
			if byReceipt.DeviceID != deviceID {
				middleware.WriteError(c, http.StatusConflict, "receipt_replay", "receipt already used", false, nil)
				return
			}
			c.JSON(http.StatusOK, gin.H{"status": "already_fulfilled", "order_id": byReceipt.OrderID})
			return
		}
	}

	var order models.Order
	if err := h.db.Where("order_id = ? AND device_id = ?", req.OrderID, deviceID).First(&order).Error; err != nil {
		middleware.WriteError(c, http.StatusNotFound, "order_not_found", "order not found", false, nil)
		return
	}
	if order.Status == "fulfilled" {
		c.JSON(http.StatusOK, gin.H{"status": "already_fulfilled", "order_id": order.OrderID})
		return
	}
	if order.Status == "refunded" {
		middleware.WriteError(c, http.StatusConflict, "order_refunded", "order refunded", false, nil)
		return
	}

	platform := order.Platform
	if platform == "" {
		platform = "mock"
	}

	// 平台策略：非 production 仅 mock；production 禁止 mock
	if h.production {
		if platform == "mock" {
			middleware.WriteError(c, http.StatusBadRequest, "platform_not_allowed", "mock platform not allowed in production", false, nil)
			return
		}
		// 商店验签已开启但尚未对接 Apple/Google 时：结构化拒绝伪造回执，不发放权益
		if !verifyStoreReceiptStub(platform, req.Receipt, order) {
			middleware.WriteError(c, http.StatusBadRequest, "receipt_invalid", "store receipt verification failed", false, map[string]any{
				"detail": "stub verifier rejects receipts until real Apple/Google integration is complete",
			})
			return
		}
	} else if platform != "mock" {
		middleware.WriteError(c, http.StatusBadRequest, "platform_not_allowed", "only mock platform allowed outside production", false, nil)
		return
	}

	var product models.Product
	if err := h.db.Where("product_id = ? AND active = ?", order.ProductID, true).First(&product).Error; err != nil {
		middleware.WriteError(c, http.StatusNotFound, "product_not_found", "product not found", false, nil)
		return
	}

	err := h.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		if err := tx.Model(&order).Updates(map[string]interface{}{
			"status": "fulfilled", "receipt_hash": receiptHash, "fulfilled_at": now,
		}).Error; err != nil {
			return err
		}
		var ent models.Entitlement
		err := tx.Where("device_id = ? AND product_id = ?", deviceID, order.ProductID).First(&ent).Error
		var exp *time.Time
		if product.DurationDay > 0 {
			e := now.AddDate(0, 0, product.DurationDay)
			exp = &e
		}
		if err == gorm.ErrRecordNotFound {
			ent = models.Entitlement{
				DeviceID: deviceID, AccountID: accountID, ProductID: order.ProductID, OrderID: order.OrderID,
				Active: true, StartsAt: now, ExpiresAt: exp,
			}
			return tx.Create(&ent).Error
		}
		if err != nil {
			return err
		}
		// 续期：从 max(now, old exp) 延长
		start := now
		if ent.ExpiresAt != nil && ent.ExpiresAt.After(now) && exp != nil {
			e := ent.ExpiresAt.AddDate(0, 0, product.DurationDay)
			exp = &e
			start = ent.StartsAt
		}
		return tx.Model(&ent).Updates(map[string]interface{}{
			"active": true, "order_id": order.OrderID, "starts_at": start, "expires_at": exp,
		}).Error
	})
	if err != nil {
		// 唯一冲突视为幂等成功
		if isDuplicate(err) {
			c.JSON(http.StatusOK, gin.H{"status": "already_fulfilled"})
			return
		}
		slog.Error("履约失败", "err", err)
		middleware.WriteError(c, http.StatusInternalServerError, "fulfill_failed", "fulfill failed", true, nil)
		return
	}
	_ = h.db.Create(&models.AuditLog{
		DeviceID: deviceID, Type: "commerce", Message: "order_fulfilled",
		Metadata: order.OrderID, Status: "closed",
	})
	c.JSON(http.StatusOK, gin.H{"status": "fulfilled", "order_id": order.OrderID})
}

// verifyStoreReceiptStub 生产侧商店验签占位。
// 真实 Apple/Google 接入前：拒绝所有回执（防伪造履约），返回 false。
// 接入后应校验 bundle/product/transaction/amount/currency/environment。
func verifyStoreReceiptStub(platform, receipt string, order models.Order) bool {
	_ = platform
	_ = receipt
	_ = order
	return false
}

// RefundOrder POST /commerce/orders/refund — 设备 JWT 路径永久禁止。
// 客户端不得自行决定退款；请走管理员或平台 webhook。
func (h *CommerceHandler) RefundOrder(c *gin.Context) {
	middleware.WriteError(c, http.StatusForbidden, "device_refund_forbidden", "device-initiated refund is forbidden", false, map[string]any{
		"detail": "use admin POST /admin/commerce/orders/refund or platform webhook",
	})
}

// AdminRefundOrder POST /admin/commerce/orders/refund — 管理员/运维退款。
// 要求 RBAC commerce.refund；审计写入真实 actor/session/reason/request_id。
func (h *CommerceHandler) AdminRefundOrder(c *gin.Context) {
	if h.commerceGate(c, "refund") {
		return
	}
	var req struct {
		OrderID  string `json:"order_id" binding:"required"`
		DeviceID string `json:"device_id"` // 可选，用于额外校验
		Reason   string `json:"reason"`
	}
	if err := middleware.BindStrictJSON(c, &req); err != nil {
		middleware.WriteBindError(c, err)
		return
	}

	actorID := "unknown"
	sessionID := ""
	role := ""
	if a := middleware.GetAdminActor(c); a != nil {
		if a.ActorID != "" {
			actorID = a.ActorID
		}
		sessionID = a.SessionID
		role = a.Role
	}
	reason := req.Reason
	if reason == "" {
		reason = middleware.GetAdminReason(c)
	}
	if reason == "" {
		middleware.AbortBadRequest(c, "admin_reason_required", "refund reason required", nil)
		return
	}

	var order models.Order
	q := h.db.Where("order_id = ?", req.OrderID)
	if req.DeviceID != "" {
		q = q.Where("device_id = ?", req.DeviceID)
	}
	if err := q.First(&order).Error; err != nil {
		middleware.WriteError(c, http.StatusNotFound, "order_not_found", "order not found", false, nil)
		return
	}
	if order.Status == "refunded" {
		c.JSON(http.StatusOK, gin.H{"status": "already_refunded", "order_id": order.OrderID, "actor": actorID})
		return
	}

	now := time.Now().UTC()
	err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&order).Updates(map[string]interface{}{
			"status": "refunded", "refunded_at": now,
		}).Error; err != nil {
			return err
		}
		return tx.Model(&models.Entitlement{}).
			Where("device_id = ? AND product_id = ? AND order_id = ?", order.DeviceID, order.ProductID, order.OrderID).
			Updates(map[string]interface{}{"active": false, "expires_at": now}).Error
	})
	if err != nil {
		middleware.WriteError(c, http.StatusInternalServerError, "refund_failed", "refund failed", true, nil)
		return
	}
	meta, _ := json.Marshal(map[string]any{
		"order_id":   order.OrderID,
		"reason":     reason,
		"actor":      actorID,
		"session_id": sessionID,
		"role":       role,
		"request_id": middleware.GetRequestID(c),
	})
	_ = h.db.Create(&models.AuditLog{
		DeviceID: order.DeviceID, Type: "commerce", Message: "order_refunded_admin",
		Metadata: string(meta), Status: "closed",
	})
	c.JSON(http.StatusOK, gin.H{
		"status": "refunded", "order_id": order.OrderID,
		"actor": actorID, "request_id": middleware.GetRequestID(c),
	})
}

// WebhookRefundOrder POST /admin/commerce/webhooks/refund — 平台 webhook 占位。
// 与 AdminAuth 相同密钥校验；真实接入后应校验平台签名。
func (h *CommerceHandler) WebhookRefundOrder(c *gin.Context) {
	h.AdminRefundOrder(c)
}

// GetOrder GET /commerce/orders/:id — 始终设备作用域。
func (h *CommerceHandler) GetOrder(c *gin.Context) {
	var order models.Order
	if err := h.db.Where("order_id = ? AND device_id = ?", c.Param("id"), middleware.GetDeviceID(c)).First(&order).Error; err != nil {
		middleware.WriteError(c, http.StatusNotFound, "order_not_found", "not found", false, nil)
		return
	}
	c.JSON(http.StatusOK, order)
}

// ListEntitlements GET /commerce/entitlements
func (h *CommerceHandler) ListEntitlements(c *gin.Context) {
	deviceID := middleware.GetDeviceID(c)
	accountID := middleware.GetAccountID(c)
	var ents []models.Entitlement
	q := h.db.Where("device_id = ?", deviceID)
	if accountID != "" {
		q = h.db.Where("device_id = ? OR (account_id = ? AND active = ?)", deviceID, accountID, true)
	}
	_ = q.Find(&ents).Error
	c.JSON(http.StatusOK, gin.H{"items": ents})
}
