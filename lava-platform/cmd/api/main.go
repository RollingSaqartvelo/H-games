package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/joho/godotenv"
	"github.com/lava-platform/internal/config"
	"github.com/lava-platform/internal/infrastructure"
	"github.com/lava-platform/internal/router"
	"github.com/lava-platform/internal/round/scheduler"
	"github.com/lava-platform/migrations"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	_ = godotenv.Load()

	cfg := config.Load()
	setupLogger(cfg.Log)

	startCtx, startCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer startCancel()

	infra, err := infrastructure.New(startCtx, cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("infrastructure init failed")
	}
	defer infra.Close()

	runMigrations(cfg.Database.DSN)

	// Wire shared engine-layer dependencies (one set per game).
	deps := router.Wire(cfg, infra)

	// Background context that lives for the full server lifetime.
	bgCtx, bgCancel := context.WithCancel(context.Background())
	defer bgCancel()

	// Outlaw Escape — hub, pub/sub, scheduler.
	go deps.Outlaw.Hub.Run(bgCtx)
	go deps.Outlaw.Pub.Subscribe(bgCtx, deps.Outlaw.Hub)
	go scheduler.New(deps.Outlaw.Engine, deps.Outlaw.Engine.Cfg()).Run(bgCtx)

	// Granny Run — hub, pub/sub, scheduler.
	go deps.Granny.Hub.Run(bgCtx)
	go deps.Granny.Pub.Subscribe(bgCtx, deps.Granny.Hub)
	go scheduler.New(deps.Granny.Engine, deps.Granny.Engine.Cfg()).Run(bgCtx)

	r := router.New(cfg, infra, deps)

	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info().Str("addr", srv.Addr).Msg("server started")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("shutting down...")

	// Stop background goroutines first (scheduler, hub, pub/sub).
	bgCancel()

	shutCtx, shutCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer shutCancel()

	if err := srv.Shutdown(shutCtx); err != nil {
		log.Error().Err(err).Msg("forced shutdown")
	}

	log.Info().Msg("server stopped")
}

func runMigrations(dsn string) {
	// golang-migrate pgx/v5 driver requires scheme "pgx5://"
	driverDSN := strings.NewReplacer("postgres://", "pgx5://", "postgresql://", "pgx5://").Replace(dsn)

	// Use embedded FS — no file path issues on any OS
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		log.Fatal().Err(err).Msg("migrate: source init failed")
	}

	m, err := migrate.NewWithSourceInstance("iofs", src, driverDSN)
	if err != nil {
		log.Fatal().Err(err).Msg("migrate: init failed")
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Fatal().Err(err).Msg("migrate: up failed")
	}
	log.Info().Msg("migrations applied")
}

func setupLogger(cfg config.LogConfig) {
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	if cfg.Pretty {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	} else {
		zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	}
}
