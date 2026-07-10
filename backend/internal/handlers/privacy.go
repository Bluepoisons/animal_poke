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
	db             *gorm.DB
	consentVersion string
}

// NewPrivacyHandler 构造。
func NewPrivacyHandler(db *gorm.DB, device *repo.DeviceRepo, animal *repo.AnimalRepo, inf *repo.InferenceRepo, audit *repo.AuditLogRepo) *PrivacyHandler {
	return &PrivacyHandler{
		deviceRepo: device, animalRepo: animal, inferenceRepo: inf, auditRepo: audit, db: db,
		consentVersion: "v1",
	}
}

type consentRequest struct {
	Version string `json:"version" binding:"required"`
	Scope   string `json:"scope"`
	Revoke  bool   `json:"revoke"`
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
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "version required"})
		return
	}
	if h.consentVersion != "" && req.Version != h.consentVersion {
		// 仅接受当前服务端版本；升级时客户端需重弹
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported consent version", "required_version": h.consentVersion})
		return
	}
	deviceID := middleware.GetDeviceID(c)
	scope, err := normalizeConsentScope(req.Scope)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid scope", "allowed": []string{"photo", "location", "precise_location"}})
		return
	}
	if err := h.deviceRepo.UpdateConsent(deviceID, req.Version, scope, req.Revoke); err != nil {
		// DB 不可用 fail-closed
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "update failed", "reason_code": "db_unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "version": req.Version, "scope": scope, "revoked": req.Revoke})
}

// ExportData POST /privacy/export
// 默认完整导出（分页循环 ListByDevice 直至空）；可选 ?cursor=<after_id> 仅返回一页便于大包分片。
// 导出始终脱敏精确坐标；安全报告仅元数据（不含 payload 密钥/原文）。
func (h *PrivacyHandler) ExportData(c *gin.Context) {
	deviceID := middleware.GetDeviceID(c)
	reqID := uuid.NewString()
	dr := models.DataRequest{
		RequestID: reqID, DeviceID: deviceID, Type: "export", Status: "processing",
		RequestedAt: time.Now().UTC(),
	}
	if err := h.db.Create(&dr).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create request failed"})
		return
	}

	pageOnly := false
	var afterID uint
	if v := strings.TrimSpace(c.Query("cursor")); v != "" {
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cursor"})
			return
		}
		afterID = uint(n)
		pageOnly = true
	}

	animals, nextCursor, err := h.collectExportAnimals(deviceID, afterID, pageOnly)
	if err != nil {
		now := time.Now().UTC()
		_ = h.db.Model(&dr).Updates(map[string]interface{}{
			"status": "failed", "error_msg": err.Error(), "completed_at": now,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "export animals failed"})
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
	var secCount int64
	_ = h.db.Model(&models.SecurityReport{}).Where("device_id = ?", deviceID).Count(&secCount).Error
	var secRows []models.SecurityReport
	_ = h.db.Select("report_id", "risk_score", "created_at").
		Where("device_id = ?", deviceID).
		Order("id asc").Limit(500).
		Find(&secRows).Error
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
	_ = h.db.Select("request_id", "type", "status", "requested_at", "completed_at", "created_at").
		Where("device_id = ?", deviceID).
		Order("id asc").Limit(200).
		Find(&reqRows).Error
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

	var orders []models.Order
	_ = h.db.Where("device_id = ?", deviceID).Order("id asc").Limit(500).Find(&orders).Error
	var entitlements []models.Entitlement
	_ = h.db.Where("device_id = ?", deviceID).Order("id asc").Limit(200).Find(&entitlements).Error

	tokenVersion := 0
	disabled := false
	var createdAt interface{}
	if dev != nil {
		tokenVersion = dev.TokenVersion
		disabled = dev.Disabled
		createdAt = dev.CreatedAt
	}

	payloadObj := gin.H{
		"device": gin.H{
			"device_id":     deviceID,
			"token_version": tokenVersion,
			"disabled":      disabled,
			"created_at":    createdAt,
		},
		"consent": consent,
		"animals": animals,
		"security_reports": gin.H{
			"count": secCount,
			"items": secMeta,
		},
		"data_requests": reqHist,
		"orders":        orders,
		"entitlements":  entitlements,
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
func (h *PrivacyHandler) collectExportAnimals(deviceID string, afterID uint, pageOnly bool) ([]models.Animal, uint, error) {
	const pageSize = 200
	var all []models.Animal
	cur := afterID
	for {
		batch, err := h.animalRepo.ListByDevice(deviceID, cur, pageSize)
		if err != nil {
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

// DeleteData POST /privacy/delete
// 事务内：软删动物（tombstone 版本提升）、删推理与安全报告、清空历史导出 payload、
// 权益失效、撤销授权、吊销 Token。订单依法保留不硬删。
func (h *PrivacyHandler) DeleteData(c *gin.Context) {
	deviceID := middleware.GetDeviceID(c)
	reqID := uuid.NewString()
	dr := models.DataRequest{
		RequestID: reqID, DeviceID: deviceID, Type: "delete", Status: "processing",
		RequestedAt: time.Now().UTC(),
	}
	if err := h.db.Create(&dr).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create request failed"})
		return
	}

	err := h.db.Transaction(func(tx *gorm.DB) error {
		ar := h.animalRepo.WithTx(tx)
		if err := ar.SoftDeleteByDevice(deviceID); err != nil {
			return err
		}
		// 清理已过期精确坐标（维护钩子；软删时亦已清空该设备精确字段）
		if err := ar.ClearExpiredPreciseLocation(time.Now().UTC()); err != nil {
			return err
		}
		if h.inferenceRepo != nil {
			if err := h.inferenceRepo.WithTx(tx).SoftDeleteByDevice(deviceID); err != nil {
				return err
			}
		}
		// 安全报告：删除设备侧诊断数据
		if err := tx.Where("device_id = ?", deviceID).Delete(&models.SecurityReport{}).Error; err != nil {
			return err
		}
		// 清空历史导出 payload（避免删除后仍可从旧 data_requests 恢复内容）
		if err := tx.Model(&models.DataRequest{}).
			Where("device_id = ? AND type = ?", deviceID, "export").
			Updates(map[string]interface{}{"payload": ""}).Error; err != nil {
			return err
		}
		// 权益标记失效
		if err := tx.Model(&models.Entitlement{}).
			Where("device_id = ?", deviceID).
			Update("active", false).Error; err != nil {
			return err
		}
		// 订单依法保留：不硬删。device_id 保留用于财务/审计对账；用户权益已失效。
		// 若监管要求匿名化链路，可后续将 device_id 替换为 hash 标记并保留 order_id 维度。

		drRepo := h.deviceRepo.WithTx(tx)
		// 撤销授权
		if err := drRepo.UpdateConsent(deviceID, "", "", true); err != nil {
			return err
		}
		// 吊销已有 Token（使旧 JWT 失效）；不 Disable 设备以便用户可重新授权注册。
		if err := drRepo.BumpTokenVersion(deviceID); err != nil {
			return err
		}
		return nil
	})

	now := time.Now().UTC()
	status := "completed"
	errMsg := ""
	if err != nil {
		status = "failed"
		errMsg = err.Error()
		slog.Error("删除失败", "err", err)
	}
	updates := map[string]interface{}{"status": status, "completed_at": now}
	if errMsg != "" {
		updates["error_msg"] = errMsg
	}
	_ = h.db.Model(&dr).Updates(updates)
	c.JSON(http.StatusOK, gin.H{"request_id": reqID, "status": status})
}

// GetDataRequest GET /privacy/requests/:id
func (h *PrivacyHandler) GetDataRequest(c *gin.Context) {
	id := c.Param("id")
	var dr models.DataRequest
	if err := h.db.Where("request_id = ? AND device_id = ?", id, middleware.GetDeviceID(c)).First(&dr).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, dr)
}

// SecurityHandler 安全报告。
type SecurityHandler struct {
	db        *gorm.DB
	auditRepo *repo.AuditLogRepo
	nonces    map[string]time.Time // 简单重放防护；生产用 Redis
}

// NewSecurityHandler 构造。
func NewSecurityHandler(db *gorm.DB, audit *repo.AuditLogRepo) *SecurityHandler {
	return &SecurityHandler{db: db, auditRepo: audit, nonces: map[string]time.Time{}}
}

type securityReportRequest struct {
	Nonce   string                 `json:"nonce" binding:"required"`
	Payload map[string]interface{} `json:"payload"`
}

// Report POST /security/report
func (h *SecurityHandler) Report(c *gin.Context) {
	var req securityReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nonce required"})
		return
	}
	if len(req.Nonce) > 64 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nonce too long"})
		return
	}
	// 重放检查
	if exp, ok := h.nonces[req.Nonce]; ok && time.Now().Before(exp) {
		c.JSON(http.StatusConflict, gin.H{"error": "nonce replay", "reason_code": "replay"})
		return
	}
	h.nonces[req.Nonce] = time.Now().Add(5 * time.Minute)

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
		c.JSON(http.StatusBadRequest, gin.H{"error": "payload too large"})
		return
	}
	report := models.SecurityReport{
		ReportID: uuid.NewString(), DeviceID: deviceID, Nonce: req.Nonce,
		Payload: string(payload), RiskScore: risk,
	}
	if err := h.db.Create(&report).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "save failed"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	// 管理员查询本身记审计
	_ = h.auditRepo.Create(&models.AuditLog{
		DeviceID: "admin", Type: "admin", Message: "audit_query",
		Metadata: deviceID + "|" + logType, Status: "closed",
	})
	c.JSON(http.StatusOK, gin.H{"items": logs, "total": total})
}

// Ack POST /admin/audit/logs/:id/ack
func (h *AuditHandler) Ack(c *gin.Context) {
	id64, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.auditRepo.Ack(uint(id64), "admin"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ack failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ack"})
}

// CommerceHandler 订单与权益。
type CommerceHandler struct {
	db *gorm.DB
}

// NewCommerceHandler 构造。
func NewCommerceHandler(db *gorm.DB) *CommerceHandler {
	return &CommerceHandler{db: db}
}

type createOrderRequest struct {
	ProductID      string `json:"product_id" binding:"required"`
	IdempotencyKey string `json:"idempotency_key" binding:"required"`
	Platform       string `json:"platform"`
}

// CreateOrder POST /commerce/orders
func (h *CommerceHandler) CreateOrder(c *gin.Context) {
	var req createOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "product_id and idempotency_key required"})
		return
	}
	deviceID := middleware.GetDeviceID(c)
	var existing models.Order
	if err := h.db.Where("idempotency_key = ?", req.IdempotencyKey).First(&existing).Error; err == nil {
		c.JSON(http.StatusOK, existing)
		return
	}
	var product models.Product
	if err := h.db.Where("product_id = ? AND active = ?", req.ProductID, true).First(&product).Error; err != nil {
		// 开发期自动种子
		product = models.Product{
			ProductID: req.ProductID, Name: req.ProductID, Type: "subscription",
			PriceCents: 1800, Currency: "CNY", DurationDay: 30, Active: true,
		}
		_ = h.db.Where("product_id = ?", req.ProductID).FirstOrCreate(&product, product)
	}
	order := models.Order{
		OrderID: uuid.NewString(), DeviceID: deviceID, ProductID: product.ProductID,
		Status: "created", Platform: req.Platform, AmountCents: product.PriceCents,
		Currency: product.Currency, IdempotencyKey: req.IdempotencyKey,
	}
	if err := h.db.Create(&order).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create order failed"})
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
	var req fulfillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "order_id and receipt required"})
		return
	}
	deviceID := middleware.GetDeviceID(c)
	sum := sha256.Sum256([]byte(req.Receipt))
	receiptHash := hex.EncodeToString(sum[:])

	// 伪造/重复回执：同一 receipt_hash 只能履约一次
	var byReceipt models.Order
	if err := h.db.Where("receipt_hash = ?", receiptHash).First(&byReceipt).Error; err == nil {
		if byReceipt.Status == "fulfilled" {
			c.JSON(http.StatusOK, gin.H{"status": "already_fulfilled", "order_id": byReceipt.OrderID})
			return
		}
	}

	var order models.Order
	if err := h.db.Where("order_id = ? AND device_id = ?", req.OrderID, deviceID).First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}
	if order.Status == "fulfilled" {
		c.JSON(http.StatusOK, gin.H{"status": "already_fulfilled", "order_id": order.OrderID})
		return
	}
	if order.Status == "refunded" {
		c.JSON(http.StatusConflict, gin.H{"error": "order refunded"})
		return
	}

	// mock 回执校验：非空即可；生产对接苹果/谷歌
	if len(req.Receipt) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid receipt"})
		return
	}

	var product models.Product
	_ = h.db.Where("product_id = ?", order.ProductID).First(&product).Error

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
				DeviceID: deviceID, ProductID: order.ProductID, OrderID: order.OrderID,
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "fulfill failed"})
		return
	}
	_ = h.db.Create(&models.AuditLog{
		DeviceID: deviceID, Type: "commerce", Message: "order_fulfilled",
		Metadata: order.OrderID, Status: "closed",
	})
	c.JSON(http.StatusOK, gin.H{"status": "fulfilled", "order_id": order.OrderID})
}

// RefundOrder POST /commerce/orders/refund
func (h *CommerceHandler) RefundOrder(c *gin.Context) {
	var req struct {
		OrderID string `json:"order_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "order_id required"})
		return
	}
	deviceID := middleware.GetDeviceID(c)
	var order models.Order
	if err := h.db.Where("order_id = ? AND device_id = ?", req.OrderID, deviceID).First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
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
			Where("device_id = ? AND product_id = ? AND order_id = ?", deviceID, order.ProductID, order.OrderID).
			Updates(map[string]interface{}{"active": false, "expires_at": now}).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "refund failed"})
		return
	}
	_ = h.db.Create(&models.AuditLog{
		DeviceID: deviceID, Type: "commerce", Message: "order_refunded",
		Metadata: order.OrderID, Status: "closed",
	})
	c.JSON(http.StatusOK, gin.H{"status": "refunded", "order_id": order.OrderID})
}

// GetOrder GET /commerce/orders/:id
func (h *CommerceHandler) GetOrder(c *gin.Context) {
	var order models.Order
	if err := h.db.Where("order_id = ? AND device_id = ?", c.Param("id"), middleware.GetDeviceID(c)).First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, order)
}

// ListEntitlements GET /commerce/entitlements
func (h *CommerceHandler) ListEntitlements(c *gin.Context) {
	var ents []models.Entitlement
	_ = h.db.Where("device_id = ?", middleware.GetDeviceID(c)).Find(&ents).Error
	c.JSON(http.StatusOK, gin.H{"items": ents})
}
