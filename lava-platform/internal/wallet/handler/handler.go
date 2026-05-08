package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lava-platform/internal/callback"
	"github.com/lava-platform/internal/domain"
	"github.com/lava-platform/internal/middleware"
	"github.com/shopspring/decimal"
)

type Handler struct {
	provider    domain.WalletProvider
	callbackSvc callback.Service
}

func New(provider domain.WalletProvider, callbackSvc callback.Service) *Handler {
	return &Handler{provider: provider, callbackSvc: callbackSvc}
}

// ─── Request / Response structs ───────────────────────────────────────────────

type balanceResp struct {
	UserID   string `json:"user_id"`
	Balance  string `json:"balance"`
	Currency string `json:"currency"`
}

type betReq struct {
	TransactionID string `json:"transaction_id" binding:"required"`
	RoundID       string `json:"round_id"       binding:"required"`
	Amount        string `json:"amount"         binding:"required"`
	Currency      string `json:"currency"       binding:"required"`
}

type winReq struct {
	TransactionID string `json:"transaction_id" binding:"required"`
	RoundID       string `json:"round_id"       binding:"required"`
	Amount        string `json:"amount"         binding:"required"`
	Currency      string `json:"currency"       binding:"required"`
}

type rollbackReq struct {
	TransactionID string `json:"transaction_id"          binding:"required"`
	OriginalTxID  string `json:"original_transaction_id" binding:"required"`
	RoundID       string `json:"round_id"                binding:"required"`
	Currency      string `json:"currency"                binding:"required"`
}

type txResp struct {
	TransactionID string `json:"transaction_id"`
	Balance       string `json:"balance"`
	Currency      string `json:"currency"`
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

// Balance POST /api/v1/wallet/balance
func (h *Handler) Balance(c *gin.Context) {
	sess := middleware.SessionFromCtx(c)

	resp, err := h.provider.GetBalance(c.Request.Context(), &domain.BalanceRequest{
		UserID:   sess.UserID,
		Currency: sess.Currency,
	})
	if err != nil {
		writeErr(c, err)
		return
	}

	c.JSON(http.StatusOK, balanceResp{
		UserID:   sess.PlayerID,
		Balance:  resp.Balance.StringFixed(2),
		Currency: resp.Currency,
	})
}

// Bet POST /api/v1/wallet/bet
func (h *Handler) Bet(c *gin.Context) {
	var req betReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	amount, err := decimal.NewFromString(req.Amount)
	if err != nil || !amount.IsPositive() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount must be a positive number"})
		return
	}

	sess := middleware.SessionFromCtx(c)

	resp, err := h.provider.Debit(c.Request.Context(), &domain.DebitRequest{
		UserID:        sess.UserID,
		Amount:        amount,
		Currency:      req.Currency,
		TransactionID: req.TransactionID,
		RoundID:       req.RoundID,
	})
	if err != nil {
		writeErr(c, err)
		return
	}

	h.sendCallback(c, "bet", req.TransactionID, req.RoundID, sess, amount, resp.Balance)

	c.JSON(http.StatusOK, txResp{
		TransactionID: resp.TransactionID,
		Balance:       resp.Balance.StringFixed(2),
		Currency:      resp.Currency,
	})
}

// Win POST /api/v1/wallet/win
func (h *Handler) Win(c *gin.Context) {
	var req winReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	amount, err := decimal.NewFromString(req.Amount)
	if err != nil || !amount.IsPositive() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount must be a positive number"})
		return
	}

	sess := middleware.SessionFromCtx(c)

	resp, err := h.provider.Credit(c.Request.Context(), &domain.CreditRequest{
		UserID:        sess.UserID,
		Amount:        amount,
		Currency:      req.Currency,
		TransactionID: req.TransactionID,
		RoundID:       req.RoundID,
	})
	if err != nil {
		writeErr(c, err)
		return
	}

	h.sendCallback(c, "win", req.TransactionID, req.RoundID, sess, amount, resp.Balance)

	c.JSON(http.StatusOK, txResp{
		TransactionID: resp.TransactionID,
		Balance:       resp.Balance.StringFixed(2),
		Currency:      resp.Currency,
	})
}

// Rollback POST /api/v1/wallet/rollback
func (h *Handler) Rollback(c *gin.Context) {
	var req rollbackReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sess := middleware.SessionFromCtx(c)

	resp, err := h.provider.Rollback(c.Request.Context(), &domain.RollbackRequest{
		UserID:        sess.UserID,
		TransactionID: req.TransactionID,
		OriginalTxID:  req.OriginalTxID,
		RoundID:       req.RoundID,
		Currency:      req.Currency,
	})
	if err != nil {
		writeErr(c, err)
		return
	}

	h.sendCallback(c, "rollback", req.TransactionID, req.RoundID, sess, resp.Balance, resp.Balance)

	c.JSON(http.StatusOK, txResp{
		TransactionID: resp.TransactionID,
		Balance:       resp.Balance.StringFixed(2),
		Currency:      resp.Currency,
	})
}

// ─── Callback dispatch ────────────────────────────────────────────────────────

func (h *Handler) sendCallback(
	c *gin.Context,
	eventType, txID, roundID string,
	sess *middleware.Session,
	amount, balance decimal.Decimal,
) {
	op := middleware.OperatorFromCtx(c)
	if op == nil || op.CallbackURL == "" {
		return
	}
	h.callbackSvc.Send(&callback.Event{
		OperatorCallbackURL: op.CallbackURL,
		OperatorSecretKey:   op.SecretKey,
		EventType:           eventType,
		TransactionID:       txID,
		PlayerID:            sess.PlayerID,
		RoundID:             roundID,
		Amount:              amount,
		Currency:            sess.Currency,
		Balance:             balance,
	})
}

// ─── Error mapping ────────────────────────────────────────────────────────────

func writeErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrInsufficientFunds):
		c.JSON(http.StatusPaymentRequired, gin.H{"error": err.Error(), "code": "INSUFFICIENT_FUNDS"})
	case errors.Is(err, domain.ErrDuplicateTransaction):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "code": "DUPLICATE_TRANSACTION"})
	case errors.Is(err, domain.ErrTransactionNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error(), "code": "TRANSACTION_NOT_FOUND"})
	case errors.Is(err, domain.ErrWalletNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error(), "code": "WALLET_NOT_FOUND"})
	case errors.Is(err, domain.ErrCurrencyMismatch):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": "CURRENCY_MISMATCH"})
	case errors.Is(err, domain.ErrTransactionAlreadyRolledBack):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "code": "ALREADY_ROLLED_BACK"})
	case errors.Is(err, domain.ErrOriginalTxNotBet):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": "INVALID_ORIGINAL_TX"})
	case errors.Is(err, domain.ErrInvalidAmount):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": "INVALID_AMOUNT"})
	case errors.Is(err, domain.ErrLockNotAcquired):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error(), "code": "LOCKED"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error", "code": "INTERNAL"})
	}
}
