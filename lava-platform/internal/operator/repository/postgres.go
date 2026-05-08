package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lava-platform/internal/domain"
)

type OperatorRepository interface {
	GetByID(ctx context.Context, id int64) (*domain.Operator, error)
	GetByAPIKey(ctx context.Context, apiKey string) (*domain.Operator, error)
	Create(ctx context.Context, req *domain.CreateOperatorRequest, apiKey, secretKey string) (*domain.Operator, error)
	UpdateStatus(ctx context.Context, id int64, status domain.OperatorStatus) error
	List(ctx context.Context) ([]*domain.Operator, error)
	GetRTPProfile(ctx context.Context, id int64) (*domain.RTPProfile, error)
	ListRTPProfiles(ctx context.Context) ([]*domain.RTPProfile, error)
}

type postgresRepo struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) OperatorRepository {
	return &postgresRepo{pool: pool}
}

func (r *postgresRepo) GetByID(ctx context.Context, id int64) (*domain.Operator, error) {
	const q = `
		SELECT o.id, o.name, o.api_key, o.secret_key, o.status,
		       o.allowed_origins, o.callback_url, o.default_rtp_profile_id,
		       o.created_at, o.updated_at,
		       rp.id, rp.name, rp.target_rtp::text, rp.created_at
		FROM operators o
		LEFT JOIN rtp_profiles rp ON rp.id = o.default_rtp_profile_id
		WHERE o.id = $1`

	op, err := scanOperator(r.pool.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrOperatorNotFound
	}
	return op, err
}

func (r *postgresRepo) GetByAPIKey(ctx context.Context, apiKey string) (*domain.Operator, error) {
	const q = `
		SELECT o.id, o.name, o.api_key, o.secret_key, o.status,
		       o.allowed_origins, o.callback_url, o.default_rtp_profile_id,
		       o.created_at, o.updated_at,
		       rp.id, rp.name, rp.target_rtp::text, rp.created_at
		FROM operators o
		LEFT JOIN rtp_profiles rp ON rp.id = o.default_rtp_profile_id
		WHERE o.api_key = $1`

	op, err := scanOperator(r.pool.QueryRow(ctx, q, apiKey))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrOperatorNotFound
	}
	return op, err
}

func (r *postgresRepo) Create(ctx context.Context, req *domain.CreateOperatorRequest, apiKey, secretKey string) (*domain.Operator, error) {
	const q = `
		INSERT INTO operators (name, api_key, secret_key, callback_url, allowed_origins, default_rtp_profile_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, name, api_key, secret_key, status, allowed_origins,
		          callback_url, default_rtp_profile_id, created_at, updated_at`

	var rtpID *int64
	if req.RTPProfileID != nil {
		rtpID = req.RTPProfileID
	}

	row := r.pool.QueryRow(ctx, q,
		req.Name, apiKey, secretKey, req.CallbackURL,
		req.AllowedOrigins, rtpID,
	)

	var op domain.Operator
	var nullRTPID sql.NullInt64
	err := row.Scan(
		&op.ID, &op.Name, &op.APIKey, &op.SecretKey, &op.Status,
		&op.AllowedOrigins, &op.CallbackURL, &nullRTPID,
		&op.CreatedAt, &op.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create operator: %w", err)
	}
	op.DefaultRTPProfileID = nullRTPID
	return &op, nil
}

func (r *postgresRepo) UpdateStatus(ctx context.Context, id int64, status domain.OperatorStatus) error {
	const q = `UPDATE operators SET status = $1, updated_at = NOW() WHERE id = $2`
	tag, err := r.pool.Exec(ctx, q, status, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrOperatorNotFound
	}
	return nil
}

func (r *postgresRepo) List(ctx context.Context) ([]*domain.Operator, error) {
	const q = `
		SELECT o.id, o.name, o.api_key, o.secret_key, o.status,
		       o.allowed_origins, o.callback_url, o.default_rtp_profile_id,
		       o.created_at, o.updated_at,
		       rp.id, rp.name, rp.target_rtp::text, rp.created_at
		FROM operators o
		LEFT JOIN rtp_profiles rp ON rp.id = o.default_rtp_profile_id
		ORDER BY o.created_at DESC`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ops []*domain.Operator
	for rows.Next() {
		op, err := scanOperator(rows)
		if err != nil {
			return nil, err
		}
		ops = append(ops, op)
	}
	return ops, rows.Err()
}

func (r *postgresRepo) GetRTPProfile(ctx context.Context, id int64) (*domain.RTPProfile, error) {
	const q = `SELECT id, name, target_rtp::text, created_at FROM rtp_profiles WHERE id = $1`
	var p domain.RTPProfile
	var rtp string
	err := r.pool.QueryRow(ctx, q, id).Scan(&p.ID, &p.Name, &rtp, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("rtp profile not found")
	}
	if err != nil {
		return nil, err
	}
	if _, err := fmt.Sscanf(rtp, "%f", &p.TargetRTP); err != nil {
		return nil, fmt.Errorf("parse rtp: %w", err)
	}
	return &p, nil
}

func (r *postgresRepo) ListRTPProfiles(ctx context.Context) ([]*domain.RTPProfile, error) {
	const q = `SELECT id, name, target_rtp::text, created_at FROM rtp_profiles ORDER BY id`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var profiles []*domain.RTPProfile
	for rows.Next() {
		var p domain.RTPProfile
		var rtp string
		if err := rows.Scan(&p.ID, &p.Name, &rtp, &p.CreatedAt); err != nil {
			return nil, err
		}
		fmt.Sscanf(rtp, "%f", &p.TargetRTP)
		profiles = append(profiles, &p)
	}
	return profiles, rows.Err()
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

type rowScanner interface {
	Scan(dest ...any) error
}

func scanOperator(row rowScanner) (*domain.Operator, error) {
	var op domain.Operator
	var nullRTPID sql.NullInt64

	// RTP columns (nullable via LEFT JOIN)
	var rtpID sql.NullInt64
	var rtpName sql.NullString
	var rtpValue sql.NullString
	var rtpCreated sql.NullTime

	err := row.Scan(
		&op.ID, &op.Name, &op.APIKey, &op.SecretKey, &op.Status,
		&op.AllowedOrigins, &op.CallbackURL, &nullRTPID,
		&op.CreatedAt, &op.UpdatedAt,
		&rtpID, &rtpName, &rtpValue, &rtpCreated,
	)
	if err != nil {
		return nil, err
	}
	op.DefaultRTPProfileID = nullRTPID

	if rtpID.Valid {
		p := &domain.RTPProfile{
			ID:        rtpID.Int64,
			Name:      rtpName.String,
			CreatedAt: rtpCreated.Time,
		}
		fmt.Sscanf(rtpValue.String, "%f", &p.TargetRTP)
		op.RTPProfile = p
	}
	return &op, nil
}
