package telegram

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lava-platform/internal/domain"
	"github.com/lava-platform/internal/session/service"
	"github.com/rs/zerolog/log"
	"github.com/shopspring/decimal"
)

const tmaSignupBonus = "1000.00" // demo balance for new TMA players

// Handler exposes the /tma/auth endpoint.
// It is intentionally NOT protected by operator HMAC middleware — Telegram's
// own initData signature replaces that trust layer for Mini App players.
type Handler struct {
	validator     *Validator
	sessionSvc    service.SessionService
	walletSvc     domain.WalletProvider
	tmaOperatorID int64
}

func NewHandler(v *Validator, svc service.SessionService, wallet domain.WalletProvider, operatorID int64) *Handler {
	return &Handler{
		validator:     v,
		sessionSvc:    svc,
		walletSvc:     wallet,
		tmaOperatorID: operatorID,
	}
}

// AuthRequest is the JSON body sent by the Mini App frontend.
type AuthRequest struct {
	InitData string `json:"init_data" binding:"required"`
	Currency string `json:"currency"`
}

// AuthResponse is returned to the frontend on success.
type AuthResponse struct {
	Token     string `json:"token"`
	PlayerID  string `json:"player_id"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
	Balance   string `json:"balance"`
}

// Auth exchanges Telegram initData for a platform session token.
// On first login the player receives a demo balance of $1000.
//
// POST /tma/auth
func (h *Handler) Auth(c *gin.Context) {
	var req AuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	currency := req.Currency
	if currency == "" {
		currency = "USD"
	}

	// Validate Telegram signature
	user, err := h.validator.Validate(req.InitData)
	if err != nil {
		status := http.StatusUnauthorized
		msg := "invalid telegram auth"
		if errors.Is(err, ErrAuthExpired) {
			msg = "telegram auth expired — reopen the app"
		}
		c.JSON(status, gin.H{"error": msg})
		return
	}

	playerID := strconv.FormatInt(user.ID, 10)

	// Create (or refresh) a platform session for this Telegram user
	sess, err := h.sessionSvc.Create(
		c.Request.Context(),
		h.tmaOperatorID,
		playerID,
		currency,
		c.ClientIP(),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "session creation failed"})
		return
	}

	// Credit signup bonus — idempotent: same TransactionID is a no-op on repeat logins
	walletUserID := fmt.Sprintf("op%d:%s", h.tmaOperatorID, playerID)
	bonus, _ := decimal.NewFromString(tmaSignupBonus)

	balance := ""
	creditResp, err := h.walletSvc.Credit(c.Request.Context(), &domain.CreditRequest{
		UserID:        walletUserID,
		Amount:        bonus,
		Currency:      currency,
		TransactionID: fmt.Sprintf("tma_signup_%s", walletUserID),
	})
	if err != nil {
		// Idempotency key already exists — fetch current balance
		log.Debug().Err(err).Str("player", playerID).Msg("tma signup credit (already exists)")
		balResp, berr := h.walletSvc.GetBalance(c.Request.Context(), &domain.BalanceRequest{
			UserID:   walletUserID,
			Currency: currency,
		})
		if berr == nil {
			balance = balResp.Balance.StringFixed(2)
			// Re-credit if balance is zero (wallet was reset or never applied)
			if balResp.Balance.IsZero() {
				resetTxID := fmt.Sprintf("tma_signup_reset_%s_%s", walletUserID,
					time.Now().UTC().Format("2006-01-02-15"))
				if retryResp, rerr := h.walletSvc.Credit(c.Request.Context(), &domain.CreditRequest{
					UserID:        walletUserID,
					Amount:        bonus,
					Currency:      currency,
					TransactionID: resetTxID,
				}); rerr == nil {
					balance = retryResp.Balance.StringFixed(2)
					log.Info().Str("player", playerID).Str("tx", resetTxID).Msg("tma balance reset bonus applied")
				}
			}
		}
	} else {
		balance = creditResp.Balance.StringFixed(2)
	}

	c.JSON(http.StatusOK, AuthResponse{
		Token:     sess.SessionToken,
		PlayerID:  playerID,
		FirstName: user.FirstName,
		Username:  user.Username,
		Balance:   balance,
	})
}

// Health is a simple liveness probe for the TMA subsystem.
// GET /tma/health
func (h *Handler) Health(c *gin.Context) {
	enabled := h.validator != nil && h.tmaOperatorID > 0
	c.JSON(http.StatusOK, gin.H{"tma": enabled})
}
