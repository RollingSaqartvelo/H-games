package domain

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// ─── Transaction types & statuses ────────────────────────────────────────────

type TxType string

const (
	TxTypeBet      TxType = "bet"
	TxTypeWin      TxType = "win"
	TxTypeRollback TxType = "rollback"
)

type TxStatus string

const (
	TxStatusCompleted  TxStatus = "completed"
	TxStatusRolledBack TxStatus = "rolled_back"
)

// ─── Entities ─────────────────────────────────────────────────────────────────

type Wallet struct {
	UserID    string
	Balance   decimal.Decimal
	Currency  string
	UpdatedAt time.Time
}

type Transaction struct {
	ID            int64
	UserID        string
	Type          TxType
	Amount        decimal.Decimal
	Currency      string
	TransactionID string // globally unique, supplied by game server
	RoundID       string
	Status        TxStatus
	CreatedAt     time.Time
}

// ─── Request / Response DTOs ──────────────────────────────────────────────────

type BalanceRequest struct {
	UserID   string
	Currency string
}

type BalanceResponse struct {
	UserID   string
	Balance  decimal.Decimal
	Currency string
}

type DebitRequest struct {
	UserID        string
	Amount        decimal.Decimal
	Currency      string
	TransactionID string
	RoundID       string
}

type DebitResponse struct {
	TransactionID string
	Balance       decimal.Decimal
	Currency      string
}

type CreditRequest struct {
	UserID        string
	Amount        decimal.Decimal
	Currency      string
	TransactionID string
	RoundID       string
}

type CreditResponse struct {
	TransactionID string
	Balance       decimal.Decimal
	Currency      string
}

type RollbackRequest struct {
	UserID        string
	TransactionID string // ID of this rollback record
	OriginalTxID  string // ID of the bet being reversed
	RoundID       string
	Currency      string
}

type RollbackResponse struct {
	TransactionID string
	Balance       decimal.Decimal
	Currency      string
}

// ─── Core interface ───────────────────────────────────────────────────────────

// WalletProvider is the casino wallet contract.
// InternalWalletProvider implements it with PostgreSQL.
// External operator wallets implement it via HTTP adapters.
type WalletProvider interface {
	GetBalance(ctx context.Context, req *BalanceRequest) (*BalanceResponse, error)
	Debit(ctx context.Context, req *DebitRequest) (*DebitResponse, error)
	Credit(ctx context.Context, req *CreditRequest) (*CreditResponse, error)
	Rollback(ctx context.Context, req *RollbackRequest) (*RollbackResponse, error)
}
