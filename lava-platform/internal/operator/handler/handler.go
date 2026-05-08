package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lava-platform/internal/domain"
	"github.com/lava-platform/internal/middleware"
	"github.com/lava-platform/internal/operator/service"
)

type Handler struct {
	svc service.OperatorService
}

func New(svc service.OperatorService) *Handler {
	return &Handler{svc: svc}
}

// ─── Admin endpoints (X-SYSTEM-KEY protected) ─────────────────────────────────

// CreateOperator POST /admin/v1/operators
func (h *Handler) CreateOperator(c *gin.Context) {
	var req struct {
		Name           string   `json:"name"            binding:"required"`
		CallbackURL    string   `json:"callback_url"    binding:"required"`
		AllowedOrigins []string `json:"allowed_origins"`
		RTPProfileID   *int64   `json:"rtp_profile_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	op, apiKey, secretKey, err := h.svc.Create(c.Request.Context(), &domain.CreateOperatorRequest{
		Name:           req.Name,
		CallbackURL:    req.CallbackURL,
		AllowedOrigins: req.AllowedOrigins,
		RTPProfileID:   req.RTPProfileID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Secret key is returned ONLY at creation time — never again.
	c.JSON(http.StatusCreated, gin.H{
		"operator":   safeOperator(op),
		"api_key":    apiKey,
		"secret_key": secretKey,
		"warning":    "Store the secret_key securely — it will not be shown again",
	})
}

// UpdateStatus PUT /admin/v1/operators/:id/status
func (h *Handler) UpdateStatus(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid operator id"})
		return
	}

	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	status := domain.OperatorStatus(req.Status)
	if status != domain.OperatorStatusActive &&
		status != domain.OperatorStatusInactive &&
		status != domain.OperatorStatusSuspended {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
		return
	}

	if err := h.svc.UpdateStatus(c.Request.Context(), id, status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"updated": true})
}

// ListOperators GET /admin/v1/operators
func (h *Handler) ListOperators(c *gin.Context) {
	ops, err := h.svc.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]any, len(ops))
	for i, op := range ops {
		out[i] = safeOperator(op)
	}
	c.JSON(http.StatusOK, gin.H{"operators": out})
}

// ListRTPProfiles GET /admin/v1/rtp-profiles
func (h *Handler) ListRTPProfiles(c *gin.Context) {
	profiles, err := h.svc.ListRTPProfiles(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"rtp_profiles": profiles})
}

// ─── Provider endpoints (HMAC + operator auth) ────────────────────────────────

// Me GET /api/v1/provider/me
func (h *Handler) Me(c *gin.Context) {
	op := middleware.OperatorFromCtx(c)
	if op == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "operator not found in context"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"operator": safeOperator(op)})
}

// ─── Response helpers ─────────────────────────────────────────────────────────

func safeOperator(op *domain.Operator) gin.H {
	out := gin.H{
		"id":             op.ID,
		"name":           op.Name,
		"api_key":        op.APIKey,
		"status":         op.Status,
		"allowed_origins": op.AllowedOrigins,
		"callback_url":   op.CallbackURL,
		"created_at":     op.CreatedAt,
		"updated_at":     op.UpdatedAt,
	}
	if op.RTPProfile != nil {
		out["rtp_profile"] = op.RTPProfile
	}
	return out
}
