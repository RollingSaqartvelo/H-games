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

// GameDeps holds the per-game engine, hub and publisher.
type GameDeps struct {
	Engine *roundEngine.Engine
	Hub    *realtime.Hub
	Pub    *realtime.Publisher
}

// Deps holds pre-constructed engine-layer dependencies for all games.
type Deps struct {
	Outlaw GameDeps // outlaw_escape
	Granny GameDeps // granny_run
}

// Wire constructs all shared dependencies from infrastructure.
// Call once in main; pass the result to both New() and the schedulers.
func Wire(cfg *config.Config, infra *infrastructure.Infra) *Deps {
	wRepo := walletRepo.NewPostgres(infra.DB)
	locker := lock.New(infra.Cache)
	provider := walletSvc.New(wRepo, locker)
	rRepo := roundRepo.NewPostgres(infra.DB)

	// Outlaw Escape
	outlawPub := realtime.NewPublisher(infra.Cache, "outlaw_escape")
	outlawHub := realtime.NewHub()
	outlawCfg := roundEngine.DefaultConfig()
	outlawCfg.GameType = "outlaw_escape"
	outlawEng := roundEngine.New(outlawCfg, rRepo, provider, outlawPub, outlawHub, locker)

	// Granny Run
	grannyPub := realtime.NewPublisher(infra.Cache, "granny_run")
	grannyHub := realtime.NewHub()
	grannyCfg := roundEngine.DefaultConfig()
	grannyCfg.GameType = "granny_run"
	grannyEng := roundEngine.New(grannyCfg, rRepo, provider, grannyPub, grannyHub, locker)

	return &Deps{
		Outlaw: GameDeps{Engine: outlawEng, Hub: outlawHub, Pub: outlawPub},
		Granny: GameDeps{Engine: grannyEng, Hub: grannyHub, Pub: grannyPub},
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
	outlawHandler := roundHandler.New(deps.Outlaw.Engine, rRepo, "outlaw_escape")
	grannyHandler := roundHandler.New(deps.Granny.Engine, rRepo, "granny_run")

	outlawWS := realtime.NewWSHandler(deps.Outlaw.Hub)
	grannyWS  := realtime.NewWSHandler(deps.Granny.Hub)

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

	// ── Provider crash game (Outlaw Escape) ──────────────────────────────────
	crash := r.Group("/api/v1/crash")
	crash.Use(middleware.OperatorAuth(opSvc))
	{
		betGroup := crash.Group("")
		betGroup.Use(middleware.SessionValidate(sessSvc))
		outlawHandler.RegisterRoutes(betGroup, crash)
	}

	// ── Provider granny game (Granny Run) ─────────────────────────────────────
	granny := r.Group("/api/v1/granny")
	granny.Use(middleware.OperatorAuth(opSvc))
	{
		grannyBet := granny.Group("")
		grannyBet.Use(middleware.SessionValidate(sessSvc))
		grannyHandler.RegisterRoutes(grannyBet, granny)
	}

	// ── Telegram Mini App auth + bot webhook (public — no operator HMAC) ─────
	if cfg.Telegram.BotToken != "" {
		tmaValidator := telegram.NewValidator(cfg.Telegram.BotToken, cfg.Telegram.AuthMaxAge)
		tmaHandler   := telegram.NewHandler(tmaValidator, sessSvc, provider, cfg.Telegram.TMAOperatorID)
		botHandler   := telegram.NewBotHandler(cfg.Telegram.BotToken, cfg.Telegram.AppURL)

		// Telegram Login Widget handler
		testerRepo    := telegram.NewTesterRepository(infra.DB)
		widgetHandler := telegram.NewWidgetHandler(
			cfg.Telegram.BotToken, sessSvc, provider,
			cfg.Telegram.TMAOperatorID, botHandler.BotClient(), testerRepo,
		)

		tma := r.Group("/tma")
		tma.POST("/auth",         tmaHandler.Auth)
		tma.GET("/health",        tmaHandler.Health)
		tma.POST("/webhook",      botHandler.Webhook)
		tma.POST("/widget-auth",  widgetHandler.WidgetAuth)
		tma.POST("/set-username", widgetHandler.SetGameUsername)

		// Outlaw Escape TMA routes
		tmaOutlaw := tma.Group("/v1/crash")
		tmaOutlaw.Use(middleware.SessionValidate(sessSvc))
		outlawHandler.RegisterRoutes(tmaOutlaw, tmaOutlaw)

		// Granny Run TMA routes
		tmaGranny := tma.Group("/v1/granny")
		tmaGranny.Use(middleware.SessionValidate(sessSvc))
		grannyHandler.RegisterRoutes(tmaGranny, tmaGranny)

		go func() {
			if err := botHandler.RegisterWebhook(context.Background(), cfg.Telegram.AppURL); err != nil {
				_ = err
			}
		}()
	}

	// ── WebSocket ─────────────────────────────────────────────────────────────
	r.GET("/ws/crash",  outlawWS.ServeWS)
	r.GET("/ws/granny", grannyWS.ServeWS)

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
	// Root → landing page (game selector)
	r.GET("/", func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		c.File(distDir + "/landing.html")
	})
	r.GET("/landing.html", func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		c.File(distDir + "/landing.html")
	})
	r.StaticFile("/privacy.html", distDir+"/privacy.html")
	// granny.html served with no-cache so Telegram WebView always fetches the latest version
	r.GET("/granny.html", func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		c.File(distDir + "/granny.html")
	})

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
