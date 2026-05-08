package domain

import (
	"database/sql"
	"fmt"
	"time"
)

// ─── Operator ─────────────────────────────────────────────────────────────────

type OperatorStatus string

const (
	OperatorStatusActive    OperatorStatus = "active"
	OperatorStatusInactive  OperatorStatus = "inactive"
	OperatorStatusSuspended OperatorStatus = "suspended"
)

type Operator struct {
	ID                  int64
	Name                string
	APIKey              string
	SecretKey           string         // never exposed in API responses — store encrypted at rest
	Status              OperatorStatus
	AllowedOrigins      []string
	CallbackURL         string
	DefaultRTPProfileID sql.NullInt64
	RTPProfile          *RTPProfile    // populated when joined
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// ─── RTP Profile ──────────────────────────────────────────────────────────────

type RTPProfile struct {
	ID        int64
	Name      string
	TargetRTP float64 // e.g. 96.00
	CreatedAt time.Time
}

// ─── Operator Session ─────────────────────────────────────────────────────────

type OperatorSession struct {
	ID           int64
	OperatorID   int64
	PlayerID     string
	SessionToken string
	Currency     string
	IP           string
	Active       bool
	ExpiresAt    time.Time
	CreatedAt    time.Time
}

// WalletUserID returns the composite key used for wallet data isolation.
// Prefixing with operator ID ensures a player's balance is siloed per operator.
func (s *OperatorSession) WalletUserID() string {
	return fmt.Sprintf("op%d:%s", s.OperatorID, s.PlayerID)
}

// ─── Request / Response DTOs ──────────────────────────────────────────────────

type CreateOperatorRequest struct {
	Name           string
	CallbackURL    string
	AllowedOrigins []string
	RTPProfileID   *int64
}

type CreateSessionRequest struct {
	OperatorID int64
	PlayerID   string
	Currency   string
	IP         string
	TTL        time.Duration
}
