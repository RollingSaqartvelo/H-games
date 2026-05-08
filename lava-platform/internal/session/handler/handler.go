package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lava-platform/internal/middleware"
	"github.com/lava-platform/internal/session/service"
)

type Handler struct {
	svc service.SessionService
}

func New(svc service.SessionService) *Handler {
	return &Handler{svc: svc}
}

// Create POST /api/v1/provider/session/create
// Called by operator's game server when launching a game for a player.
func (h *Handler) Create(c *gin.Context) {
	var req struct {
		PlayerID string `json:"player_id" binding:"required"`
		Currency string `json:"currency"  binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	op := middleware.OperatorFromCtx(c)

	sess, err := h.svc.Create(
		c.Request.Context(),
		op.ID,
		req.PlayerID,
		req.Currency,
		c.ClientIP(),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"session_token": sess.SessionToken,
		"player_id":     sess.PlayerID,
		"currency":      sess.Currency,
		"expires_at":    sess.ExpiresAt,
	})
}

// Validate POST /api/v1/provider/session/validate
// Called by operator's game server to check if a session is still valid.
func (h *Handler) Validate(c *gin.Context) {
	var req struct {
		SessionToken string `json:"session_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sess, err := h.svc.Validate(c.Request.Context(), req.SessionToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error(), "valid": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":      true,
		"player_id":  sess.PlayerID,
		"currency":   sess.Currency,
		"expires_at": sess.ExpiresAt,
	})
}

// Revoke DELETE /api/v1/provider/session/revoke
func (h *Handler) Revoke(c *gin.Context) {
	var req struct {
		SessionToken string `json:"session_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.svc.Revoke(c.Request.Context(), req.SessionToken); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"revoked": true})
}
