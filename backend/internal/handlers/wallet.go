// AP-082 钱包 / 库存 HTTP 处理器。
package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/models"
	"animalpoke/backend/internal/repo"

	"github.com/gin-gonic/gin"
)

// WalletHandler 服务端权威钱包与道具。
type WalletHandler struct {
	wallet *repo.WalletRepo
}

// NewWalletHandler 构造。
func NewWalletHandler(wallet *repo.WalletRepo) *WalletHandler {
	return &WalletHandler{wallet: wallet}
}

type walletMutateRequest struct {
	Currency    string `json:"currency" binding:"required"`
	Amount      int64  `json:"amount" binding:"required"`
	OperationID string `json:"operation_id" binding:"required"`
	SourceType  string `json:"source_type"`
	SourceID    string `json:"source_id"`
	Metadata    string `json:"metadata"`
}

type inventoryMutateRequest struct {
	ItemID      string `json:"item_id" binding:"required"`
	Quantity    int64  `json:"quantity" binding:"required"`
	OperationID string `json:"operation_id" binding:"required"`
	SourceType  string `json:"source_type"`
	SourceID    string `json:"source_id"`
	Metadata    string `json:"metadata"`
}

// GetWallet GET /api/v1/wallet — 余额快照。
func (h *WalletHandler) GetWallet(c *gin.Context) {
	if h == nil || h.wallet == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "wallet unavailable", "reason_code": "db_unavailable"})
		return
	}
	deviceID := middleware.GetDeviceID(c)
	accountID := middleware.GetAccountID(c)
	rows, err := h.wallet.GetBalances(accountID, deviceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list balances failed", "reason_code": "wallet_error"})
		return
	}
	// 保证 gold/stamina 至少出现（缺省 0）
	byCur := map[string]int64{models.CurrencyGold: 0, models.CurrencyStamina: 0}
	for _, r := range rows {
		byCur[r.Currency] = r.Balance
	}
	balances := []gin.H{
		{"currency": models.CurrencyGold, "balance": byCur[models.CurrencyGold]},
		{"currency": models.CurrencyStamina, "balance": byCur[models.CurrencyStamina]},
	}
	c.JSON(http.StatusOK, gin.H{
		"owner_key":  repo.OwnerKey(accountID, deviceID),
		"balances":   balances,
		"request_id": middleware.GetRequestID(c),
	})
}

// Credit POST /api/v1/wallet/credit — 入账（奖励/补偿）。
func (h *WalletHandler) Credit(c *gin.Context) {
	h.mutateCurrency(c, +1)
}

// Debit POST /api/v1/wallet/debit — 出账（消费）。
func (h *WalletHandler) Debit(c *gin.Context) {
	h.mutateCurrency(c, -1)
}

func (h *WalletHandler) mutateCurrency(c *gin.Context, sign int64) {
	if h == nil || h.wallet == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "wallet unavailable", "reason_code": "db_unavailable"})
		return
	}
	var req walletMutateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "currency, amount, operation_id required", "reason_code": "bad_request"})
		return
	}
	if req.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount must be positive", "reason_code": "invalid_amount"})
		return
	}
	deviceID := middleware.GetDeviceID(c)
	accountID := middleware.GetAccountID(c)
	delta := req.Amount * sign
	res, err := h.wallet.Apply(repo.ApplyRequest{
		DeviceID:    deviceID,
		AccountID:   accountID,
		Kind:        models.LedgerKindCurrency,
		Currency:    strings.TrimSpace(req.Currency),
		Delta:       delta,
		OperationID: strings.TrimSpace(req.OperationID),
		SourceType:  req.SourceType,
		SourceID:    req.SourceID,
		Metadata:    req.Metadata,
	})
	if err != nil {
		writeWalletError(c, err)
		return
	}
	status := http.StatusOK
	if !res.Idempotent && sign > 0 {
		status = http.StatusCreated
	}
	if !res.Idempotent && sign < 0 {
		status = http.StatusOK
	}
	c.JSON(status, gin.H{
		"entry":      res.Entry,
		"balance":    res.Balance,
		"idempotent": res.Idempotent,
		"request_id": middleware.GetRequestID(c),
	})
}

// ListLedger GET /api/v1/wallet/ledger — 分页流水。
func (h *WalletHandler) ListLedger(c *gin.Context) {
	if h == nil || h.wallet == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "wallet unavailable", "reason_code": "db_unavailable"})
		return
	}
	deviceID := middleware.GetDeviceID(c)
	accountID := middleware.GetAccountID(c)
	currency := strings.TrimSpace(c.Query("currency"))
	var afterID uint
	if s := c.Query("after_id"); s != "" {
		if n, err := strconv.ParseUint(s, 10, 64); err == nil {
			afterID = uint(n)
		}
	}
	limit := 50
	if s := c.Query("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			limit = n
		}
	}
	rows, err := h.wallet.ListLedger(accountID, deviceID, currency, afterID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list ledger failed", "reason_code": "wallet_error"})
		return
	}
	var nextAfter uint
	if len(rows) > 0 {
		nextAfter = rows[len(rows)-1].ID
	}
	c.JSON(http.StatusOK, gin.H{
		"entries":    rows,
		"next_after": nextAfter,
		"request_id": middleware.GetRequestID(c),
	})
}

// Reconcile POST /api/v1/wallet/reconcile — 从流水重建余额。
func (h *WalletHandler) Reconcile(c *gin.Context) {
	if h == nil || h.wallet == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "wallet unavailable", "reason_code": "db_unavailable"})
		return
	}
	var req struct {
		Currency string `json:"currency" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "currency required", "reason_code": "bad_request"})
		return
	}
	deviceID := middleware.GetDeviceID(c)
	accountID := middleware.GetAccountID(c)
	sum, err := h.wallet.RebuildBalance(accountID, deviceID, strings.TrimSpace(req.Currency))
	if err != nil {
		writeWalletError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"currency":   strings.TrimSpace(req.Currency),
		"balance":    sum,
		"request_id": middleware.GetRequestID(c),
	})
}

// GetInventory GET /api/v1/inventory — 道具列表。
func (h *WalletHandler) GetInventory(c *gin.Context) {
	if h == nil || h.wallet == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "wallet unavailable", "reason_code": "db_unavailable"})
		return
	}
	deviceID := middleware.GetDeviceID(c)
	accountID := middleware.GetAccountID(c)
	rows, err := h.wallet.GetInventory(accountID, deviceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list inventory failed", "reason_code": "wallet_error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items":      rows,
		"request_id": middleware.GetRequestID(c),
	})
}

// GrantInventory POST /api/v1/inventory/grant — 发放道具。
func (h *WalletHandler) GrantInventory(c *gin.Context) {
	h.mutateInventory(c, +1)
}

// ConsumeInventory POST /api/v1/inventory/consume — 消耗道具。
func (h *WalletHandler) ConsumeInventory(c *gin.Context) {
	h.mutateInventory(c, -1)
}

func (h *WalletHandler) mutateInventory(c *gin.Context, sign int64) {
	if h == nil || h.wallet == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "wallet unavailable", "reason_code": "db_unavailable"})
		return
	}
	var req inventoryMutateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "item_id, quantity, operation_id required", "reason_code": "bad_request"})
		return
	}
	if req.Quantity <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "quantity must be positive", "reason_code": "invalid_amount"})
		return
	}
	deviceID := middleware.GetDeviceID(c)
	accountID := middleware.GetAccountID(c)
	delta := req.Quantity * sign
	res, err := h.wallet.Apply(repo.ApplyRequest{
		DeviceID:    deviceID,
		AccountID:   accountID,
		Kind:        models.LedgerKindItem,
		Currency:    strings.TrimSpace(req.ItemID),
		Delta:       delta,
		OperationID: strings.TrimSpace(req.OperationID),
		SourceType:  req.SourceType,
		SourceID:    req.SourceID,
		Metadata:    req.Metadata,
	})
	if err != nil {
		writeWalletError(c, err)
		return
	}
	status := http.StatusOK
	if !res.Idempotent && sign > 0 {
		status = http.StatusCreated
	}
	c.JSON(status, gin.H{
		"entry":      res.Entry,
		"quantity":   res.Balance,
		"idempotent": res.Idempotent,
		"request_id": middleware.GetRequestID(c),
	})
}

func writeWalletError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repo.ErrInsufficientBalance):
		c.JSON(http.StatusConflict, gin.H{"error": "insufficient balance", "reason_code": "insufficient_balance"})
	case errors.Is(err, repo.ErrInsufficientItem):
		c.JSON(http.StatusConflict, gin.H{"error": "insufficient item quantity", "reason_code": "insufficient_item"})
	case errors.Is(err, repo.ErrInvalidAmount):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid amount", "reason_code": "invalid_amount"})
	case errors.Is(err, repo.ErrInvalidCurrency):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid currency", "reason_code": "invalid_currency"})
	case errors.Is(err, repo.ErrInvalidItemID):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item_id", "reason_code": "invalid_item_id"})
	case errors.Is(err, repo.ErrInvalidOperationID):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid operation_id", "reason_code": "invalid_operation_id"})
	case errors.Is(err, repo.ErrInvalidOwner):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "device required", "reason_code": "unauthorized"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "wallet apply failed", "reason_code": "wallet_error"})
	}
}
