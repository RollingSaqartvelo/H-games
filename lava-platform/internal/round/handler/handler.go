package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"github.com/lava-platform/internal/domain"
	"github.com/lava-platform/internal/middleware"
	"github.com/lava-platform/internal/round/engine"
	"github.com/lava-platform/internal/round/fair"
	roundrepo "github.com/lava-platform/internal/round/repository"
)

type Handler struct {
	eng      *engine.Engine
	repo     roundrepo.Repository
	gameType string
}

func New(eng *engine.Engine, repo roundrepo.Repository, gameType string) *Handler {
	return &Handler{eng: eng, repo: repo, gameType: gameType}
}

func (h *Handler) RegisterRoutes(bet gin.IRouter, public gin.IRouter) {
	bet.POST("/bet", h.PlaceBet)
	bet.POST("/cashout", h.Cashout)

	public.GET("/round/current", h.CurrentRound)
	public.GET("/round/history", h.RoundHistory)
	public.GET("/round/:id", h.GetRound)
	public.GET("/round/:id/verify", h.VerifyRound)
}

// ─── Request types ────────────────────────────────────────────────────────────

type BetRequest struct {
	Amount      string   `json:"amount" binding:"required"`
	Currency    string   `json:"currency" binding:"required"`
	AutoCashout *float64 `json:"auto_cashout"`
}

type CashoutRequest struct {
	BetID string `json:"bet_id" binding:"required"`
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

func (h *Handler) PlaceBet(c *gin.Context) {
	sess := middleware.SessionFromCtx(c)
	if sess == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "session required"})
		return
	}

	var req BetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	amount, err := decimal.NewFromString(req.Amount)
	if err != nil || !amount.IsPositive() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid amount"})
		return
	}

	autoCashout := 0.0
	if req.AutoCashout != nil {
		autoCashout = *req.AutoCashout
	}

	resp, err := h.eng.PlaceBet(c.Request.Context(), &engine.PlaceBetRequest{
		OperatorID:   sess.OperatorID,
		WalletUserID: sess.UserID,
		PlayerID:     sess.PlayerID,
		Currency:     req.Currency,
		Amount:       amount,
		AutoCashout:  autoCashout,
	})
	if err != nil {
		c.JSON(resolveErr(err), gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *Handler) Cashout(c *gin.Context) {
	sess := middleware.SessionFromCtx(c)
	if sess == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "session required"})
		return
	}

	var req CashoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.eng.Cashout(c.Request.Context(), &engine.CashoutRequest{
		BetID:        req.BetID,
		WalletUserID: sess.UserID,
		PlayerID:     sess.PlayerID,
		Currency:     sess.Currency,
	})
	if err != nil {
		c.JSON(resolveErr(err), gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) CurrentRound(c *gin.Context) {
	r := h.eng.CurrentRound()
	if r == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no active round"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"round":      r,
		"multiplier": h.eng.CurrentMultiplier(),
	})
}

func (h *Handler) RoundHistory(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit > 100 {
		limit = 100
	}

	rounds, err := h.repo.GetRoundHistory(c.Request.Context(), h.gameType, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rounds)
}

func (h *Handler) GetRound(c *gin.Context) {
	r, err := h.repo.GetRoundByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, r)
}

func (h *Handler) VerifyRound(c *gin.Context) {
	r, err := h.repo.GetRoundByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if r.State != domain.RoundStateCrashed && r.State != domain.RoundStateFinished {
		c.JSON(http.StatusConflict, gin.H{"error": "round not yet finished"})
		return
	}

	computed := fair.CrashPoint(r.ServerSeed, r.ClientSeed, r.Nonce, r.HouseEdge)
	c.JSON(http.StatusOK, gin.H{
		"round_id":             r.ID,
		"server_seed":          r.ServerSeed,
		"server_seed_hash":     r.ServerSeedHash,
		"client_seed":          r.ClientSeed,
		"nonce":                r.Nonce,
		"house_edge":           r.HouseEdge,
		"crash_point_db":       r.CrashPoint,
		"crash_point_computed": computed,
		"verified":             computed == r.CrashPoint,
	})
}

// ─── Error mapping ────────────────────────────────────────────────────────────

func resolveErr(err error) int {
	switch err {
	case domain.ErrInsufficientFunds:
		return http.StatusPaymentRequired
	case domain.ErrLockNotAcquired:
		return http.StatusConflict
	default:
		return http.StatusBadRequest
	}
}
