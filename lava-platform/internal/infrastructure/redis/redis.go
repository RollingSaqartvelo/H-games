package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/lava-platform/internal/config"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

func New(ctx context.Context, cfg config.RedisConfig) (*redis.Client, error) {
	opts, err := redis.ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}

	opts.PoolSize = cfg.PoolSize
	opts.MinIdleConns = cfg.MinIdleConns
	opts.DialTimeout = cfg.DialTimeout
	opts.ReadTimeout = cfg.ReadTimeout
	opts.WriteTimeout = cfg.WriteTimeout

	client := redis.NewClient(opts)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}

	log.Info().
		Str("addr", opts.Addr).
		Int("pool_size", opts.PoolSize).
		Msg("redis connected")

	return client, nil
}
