package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Health struct {
	Status      string `json:"status"`
	TotalConns  int32  `json:"total_conns"`
	IdleConns   int32  `json:"idle_conns"`
	AcquiredConns int32 `json:"acquired_conns"`
}

func HealthCheck(ctx context.Context, pool *pgxpool.Pool) Health {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		return Health{Status: "unhealthy"}
	}

	stat := pool.Stat()
	return Health{
		Status:        "healthy",
		TotalConns:    stat.TotalConns(),
		IdleConns:     stat.IdleConns(),
		AcquiredConns: stat.AcquiredConns(),
	}
}
