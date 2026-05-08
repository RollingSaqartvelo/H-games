package router

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	adminHandler "github.com/lava-platform/internal/admin"
	"github.com/lava-platform/internal/callback"
	"github.com/lava-platform/internal/config"
	healthHandler "github.com/lava-platform/internal/handler"
	"github.com/lava-platform/internal/infrastructure"
	"github.com/lava-platform/internal/lock"
	"github.com/lava-platform/internal/middleware"
	operatorHandler "github.com/lava-platform/internal/operator/handler"
	operatorRepo "github.com/lava-platform/internal/operator/repository"
	operatorSvc "github.com/lava-platform/internal/operator/service"
	"github.com/lava-platform/internal/realtime"
	roundEngine "github.com/lava-platform/internal/round/engine"
	roundHandler "github.com/lava-platform/internal/round/handler"
	roundRepo "github.com/lava-platform/internal/round/repository"
	sessionHandler "github.com/lava-platform/internal/session/handler"
	sessionRepo "github.com/lava-platform/internal/session/repository"
	sessionSvc "github.com/lava-platform/internal/session/service"
	"github.com/lava-platform/internal/telegram"
	walletHandler "github.com/lava-platform/internal/wallet/handler"
	walletRepo "github.com/lava-platform/internal/wallet/repository"
	walletSvc "github.com/lava-platform/internal/wallet/service"
)

// Deps holds pre-constructed engine-layer dependencies that are shared
// between the router and the scheduler (both need the same Engine instance).
type Deps struct {
	Engine *roundEngine.Engine
	Hub    *realtime.Hub
	Pub    *realtime.Publisher
}

// Wire constructs all shared dependencies from infrastructure.
// Call once in main; pass the result to both New() and the scheduler.
func Wire(cfg *config.Config, infra *infrastructure.Infra) *Deps {
	pub := realtime.NewPublisher(infra.Cache)
	hub := realtime.NewHub()

	wRepo := walletRepo.NewPostgres(infra.DB)
	locker := lock.New(infra.Cache)
	provider := walletSvc.New(wRepo, locker)

	rRepo := roundRepo.NewPostgres(infra.DB)
	eng := roundEngine.New(roundEngine.DefaultConfig(), rRepo, provider, pub, hub, locker)

	return &Deps{
		Engine: eng,
		Hub:    hub,
		Pub:    pub,
	}
}

// New builds the Gin router. deps must come from Wire().
func New(cfg *config.Config, infra *infrastructure.Infra, deps *Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()

	// ── Global middleware ─────────────────────────────────────────────────────
	r.Use(middleware.RequestID())
	r.Use(middleware.Recovery())
	r.Use(middleware.Logger())
	r.Use(middleware.CORS(cfg.Server.AllowedOrigins))
	r.Use(middleware.RateLimit(cfg.Server.RateLimit, cfg.Server.RateBurst))

	// ── Health probes ─────────────────────────────────────────────────────────
	health := healthHandler.NewHealth(infra.DB, infra.Cache)
	r.GET("/healthz", health.Health)
	r.GET("/readyz", health.Ready)

	// ── Wire dependencies ─────────────────────────────────────────────────────
	opRepo := operatorRepo.NewPostgres(infra.DB)
	opSvc := operatorSvc.New(opRepo, infra.Cache, cfg.Operator.OperatorCacheTTL)
	opHandler := operatorHandler.New(opSvc)

	sessRepo := sessionRepo.NewPostgres(infra.DB, infra.Cache, cfg.Operator.SessionTTL)
	sessSvc := sessionSvc.New(sessRepo, cfg.Operator.SessionTTL)
	sessHandler := sessionHandler.New(sessSvc)

	callbackSvc := callback.NewService(cfg.Operator.CallbackTimeout, cfg.Operator.CallbackRetries)

	wRepo := walletRepo.NewPostgres(infra.DB)
	locker := lock.New(infra.Cache)
	provider := walletSvc.New(wRepo, locker)
	wallet := walletHandler.New(provider, callbackSvc)

	rRepo := roundRepo.NewPostgres(infra.DB)
	rHandler := roundHandler.New(deps.Engine, rRepo)

	wsHandler := realtime.NewWSHandler(deps.Hub)

	// ── Admin API (X-SYSTEM-KEY) ──────────────────────────────────────────────
	adm := adminHandler.New(infra.DB)
	r.GET("/admin", adm.Dashboard)

	admin := r.Group("/admin/v1")
	admin.Use(middleware.SystemAuth(cfg.Operator.SystemAPIKey))
	{
		admin.POST("/operators",           opHandler.CreateOperator)
		admin.GET("/operators",            opHandler.ListOperators)
		admin.PUT("/operators/:id/status", opHandler.UpdateStatus)
		admin.GET("/rtp-profiles",         opHandler.ListRTPProfiles)
		admin.GET("/stats",                adm.Stats)
	}

	// ── Provider API (HMAC signed) ────────────────────────────────────────────
	providerAPI := r.Group("/api/v1")
	providerAPI.Use(middleware.OperatorAuth(opSvc))
	{
		providerAPI.GET("/provider/me", opHandler.Me)

		sess := providerAPI.Group("/provider/session")
		{
			sess.POST("/create",   sessHandler.Create)
			sess.POST("/validate", sessHandler.Validate)
			sess.DELETE("/revoke", sessHandler.Revoke)
		}

		w := providerAPI.Group("/wallet")
		w.Use(middleware.SessionValidate(sessSvc))
		{
			w.POST("/balance",  wallet.Balance)
			w.POST("/bet",      wallet.Bet)
			w.POST("/win",      wallet.Win)
			w.POST("/rollback", wallet.Rollback)
		}
	}

	// ── Crash game ────────────────────────────────────────────────────────────
	crash := r.Group("/api/v1/crash")
	crash.Use(middleware.OperatorAuth(opSvc))
	{
		// Mutations require a player session
		betGroup := crash.Group("")
		betGroup.Use(middleware.SessionValidate(sessSvc))

		rHandler.RegisterRoutes(betGroup, crash)
	}

	// ── Telegram Mini App auth + bot webhook (public — no operator HMAC) ────
	if cfg.Telegram.BotToken != "" {
		tmaValidator := telegram.NewValidator(cfg.Telegram.BotToken, cfg.Telegram.AuthMaxAge)
		tmaHandler   := telegram.NewHandler(tmaValidator, sessSvc, provider, cfg.Telegram.TMAOperatorID)
		botHandler   := telegram.NewBotHandler(cfg.Telegram.BotToken, cfg.Telegram.AppURL)

		tma := r.Group("/tma")
		tma.POST("/auth",    tmaHandler.Auth)
		tma.GET("/health",   tmaHandler.Health)
		tma.POST("/webhook", botHandler.Webhook)

		// Game routes for TMA players — session auth only, no operator HMAC.
		// The session itself proves operator identity (TMA operator ID is embedded in the token).
		tmaGame := tma.Group("/v1/crash")
		tmaGame.Use(middleware.SessionValidate(sessSvc))
		rHandler.RegisterRoutes(tmaGame, tmaGame)

		// Register webhook with Telegram asynchronously (non-fatal if it fails)
		go func() {
			if err := botHandler.RegisterWebhook(context.Background(), cfg.Telegram.AppURL); err != nil {
				// Log only — webhook can be registered manually via setup-tma.sh
				_ = err
			}
		}()
	}

	// ── WebSocket ─────────────────────────────────────────────────────────────
	r.GET("/ws/crash", wsHandler.ServeWS)

	// ── Frontend SPA ──────────────────────────────────────────────────────────
	// Serve built React app from frontend/dist if it exists.
	serveFrontend(r)

	return r
}

// serveFrontend mounts the built React SPA at the root.
// API, WS, and TMA routes take priority — everything else falls through to index.html.
func serveFrontend(r *gin.Engine) {
	const distDir = "frontend/dist"
	if _, err := os.Stat(distDir); os.IsNotExist(err) {
		return
	}

	// Serve static assets (JS, CSS, images)
	r.Static("/assets", distDir+"/assets")

	// Serve video files (betting-loop.mp4 etc.)
	r.Static("/video", distDir+"/video")

	// Serve audio files
	r.Static("/audio", distDir+"/audio")

	// Serve other static root files (favicon, manifest, etc.)
	r.StaticFile("/favicon.ico", distDir+"/favicon.ico")

	// SPA catch-all: serve index.html for any non-API path
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api/") ||
			strings.HasPrefix(path, "/ws/") ||
			strings.HasPrefix(path, "/tma/") ||
			strings.HasPrefix(path, "/admin") ||
			strings.HasPrefix(path, "/healthz") ||
			strings.HasPrefix(path, "/readyz") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		c.File(distDir + "/index.html")
	})
}
