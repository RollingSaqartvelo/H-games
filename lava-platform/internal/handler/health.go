package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	infrapostgres "github.com/lava-platform/internal/infrastructure/postgres"
	infraredis "github.com/lava-platform/internal/infrastructure/redis"
	"github.com/redis/go-redis/v9"
)

var startTime = time.Now()

type HealthHandler struct {
	db    *pgxpool.Pool
	cache *redis.Client
}

func NewHealth(db *pgxpool.Pool, cache *redis.Client) *HealthHandler {
	return &HealthHandler{db: db, cache: cache}
}

func (h *HealthHandler) Health(c *gin.Context) {
	ctx := c.Request.Context()

	dbHealth := infrapostgres.HealthCheck(ctx, h.db)
	cacheHealth := infraredis.HealthCheck(ctx, h.cache)

	httpStatus := http.StatusOK
	status := "ok"
	if dbHealth.Status != "healthy" || cacheHealth.Status != "healthy" {
		status = "degraded"
		httpStatus = http.StatusServiceUnavailable
	}

	c.JSON(httpStatus, gin.H{
		"status":   status,
		"uptime":   time.Since(startTime).String(),
		"version":  "1.0.0",
		"postgres": dbHealth,
		"redis":    cacheHealth,
	})
}

// Ready is a lightweight liveness probe — no dependency checks.
func (h *HealthHandler) Ready(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}
