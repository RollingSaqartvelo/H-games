package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lava-platform/internal/domain"
	"github.com/shopspring/decimal"
)

type Repository interface {
	CreateRound(ctx context.Context, r *domain.Round) (*domain.Round, error)
	UpdateRoundState(ctx context.Context, id string, state domain.RoundState, startedAt, crashedAt *time.Time) error
	RevealServerSeed(ctx context.Context, id, serverSeed string) error
	GetRoundByID(ctx context.Context, id string) (*domain.Round, error)
	GetLatestRound(ctx context.Context) (*domain.Round, error)
	GetRoundHistory(ctx context.Context, limit, offset int) ([]*domain.Round, error)

	CreateBet(ctx context.Context, bet *domain.RoundBet) (*domain.RoundBet, error)
	GetBetByID(ctx context.Context, id string) (*domain.RoundBet, error)
	GetActiveBetsByRound(ctx context.Context, roundID string) ([]*domain.RoundBet, error)
	SettleBet(ctx context.Context, betID string, status domain.BetStatus, cashoutAt *float64, payout *decimal.Decimal, payoutTxID string) error
	GetPlayerBets(ctx context.Context, walletUserID string, limit, offset int) ([]*domain.RoundBet, error)
}

type postgresRepo struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) Repository {
	return &postgresRepo{pool: pool}
}

// ─── Round ────────────────────────────────────────────────────────────────────

func (r *postgresRepo) CreateRound(ctx context.Context, round *domain.Round) (*domain.Round, error) {
	const q = `
		INSERT INTO rounds
			(id, state, server_seed_hash, client_seed, nonce, rtp_profile, house_edge, crash_point)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING created_at`

	err := r.pool.QueryRow(ctx, q,
		round.ID, round.State, round.ServerSeedHash,
		round.ClientSeed, round.Nonce, round.RTPProfile,
		round.HouseEdge, round.CrashPoint,
	).Scan(&round.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create round: %w", err)
	}
	return round, nil
}

func (r *postgresRepo) UpdateRoundState(ctx context.Context, id string, state domain.RoundState, startedAt, crashedAt *time.Time) error {
	const q = `
		UPDATE rounds SET state = $2, started_at = $3, crashed_at = $4, updated_at = NOW()
		WHERE id = $1`
	_, err := r.pool.Exec(ctx, q, id, state, startedAt, crashedAt)
	return err
}

func (r *postgresRepo) RevealServerSeed(ctx context.Context, id, serverSeed string) error {
	const q = `UPDATE rounds SET server_seed = $2, updated_at = NOW() WHERE id = $1`
	_, err := r.pool.Exec(ctx, q, id, serverSeed)
	return err
}

func (r *postgresRepo) GetRoundByID(ctx context.Context, id string) (*domain.Round, error) {
	const q = `
		SELECT id, state, COALESCE(server_seed,''), server_seed_hash, client_seed,
		       nonce, rtp_profile, house_edge::text, crash_point::text,
		       started_at, crashed_at, created_at
		FROM rounds WHERE id = $1`

	round, err := scanRound(r.pool.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("round not found: %s", id)
	}
	return round, err
}

func (r *postgresRepo) GetLatestRound(ctx context.Context) (*domain.Round, error) {
	const q = `
		SELECT id, state, COALESCE(server_seed,''), server_seed_hash, client_seed,
		       nonce, rtp_profile, house_edge::text, crash_point::text,
		       started_at, crashed_at, created_at
		FROM rounds ORDER BY created_at DESC LIMIT 1`

	round, err := scanRound(r.pool.QueryRow(ctx, q))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return round, err
}

func (r *postgresRepo) GetRoundHistory(ctx context.Context, limit, offset int) ([]*domain.Round, error) {
	const q = `
		SELECT id, state, COALESCE(server_seed,''), server_seed_hash, client_seed,
		       nonce, rtp_profile, house_edge::text, crash_point::text,
		       started_at, crashed_at, created_at
		FROM rounds WHERE state IN ('CRASHED','FINISHED')
		ORDER BY created_at DESC LIMIT $1 OFFSET $2`

	rows, err := r.pool.Query(ctx, q, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rounds []*domain.Round
	for rows.Next() {
		round, err := scanRound(rows)
		if err != nil {
			return nil, err
		}
		rounds = append(rounds, round)
	}
	return rounds, rows.Err()
}

// ─── Bets ─────────────────────────────────────────────────────────────────────

func (r *postgresRepo) CreateBet(ctx context.Context, bet *domain.RoundBet) (*domain.RoundBet, error) {
	const q = `
		INSERT INTO round_bets
			(id, round_id, operator_id, wallet_user_id, player_id,
			 bet_amount, currency, auto_cashout, transaction_id, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING created_at`

	err := r.pool.QueryRow(ctx, q,
		bet.ID, bet.RoundID, bet.OperatorID, bet.WalletUserID, bet.PlayerID,
		bet.BetAmount.String(), bet.Currency, bet.AutoCashout,
		bet.TransactionID, bet.Status,
	).Scan(&bet.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create bet: %w", err)
	}
	return bet, nil
}

func (r *postgresRepo) GetBetByID(ctx context.Context, id string) (*domain.RoundBet, error) {
	const q = `
		SELECT id, round_id, operator_id, wallet_user_id, player_id,
		       bet_amount::text, currency, auto_cashout::text,
		       cashout_at, payout_amount::text, transaction_id, COALESCE(payout_tx_id,''), status, created_at
		FROM round_bets WHERE id = $1`

	bet, err := scanBet(r.pool.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("bet not found: %s", id)
	}
	return bet, err
}

func (r *postgresRepo) GetActiveBetsByRound(ctx context.Context, roundID string) ([]*domain.RoundBet, error) {
	const q = `
		SELECT id, round_id, operator_id, wallet_user_id, player_id,
		       bet_amount::text, currency, auto_cashout::text,
		       cashout_at, payout_amount::text, transaction_id, COALESCE(payout_tx_id,''), status, created_at
		FROM round_bets WHERE round_id = $1 AND status IN ('PENDING','ACTIVE')`

	rows, err := r.pool.Query(ctx, q, roundID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bets []*domain.RoundBet
	for rows.Next() {
		bet, err := scanBet(rows)
		if err != nil {
			return nil, err
		}
		bets = append(bets, bet)
	}
	return bets, rows.Err()
}

func (r *postgresRepo) SettleBet(ctx context.Context, betID string, status domain.BetStatus, cashoutAt *float64, payout *decimal.Decimal, payoutTxID string) error {
	var payoutStr *string
	if payout != nil {
		s := payout.String()
		payoutStr = &s
	}
	const q = `
		UPDATE round_bets
		SET status = $2, cashout_at = $3, payout_amount = $4::numeric, payout_tx_id = $5
		WHERE id = $1`
	_, err := r.pool.Exec(ctx, q, betID, status, cashoutAt, payoutStr, payoutTxID)
	return err
}

func (r *postgresRepo) GetPlayerBets(ctx context.Context, walletUserID string, limit, offset int) ([]*domain.RoundBet, error) {
	const q = `
		SELECT id, round_id, operator_id, wallet_user_id, player_id,
		       bet_amount::text, currency, auto_cashout::text,
		       cashout_at, payout_amount::text, transaction_id, COALESCE(payout_tx_id,''), status, created_at
		FROM round_bets WHERE wallet_user_id = $1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, q, walletUserID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bets []*domain.RoundBet
	for rows.Next() {
		bet, err := scanBet(rows)
		if err != nil {
			return nil, err
		}
		bets = append(bets, bet)
	}
	return bets, rows.Err()
}

// ─── Scan helpers ─────────────────────────────────────────────────────────────

type rowScanner interface{ Scan(dest ...any) error }

func scanRound(row rowScanner) (*domain.Round, error) {
	var r domain.Round
	var he, cp string
	err := row.Scan(
		&r.ID, &r.State, &r.ServerSeed, &r.ServerSeedHash, &r.ClientSeed,
		&r.Nonce, &r.RTPProfile, &he, &cp,
		&r.StartedAt, &r.CrashedAt, &r.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	fmt.Sscanf(he, "%f", &r.HouseEdge)
	fmt.Sscanf(cp, "%f", &r.CrashPoint)
	return &r, nil
}

func scanBet(row rowScanner) (*domain.RoundBet, error) {
	var b domain.RoundBet
	var amount, autoCashout, payoutAmount string
	var cashoutAt *float64
	err := row.Scan(
		&b.ID, &b.RoundID, &b.OperatorID, &b.WalletUserID, &b.PlayerID,
		&amount, &b.Currency, &autoCashout,
		&cashoutAt, &payoutAmount, &b.TransactionID, &b.PayoutTxID, &b.Status, &b.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	b.BetAmount, _ = decimal.NewFromString(amount)
	fmt.Sscanf(autoCashout, "%f", &b.AutoCashout)
	b.CashoutAt = cashoutAt
	if payoutAmount != "" && payoutAmount != "<nil>" {
		p, err := decimal.NewFromString(payoutAmount)
		if err == nil {
			b.PayoutAmount = &p
		}
	}
	return &b, nil
}
