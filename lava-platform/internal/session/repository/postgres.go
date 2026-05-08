package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lava-platform/internal/domain"
	"github.com/redis/go-redis/v9"
)

const sessionCachePrefix = "session:"

type SessionRepository interface {
	Create(ctx context.Context, req *domain.CreateSessionRequest, token string) (*domain.OperatorSession, error)
	GetByToken(ctx context.Context, token string) (*domain.OperatorSession, error)
	Revoke(ctx context.Context, token string) error
	DeleteExpired(ctx context.Context) (int64, error)
}

type postgresRepo struct {
	pool  *pgxpool.Pool
	cache *redis.Client
	ttl   time.Duration
}

func NewPostgres(pool *pgxpool.Pool, cache *redis.Client, ttl time.Duration) SessionRepository {
	return &postgresRepo{pool: pool, cache: cache, ttl: ttl}
}

func (r *postgresRepo) Create(ctx context.Context, req *domain.CreateSessionRequest, token string) (*domain.OperatorSession, error) {
	ttl := req.TTL
	if ttl == 0 {
		ttl = r.ttl
	}
	expiresAt := time.Now().Add(ttl)

	const q = `
		INSERT INTO operator_sessions
			(operator_id, player_id, session_token, currency, ip, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, operator_id, player_id, session_token, currency, ip, active, expires_at, created_at`

	sess, err := scanSession(r.pool.QueryRow(ctx, q,
		req.OperatorID, req.PlayerID, token, req.Currency, req.IP, expiresAt,
	))
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	r.cacheSet(ctx, sess)
	return sess, nil
}

func (r *postgresRepo) GetByToken(ctx context.Context, token string) (*domain.OperatorSession, error) {
	// Try Redis cache first
	if sess, err := r.cacheGet(ctx, token); err == nil {
		return sess, nil
	}

	// Fallback to PostgreSQL
	const q = `
		SELECT id, operator_id, player_id, session_token, currency, ip, active, expires_at, created_at
		FROM operator_sessions
		WHERE session_token = $1`

	sess, err := scanSession(r.pool.QueryRow(ctx, q, token))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrSessionNotFound
	}
	if err != nil {
		return nil, err
	}

	if !sess.Active {
		return nil, domain.ErrSessionRevoked
	}
	if time.Now().After(sess.ExpiresAt) {
		return nil, domain.ErrSessionExpired
	}

	r.cacheSet(ctx, sess)
	return sess, nil
}

func (r *postgresRepo) Revoke(ctx context.Context, token string) error {
	const q = `UPDATE operator_sessions SET active = FALSE WHERE session_token = $1`
	tag, err := r.pool.Exec(ctx, q, token)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrSessionNotFound
	}
	r.cache.Del(ctx, sessionCachePrefix+token)
	return nil
}

func (r *postgresRepo) DeleteExpired(ctx context.Context) (int64, error) {
	const q = `DELETE FROM operator_sessions WHERE expires_at < NOW() OR active = FALSE`
	tag, err := r.pool.Exec(ctx, q)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// ─── Cache helpers ────────────────────────────────────────────────────────────

func (r *postgresRepo) cacheSet(ctx context.Context, sess *domain.OperatorSession) {
	data, err := json.Marshal(sess)
	if err != nil {
		return
	}
	ttl := time.Until(sess.ExpiresAt)
	if ttl <= 0 {
		return
	}
	_ = r.cache.Set(ctx, sessionCachePrefix+sess.SessionToken, data, ttl).Err()
}

func (r *postgresRepo) cacheGet(ctx context.Context, token string) (*domain.OperatorSession, error) {
	data, err := r.cache.Get(ctx, sessionCachePrefix+token).Bytes()
	if err != nil {
		return nil, err
	}
	var sess domain.OperatorSession
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, err
	}
	if !sess.Active || time.Now().After(sess.ExpiresAt) {
		r.cache.Del(ctx, sessionCachePrefix+token)
		return nil, domain.ErrSessionExpired
	}
	return &sess, nil
}

func scanSession(row interface{ Scan(dest ...any) error }) (*domain.OperatorSession, error) {
	var s domain.OperatorSession
	err := row.Scan(
		&s.ID, &s.OperatorID, &s.PlayerID, &s.SessionToken,
		&s.Currency, &s.IP, &s.Active, &s.ExpiresAt, &s.CreatedAt,
	)
	return &s, err
}
