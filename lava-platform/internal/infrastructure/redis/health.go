package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type Health struct {
	Status      string `json:"status"`
	TotalConns  uint32 `json:"total_conns"`
	IdleConns   uint32 `json:"idle_conns"`
	StaleConns  uint32 `json:"stale_conns"`
}

func HealthCheck(ctx context.Context, client *redis.Client) Health {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return Health{Status: "unhealthy"}
	}

	stats := client.PoolStats()
	return Health{
		Status:     "healthy",
		TotalConns: stats.TotalConns,
		IdleConns:  stats.IdleConns,
		StaleConns: stats.StaleConns,
	}
}
