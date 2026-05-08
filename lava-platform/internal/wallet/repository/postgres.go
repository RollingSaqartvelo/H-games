package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lava-platform/internal/domain"
	"github.com/shopspring/decimal"
)

// Repository is the data-access contract for wallet operations.
// Implemented by postgresRepo; consumed by the service layer.
type Repository interface {
	// GetOrCreateWallet upserts a wallet row (idempotent).
	GetOrCreateWallet(ctx context.Context, userID, currency string) (*domain.Wallet, error)
	// LockWallet acquires a row-level lock inside an open transaction.
	LockWallet(ctx context.Context, tx pgx.Tx, userID string) (*domain.Wallet, error)
	// UpdateBalance applies a signed delta (negative for debit, positive for credit).
	UpdateBalance(ctx context.Context, tx pgx.Tx, userID string, delta decimal.Decimal) (*domain.Wallet, error)
	// FindTransaction looks up a transaction by its external transaction_id.
	FindTransaction(ctx context.Context, transactionID string) (*domain.Transaction, error)
	// CreateTransaction inserts a new ledger row within an open transaction.
	CreateTransaction(ctx context.Context, tx pgx.Tx, t *domain.Transaction) (*domain.Transaction, error)
	// UpdateTransactionStatus changes the status of a ledger row (e.g. → rolled_back).
	UpdateTransactionStatus(ctx context.Context, tx pgx.Tx, id int64, status domain.TxStatus) error
	// BeginTx starts a serializable PostgreSQL transaction.
	BeginTx(ctx context.Context) (pgx.Tx, error)
}

type postgresRepo struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) Repository {
	return &postgresRepo{pool: pool}
}

func (r *postgresRepo) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.Serializable,
	})
}

func (r *postgresRepo) GetOrCreateWallet(ctx context.Context, userID, currency string) (*domain.Wallet, error) {
	const q = `
		INSERT INTO wallets (user_id, balance, currency, updated_at)
		VALUES ($1, 0, $2, NOW())
		ON CONFLICT (user_id) DO UPDATE SET updated_at = wallets.updated_at
		RETURNING user_id, balance::text, currency, updated_at`

	return scanWallet(r.pool.QueryRow(ctx, q, userID, currency))
}

func (r *postgresRepo) LockWallet(ctx context.Context, tx pgx.Tx, userID string) (*domain.Wallet, error) {
	const q = `
		SELECT user_id, balance::text, currency, updated_at
		FROM wallets
		WHERE user_id = $1
		FOR UPDATE`

	w, err := scanWallet(tx.QueryRow(ctx, q, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrWalletNotFound
	}
	return w, err
}

func (r *postgresRepo) UpdateBalance(ctx context.Context, tx pgx.Tx, userID string, delta decimal.Decimal) (*domain.Wallet, error) {
	const q = `
		UPDATE wallets
		SET balance = balance + $1::numeric, updated_at = NOW()
		WHERE user_id = $2
		RETURNING user_id, balance::text, currency, updated_at`

	w, err := scanWallet(tx.QueryRow(ctx, q, delta.String(), userID))
	if err != nil {
		return nil, err
	}
	// DB CHECK constraint (balance >= 0) fires first, but guard here too.
	if w.Balance.IsNegative() {
		return nil, domain.ErrInsufficientFunds
	}
	return w, nil
}

func (r *postgresRepo) FindTransaction(ctx context.Context, transactionID string) (*domain.Transaction, error) {
	const q = `
		SELECT id, user_id, type, amount::text, currency, transaction_id, round_id, status, created_at
		FROM transactions
		WHERE transaction_id = $1`

	t, err := scanTransaction(r.pool.QueryRow(ctx, q, transactionID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrTransactionNotFound
	}
	return t, err
}

func (r *postgresRepo) CreateTransaction(ctx context.Context, tx pgx.Tx, t *domain.Transaction) (*domain.Transaction, error) {
	const q = `
		INSERT INTO transactions
			(user_id, type, amount, currency, transaction_id, round_id, status)
		VALUES ($1, $2, $3::numeric, $4, $5, $6, $7)
		RETURNING id, created_at`

	err := tx.QueryRow(ctx, q,
		t.UserID, t.Type, t.Amount.String(), t.Currency,
		t.TransactionID, t.RoundID, t.Status,
	).Scan(&t.ID, &t.CreatedAt)

	if err != nil {
		if isUniqueViolation(err) {
			return nil, domain.ErrDuplicateTransaction
		}
		return nil, fmt.Errorf("create transaction: %w", err)
	}
	return t, nil
}

func (r *postgresRepo) UpdateTransactionStatus(ctx context.Context, tx pgx.Tx, id int64, status domain.TxStatus) error {
	const q = `UPDATE transactions SET status = $1 WHERE id = $2`
	_, err := tx.Exec(ctx, q, status, id)
	return err
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

type scanner interface{ Scan(dest ...any) error }

func scanWallet(row scanner) (*domain.Wallet, error) {
	var w domain.Wallet
	var balance string
	if err := row.Scan(&w.UserID, &balance, &w.Currency, &w.UpdatedAt); err != nil {
		return nil, err
	}
	var err error
	if w.Balance, err = decimal.NewFromString(balance); err != nil {
		return nil, fmt.Errorf("parse balance: %w", err)
	}
	return &w, nil
}

func scanTransaction(row scanner) (*domain.Transaction, error) {
	var t domain.Transaction
	var amount string
	err := row.Scan(
		&t.ID, &t.UserID, &t.Type, &amount, &t.Currency,
		&t.TransactionID, &t.RoundID, &t.Status, &t.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if t.Amount, err = decimal.NewFromString(amount); err != nil {
		return nil, fmt.Errorf("parse amount: %w", err)
	}
	return &t, nil
}

// isUniqueViolation checks for PostgreSQL SQLSTATE 23505.
// pgx wraps PgError in its message, so string check is the safest cross-version approach.
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "23505")
}
