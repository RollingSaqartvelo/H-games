package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/lava-platform/internal/domain"
	"github.com/lava-platform/internal/lock"
	"github.com/lava-platform/internal/wallet/repository"
	"github.com/rs/zerolog/log"
)

// InternalWalletProvider implements domain.WalletProvider with:
//   - Redis distributed lock per user (prevents double-spend race)
//   - Idempotency via unique transaction_id
//   - Atomic PostgreSQL SERIALIZABLE transactions
//   - Row-level SELECT FOR UPDATE to guarantee no negative balance
type InternalWalletProvider struct {
	repo   repository.Repository
	locker *lock.Locker
}

func New(repo repository.Repository, locker *lock.Locker) *InternalWalletProvider {
	return &InternalWalletProvider{repo: repo, locker: locker}
}

// ─── WalletProvider implementation ───────────────────────────────────────────

func (s *InternalWalletProvider) GetBalance(ctx context.Context, req *domain.BalanceRequest) (*domain.BalanceResponse, error) {
	w, err := s.repo.GetOrCreateWallet(ctx, req.UserID, req.Currency)
	if err != nil {
		return nil, fmt.Errorf("get balance: %w", err)
	}
	return &domain.BalanceResponse{
		UserID:   w.UserID,
		Balance:  w.Balance,
		Currency: w.Currency,
	}, nil
}

func (s *InternalWalletProvider) Debit(ctx context.Context, req *domain.DebitRequest) (*domain.DebitResponse, error) {
	if req.Amount.IsNegative() || req.Amount.IsZero() {
		return nil, domain.ErrInvalidAmount
	}

	// ── Idempotency: return cached result if already processed ─────────────
	if existing, err := s.repo.FindTransaction(ctx, req.TransactionID); err == nil {
		w, err := s.repo.GetOrCreateWallet(ctx, req.UserID, req.Currency)
		if err != nil {
			return nil, err
		}
		log.Debug().Str("tx_id", req.TransactionID).Msg("debit idempotent hit")
		return &domain.DebitResponse{
			TransactionID: existing.TransactionID,
			Balance:       w.Balance,
			Currency:      existing.Currency,
		}, nil
	} else if !errors.Is(err, domain.ErrTransactionNotFound) {
		return nil, fmt.Errorf("idempotency check: %w", err)
	}

	// ── Ensure wallet exists before locking (idempotent upsert) ───────────
	if _, err := s.repo.GetOrCreateWallet(ctx, req.UserID, req.Currency); err != nil {
		return nil, fmt.Errorf("ensure wallet: %w", err)
	}

	// ── Redis distributed lock (per user) ─────────────────────────────────
	lk, err := s.locker.Acquire(ctx, req.UserID)
	if err != nil {
		return nil, domain.ErrLockNotAcquired
	}
	defer lk.Release(ctx)

	// ── PostgreSQL SERIALIZABLE transaction ───────────────────────────────
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer rollback(ctx, tx)

	// SELECT FOR UPDATE — prevents concurrent balance mutation
	wallet, err := s.repo.LockWallet(ctx, tx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("lock wallet: %w", err)
	}

	if wallet.Currency != req.Currency {
		return nil, domain.ErrCurrencyMismatch
	}
	if wallet.Balance.LessThan(req.Amount) {
		return nil, domain.ErrInsufficientFunds
	}

	t := &domain.Transaction{
		UserID:        req.UserID,
		Type:          domain.TxTypeBet,
		Amount:        req.Amount,
		Currency:      req.Currency,
		TransactionID: req.TransactionID,
		RoundID:       req.RoundID,
		Status:        domain.TxStatusCompleted,
	}
	if _, err := s.repo.CreateTransaction(ctx, tx, t); err != nil {
		return nil, err
	}

	updated, err := s.repo.UpdateBalance(ctx, tx, req.UserID, req.Amount.Neg())
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	log.Info().
		Str("user_id", req.UserID).
		Str("tx_id", req.TransactionID).
		Str("amount", req.Amount.String()).
		Str("balance_after", updated.Balance.String()).
		Msg("debit")

	return &domain.DebitResponse{
		TransactionID: req.TransactionID,
		Balance:       updated.Balance,
		Currency:      updated.Currency,
	}, nil
}

func (s *InternalWalletProvider) Credit(ctx context.Context, req *domain.CreditRequest) (*domain.CreditResponse, error) {
	if req.Amount.IsNegative() || req.Amount.IsZero() {
		return nil, domain.ErrInvalidAmount
	}

	if existing, err := s.repo.FindTransaction(ctx, req.TransactionID); err == nil {
		w, err := s.repo.GetOrCreateWallet(ctx, req.UserID, req.Currency)
		if err != nil {
			return nil, err
		}
		log.Debug().Str("tx_id", req.TransactionID).Msg("credit idempotent hit")
		return &domain.CreditResponse{
			TransactionID: existing.TransactionID,
			Balance:       w.Balance,
			Currency:      existing.Currency,
		}, nil
	} else if !errors.Is(err, domain.ErrTransactionNotFound) {
		return nil, fmt.Errorf("idempotency check: %w", err)
	}

	if _, err := s.repo.GetOrCreateWallet(ctx, req.UserID, req.Currency); err != nil {
		return nil, fmt.Errorf("ensure wallet: %w", err)
	}

	lk, err := s.locker.Acquire(ctx, req.UserID)
	if err != nil {
		return nil, domain.ErrLockNotAcquired
	}
	defer lk.Release(ctx)

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer rollback(ctx, tx)

	wallet, err := s.repo.LockWallet(ctx, tx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("lock wallet: %w", err)
	}
	if wallet.Currency != req.Currency {
		return nil, domain.ErrCurrencyMismatch
	}

	t := &domain.Transaction{
		UserID:        req.UserID,
		Type:          domain.TxTypeWin,
		Amount:        req.Amount,
		Currency:      req.Currency,
		TransactionID: req.TransactionID,
		RoundID:       req.RoundID,
		Status:        domain.TxStatusCompleted,
	}
	if _, err := s.repo.CreateTransaction(ctx, tx, t); err != nil {
		return nil, err
	}

	updated, err := s.repo.UpdateBalance(ctx, tx, req.UserID, req.Amount)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	log.Info().
		Str("user_id", req.UserID).
		Str("tx_id", req.TransactionID).
		Str("amount", req.Amount.String()).
		Str("balance_after", updated.Balance.String()).
		Msg("credit")

	return &domain.CreditResponse{
		TransactionID: req.TransactionID,
		Balance:       updated.Balance,
		Currency:      updated.Currency,
	}, nil
}

func (s *InternalWalletProvider) Rollback(ctx context.Context, req *domain.RollbackRequest) (*domain.RollbackResponse, error) {
	// Fetch the original bet — must exist and be a bet
	originalTx, err := s.repo.FindTransaction(ctx, req.OriginalTxID)
	if err != nil {
		return nil, err // ErrTransactionNotFound propagates as-is
	}
	if originalTx.Type != domain.TxTypeBet {
		return nil, domain.ErrOriginalTxNotBet
	}
	if originalTx.Status == domain.TxStatusRolledBack {
		return nil, domain.ErrTransactionAlreadyRolledBack
	}

	// Idempotency for the rollback transaction itself
	if existing, err := s.repo.FindTransaction(ctx, req.TransactionID); err == nil {
		w, err := s.repo.GetOrCreateWallet(ctx, req.UserID, req.Currency)
		if err != nil {
			return nil, err
		}
		log.Debug().Str("tx_id", req.TransactionID).Msg("rollback idempotent hit")
		return &domain.RollbackResponse{
			TransactionID: existing.TransactionID,
			Balance:       w.Balance,
			Currency:      existing.Currency,
		}, nil
	} else if !errors.Is(err, domain.ErrTransactionNotFound) {
		return nil, fmt.Errorf("idempotency check: %w", err)
	}

	lk, err := s.locker.Acquire(ctx, req.UserID)
	if err != nil {
		return nil, domain.ErrLockNotAcquired
	}
	defer lk.Release(ctx)

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer rollback(ctx, tx)

	wallet, err := s.repo.LockWallet(ctx, tx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("lock wallet: %w", err)
	}
	if wallet.Currency != req.Currency {
		return nil, domain.ErrCurrencyMismatch
	}

	// Record the rollback in the ledger
	rbTx := &domain.Transaction{
		UserID:        req.UserID,
		Type:          domain.TxTypeRollback,
		Amount:        originalTx.Amount,
		Currency:      req.Currency,
		TransactionID: req.TransactionID,
		RoundID:       req.RoundID,
		Status:        domain.TxStatusCompleted,
	}
	if _, err := s.repo.CreateTransaction(ctx, tx, rbTx); err != nil {
		return nil, err
	}

	// Credit the original bet amount back
	updated, err := s.repo.UpdateBalance(ctx, tx, req.UserID, originalTx.Amount)
	if err != nil {
		return nil, err
	}

	// Mark the original bet as rolled back
	if err := s.repo.UpdateTransactionStatus(ctx, tx, originalTx.ID, domain.TxStatusRolledBack); err != nil {
		return nil, fmt.Errorf("mark rolled back: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	log.Info().
		Str("user_id", req.UserID).
		Str("tx_id", req.TransactionID).
		Str("original_tx_id", req.OriginalTxID).
		Str("amount", originalTx.Amount.String()).
		Str("balance_after", updated.Balance.String()).
		Msg("rollback")

	return &domain.RollbackResponse{
		TransactionID: req.TransactionID,
		Balance:       updated.Balance,
		Currency:      updated.Currency,
	}, nil
}

// ─── Helper ───────────────────────────────────────────────────────────────────

func rollback(ctx context.Context, tx pgx.Tx) {
	if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		log.Error().Err(err).Msg("tx rollback failed")
	}
}
