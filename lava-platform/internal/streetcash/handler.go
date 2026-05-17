package streetcash

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lava-platform/internal/domain"
	"github.com/lava-platform/internal/middleware"
	walletSvc "github.com/lava-platform/internal/wallet/service"
	"github.com/shopspring/decimal"
)

type Handler struct {
	wallet *walletSvc.InternalWalletProvider
	mu     sync.Mutex
	nonces map[string]int64
}

func NewHandler(wallet *walletSvc.InternalWalletProvider) *Handler {
	return &Handler{wallet: wallet, nonces: make(map[string]int64)}
}

func (h *Handler) nextNonce(userID string) int64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.nonces[userID]++
	return h.nonces[userID]
}

func randSeed() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// POST /tma/v1/street-cash/spin
func (h *Handler) Spin(c *gin.Context) {
	sess := middleware.SessionFromCtx(c)
	if sess == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req struct {
		Bet float64 `json:"bet" binding:"required,min=0.10,max=10000"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	betDec := decimal.NewFromFloat(req.Bet)
	txID := fmt.Sprintf("sc-bet-%s-%d", sess.UserID, time.Now().UnixNano())

	debitResp, err := h.wallet.Debit(c.Request.Context(), &domain.DebitRequest{
		UserID:        sess.UserID,
		Amount:        betDec,
		Currency:      sess.Currency,
		TransactionID: txID,
		RoundID:       txID,
	})
	if err != nil {
		if err == domain.ErrInsufficientFunds {
			c.JSON(http.StatusPaymentRequired, gin.H{"error": "insufficient funds"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "wallet error"})
		return
	}

	seed := randSeed()
	nonce := h.nextNonce(sess.UserID)
	result := Spin(seed, nonce, req.Bet)

	finalBalance := debitResp.Balance
	if result.WinReel >= 0 && result.Payout > 0 {
		winDec := decimal.NewFromFloat(result.Payout)
		winTxID := fmt.Sprintf("sc-win-%s-%d", sess.UserID, time.Now().UnixNano())
		creditResp, err := h.wallet.Credit(c.Request.Context(), &domain.CreditRequest{
			UserID:        sess.UserID,
			Amount:        winDec,
			Currency:      sess.Currency,
			TransactionID: winTxID,
			RoundID:       txID,
		})
		if err == nil {
			finalBalance = creditResp.Balance
		}
	}

	balF, _ := finalBalance.Float64()
	c.JSON(http.StatusOK, gin.H{
		"server_seed_hash": result.ServerSeedHash,
		"nonce":            result.Nonce,
		"cursor_sym":       result.CursorSym,
		"reels":            result.Reels,
		"win_reel":         result.WinReel,
		"payout":           result.Payout,
		"bet":              result.Bet,
		"balance":          balF,
	})
}

// GET /tma/v1/street-cash/config
func (h *Handler) Config(c *gin.Context) {
	sess := middleware.SessionFromCtx(c)
	if sess == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	balResp, err := h.wallet.GetBalance(c.Request.Context(), &domain.BalanceRequest{
		UserID:   sess.UserID,
		Currency: sess.Currency,
	})
	bal := 0.0
	if err == nil {
		bal, _ = balResp.Balance.Float64()
	}
	c.JSON(http.StatusOK, gin.H{
		"balance":  bal,
		"currency": sess.Currency,
		"min_bet":  0.10,
		"max_bet":  1000.0,
		"mults":    RouletteMultipliers,
	})
}
