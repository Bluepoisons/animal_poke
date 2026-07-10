package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupCommerce(t *testing.T, opts CommerceOptions) (*gin.Engine, *gorm.DB, *CommerceHandler) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open("file:commerce_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Product{}, &models.Order{}, &models.Entitlement{}, &models.AuditLog{}))

	// 已知目录商品
	require.NoError(t, db.Create(&models.Product{
		ProductID: "month_card", Name: "月卡", Type: "subscription",
		PriceCents: 1800, Currency: "CNY", DurationDay: 30, Active: true,
	}).Error)

	h := NewCommerceHandlerWithOptions(db, opts)
	r := gin.New()

	// 设备作用域路由：测试用 header X-Test-Device 注入 device_id
	deviceAuth := func(c *gin.Context) {
		dev := c.GetHeader("X-Test-Device")
		if dev == "" {
			dev = "dev-1"
		}
		c.Set(middleware.ContextKeyDeviceID, dev)
		c.Next()
	}
	auth := r.Group("/api/v1")
	auth.Use(deviceAuth)
	{
		auth.POST("/commerce/orders", h.CreateOrder)
		auth.POST("/commerce/orders/fulfill", h.FulfillOrder)
		auth.POST("/commerce/orders/refund", h.RefundOrder)
		auth.GET("/commerce/orders/:id", h.GetOrder)
		auth.GET("/commerce/entitlements", h.ListEntitlements)
	}
	// 管理员退款（测试直接挂路由，不走 AdminAuth）
	r.POST("/api/v1/admin/commerce/orders/refund", h.AdminRefundOrder)
	return r, db, h
}

func postJSON(r *gin.Engine, path string, body map[string]interface{}, device string) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if device != "" {
		req.Header.Set("X-Test-Device", device)
	}
	r.ServeHTTP(w, req)
	return w
}

func TestCommerce_DisabledReturns501(t *testing.T) {
	r, _, _ := setupCommerce(t, CommerceOptions{Enabled: false})

	w := postJSON(r, "/api/v1/commerce/orders", map[string]interface{}{
		"product_id": "month_card", "idempotency_key": "k1", "platform": "mock",
	}, "dev-1")
	assert.Equal(t, http.StatusNotImplemented, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "commerce_not_ready", resp["reason_code"])
}

func TestCommerce_UnknownProduct404(t *testing.T) {
	r, _, _ := setupCommerce(t, CommerceOptions{Enabled: true})

	w := postJSON(r, "/api/v1/commerce/orders", map[string]interface{}{
		"product_id": "unknown_sku", "idempotency_key": "k-unknown", "platform": "mock",
	}, "dev-1")
	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "product_not_found", resp["reason_code"])
}

func TestCommerce_CrossDeviceSameIdempotencyKey(t *testing.T) {
	r, _, _ := setupCommerce(t, CommerceOptions{Enabled: true})

	w1 := postJSON(r, "/api/v1/commerce/orders", map[string]interface{}{
		"product_id": "month_card", "idempotency_key": "shared-key", "platform": "mock",
	}, "device-a")
	assert.Equal(t, http.StatusCreated, w1.Code)
	var o1 models.Order
	require.NoError(t, json.Unmarshal(w1.Body.Bytes(), &o1))
	require.NotEmpty(t, o1.OrderID)

	// 同 key 不同设备 → 独立订单，不返回 device-a 的订单
	w2 := postJSON(r, "/api/v1/commerce/orders", map[string]interface{}{
		"product_id": "month_card", "idempotency_key": "shared-key", "platform": "mock",
	}, "device-b")
	assert.Equal(t, http.StatusCreated, w2.Code)
	var o2 models.Order
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &o2))
	assert.NotEqual(t, o1.OrderID, o2.OrderID)
	assert.Equal(t, "device-b", o2.DeviceID)

	// 同设备同 key → 幂等返回原订单
	w3 := postJSON(r, "/api/v1/commerce/orders", map[string]interface{}{
		"product_id": "month_card", "idempotency_key": "shared-key", "platform": "mock",
	}, "device-a")
	assert.Equal(t, http.StatusOK, w3.Code)
	var o3 models.Order
	require.NoError(t, json.Unmarshal(w3.Body.Bytes(), &o3))
	assert.Equal(t, o1.OrderID, o3.OrderID)
}

func TestCommerce_DeviceRefundForbidden(t *testing.T) {
	r, _, _ := setupCommerce(t, CommerceOptions{Enabled: true})

	// 先建单
	w := postJSON(r, "/api/v1/commerce/orders", map[string]interface{}{
		"product_id": "month_card", "idempotency_key": "k-refund", "platform": "mock",
	}, "dev-1")
	require.Equal(t, http.StatusCreated, w.Code)
	var order models.Order
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &order))

	wr := postJSON(r, "/api/v1/commerce/orders/refund", map[string]interface{}{
		"order_id": order.OrderID,
	}, "dev-1")
	assert.Equal(t, http.StatusForbidden, wr.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(wr.Body.Bytes(), &resp))
	assert.Equal(t, "device_refund_forbidden", resp["reason_code"])
}

func TestCommerce_AdminRefundWorks(t *testing.T) {
	r, db, _ := setupCommerce(t, CommerceOptions{Enabled: true})

	w := postJSON(r, "/api/v1/commerce/orders", map[string]interface{}{
		"product_id": "month_card", "idempotency_key": "k-admin-refund", "platform": "mock",
	}, "dev-1")
	require.Equal(t, http.StatusCreated, w.Code)
	var order models.Order
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &order))

	// 模拟已履约权益
	require.NoError(t, db.Model(&order).Update("status", "fulfilled").Error)
	require.NoError(t, db.Create(&models.Entitlement{
		DeviceID: order.DeviceID, ProductID: order.ProductID, OrderID: order.OrderID, Active: true,
	}).Error)

	wr := postJSON(r, "/api/v1/admin/commerce/orders/refund", map[string]interface{}{
		"order_id": order.OrderID, "reason": "test",
	}, "")
	assert.Equal(t, http.StatusOK, wr.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(wr.Body.Bytes(), &resp))
	assert.Equal(t, "refunded", resp["status"])

	var updated models.Order
	require.NoError(t, db.Where("order_id = ?", order.OrderID).First(&updated).Error)
	assert.Equal(t, "refunded", updated.Status)

	var ent models.Entitlement
	require.NoError(t, db.Where("order_id = ?", order.OrderID).First(&ent).Error)
	assert.False(t, ent.Active)
}

func TestCommerce_ShortReceiptRejected(t *testing.T) {
	r, _, _ := setupCommerce(t, CommerceOptions{Enabled: true})

	w := postJSON(r, "/api/v1/commerce/orders", map[string]interface{}{
		"product_id": "month_card", "idempotency_key": "k-short", "platform": "mock",
	}, "dev-1")
	require.Equal(t, http.StatusCreated, w.Code)
	var order models.Order
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &order))

	wf := postJSON(r, "/api/v1/commerce/orders/fulfill", map[string]interface{}{
		"order_id": order.OrderID, "receipt": "short",
	}, "dev-1")
	assert.Equal(t, http.StatusBadRequest, wf.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(wf.Body.Bytes(), &resp))
	assert.Equal(t, "receipt_too_short", resp["reason_code"])
}

func TestCommerce_ProductionWithoutStoreVerifyDisabled(t *testing.T) {
	r, _, _ := setupCommerce(t, CommerceOptions{Production: true, Enabled: true, StoreVerify: false})

	w := postJSON(r, "/api/v1/commerce/orders", map[string]interface{}{
		"product_id": "month_card", "idempotency_key": "k-prod", "platform": "apple",
	}, "dev-1")
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "commerce_not_ready", resp["reason_code"])
}

func TestCommerce_FulfillMockHappyPath(t *testing.T) {
	r, db, _ := setupCommerce(t, CommerceOptions{Enabled: true})

	w := postJSON(r, "/api/v1/commerce/orders", map[string]interface{}{
		"product_id": "month_card", "idempotency_key": "k-fulfill", "platform": "mock",
	}, "dev-1")
	require.Equal(t, http.StatusCreated, w.Code)
	var order models.Order
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &order))

	wf := postJSON(r, "/api/v1/commerce/orders/fulfill", map[string]interface{}{
		"order_id": order.OrderID, "receipt": "mock-receipt-ok-12345",
	}, "dev-1")
	assert.Equal(t, http.StatusOK, wf.Code)

	var ent models.Entitlement
	require.NoError(t, db.Where("device_id = ? AND product_id = ?", "dev-1", "month_card").First(&ent).Error)
	assert.True(t, ent.Active)
	assert.Equal(t, order.OrderID, ent.OrderID)
}

func TestCommerce_GetOrderDeviceScoped(t *testing.T) {
	r, _, _ := setupCommerce(t, CommerceOptions{Enabled: true})

	w := postJSON(r, "/api/v1/commerce/orders", map[string]interface{}{
		"product_id": "month_card", "idempotency_key": "k-get", "platform": "mock",
	}, "dev-owner")
	require.Equal(t, http.StatusCreated, w.Code)
	var order models.Order
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &order))

	// 他人设备不可见
	wOther := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/commerce/orders/"+order.OrderID, nil)
	req.Header.Set("X-Test-Device", "dev-other")
	r.ServeHTTP(wOther, req)
	assert.Equal(t, http.StatusNotFound, wOther.Code)

	// 本设备可见
	wOwn := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/api/v1/commerce/orders/"+order.OrderID, nil)
	req2.Header.Set("X-Test-Device", "dev-owner")
	r.ServeHTTP(wOwn, req2)
	assert.Equal(t, http.StatusOK, wOwn.Code)
}
