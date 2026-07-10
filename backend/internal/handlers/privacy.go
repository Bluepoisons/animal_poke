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
	// 同步导出（小规模）
	dev, _ := h.deviceRepo.Find(deviceID)
	animals, _ := h.animalRepo.ListByDevice(deviceID, 0, 200)
	// 脱敏精确坐标
	for i := range animals {
		animals[i].PreciseLat = nil
		animals[i].PreciseLng = nil
	}
	payload, _ := json.Marshal(gin.H{"device": dev, "animals": animals})
	now := time.Now().UTC()
	_ = h.db.Model(&dr).Updates(map[string]interface{}{
		"status": "completed", "payload": string(payload), "completed_at": now,
	})
	c.JSON(http.StatusOK, gin.H{"request_id": reqID, "status": "completed", "data": json.RawMessage(payload)})
}

// DeleteData POST /privacy/delete
func (h *PrivacyHandler) DeleteData(c *gin.Context) {
	deviceID := middleware.GetDeviceID(c)
	reqID := uuid.NewString()
	dr := models.DataRequest{
		RequestID: reqID, DeviceID: deviceID, Type: "delete", Status: "processing",
		RequestedAt: time.Now().UTC(),
	}
	_ = h.db.Create(&dr).Error
	err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := h.animalRepo.WithTx(tx).SoftDeleteByDevice(deviceID); err != nil {
			return err
		}
		if h.inferenceRepo != nil {
			if err := h.inferenceRepo.WithTx(tx).SoftDeleteByDevice(deviceID); err != nil {
				return err
			}
		}
		// 撤销授权
		return h.deviceRepo.UpdateConsent(deviceID, "", "", true)
	})
	now := time.Now().UTC()
	status := "completed"
	if err != nil {
		status = "failed"
		slog.Error("删除失败", "err", err)
	}
	_ = h.db.Model(&dr).Updates(map[string]interface{}{"status": status, "completed_at": now})
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
	accountID := middleware.GetAccountID(c)
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
		OrderID: uuid.NewString(), DeviceID: deviceID, AccountID: accountID, ProductID: product.ProductID,
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
	accountID := middleware.GetAccountID(c)
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
