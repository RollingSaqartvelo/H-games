package infrastructure

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lava-platform/internal/config"
	infrapostgres "github.com/lava-platform/internal/infrastructure/postgres"
	infraredis "github.com/lava-platform/internal/infrastructure/redis"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

const maxRetries = 5

// Infra holds all infrastructure clients. Caller must defer Close().
type Infra struct {
	DB    *pgxpool.Pool
	Cache *redis.Client
}

func New(ctx context.Context, cfg *config.Config) (*Infra, error) {
	var db *pgxpool.Pool
	if err := retry(ctx, "postgres", maxRetries, func() error {
		var err error
		db, err = infrapostgres.New(ctx, cfg.Database)
		return err
	}); err != nil {
		return nil, err
	}

	var cache *redis.Client
	if err := retry(ctx, "redis", maxRetries, func() error {
		var err error
		cache, err = infraredis.New(ctx, cfg.Redis)
		return err
	}); err != nil {
		db.Close()
		return nil, err
	}

	return &Infra{DB: db, Cache: cache}, nil
}

func (i *Infra) Close() {
	if i.Cache != nil {
		if err := i.Cache.Close(); err != nil {
			log.Error().Err(err).Msg("redis close error")
		}
	}
	if i.DB != nil {
		i.DB.Close()
	}
	log.Info().Msg("infrastructure closed")
}

func retry(ctx context.Context, name string, attempts int, fn func() error) error {
	backoff := time.Second
	for i := 1; i <= attempts; i++ {
		err := fn()
		if err == nil {
			return nil
		}
		if i == attempts {
			return fmt.Errorf("connect %s: max retries exceeded: %w", name, err)
		}
		log.Warn().
			Err(err).
			Str("service", name).
			Int("attempt", i).
			Dur("backoff", backoff).
			Msg("connection failed, retrying")

		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled waiting for %s: %w", name, ctx.Err())
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
	return nil
}
