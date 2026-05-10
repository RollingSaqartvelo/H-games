package domain

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// ─── Round lifecycle ──────────────────────────────────────────────────────────

type RoundState string

const (
	RoundStateCreated  RoundState = "CREATED"   // generated, crash point sealed
	RoundStateStarting RoundState = "STARTING"  // betting window open
	RoundStateRunning  RoundState = "RUNNING"   // multiplier climbing
	RoundStateCrashed  RoundState = "CRASHED"   // crash point hit, server_seed revealed
	RoundStateFinished RoundState = "FINISHED"  // all players cashed out before crash
)

func (s RoundState) IsTerminal() bool {
	return s == RoundStateCrashed || s == RoundStateFinished
}

func (s RoundState) CanTransitionTo(next RoundState) bool {
	allowed := map[RoundState]RoundState{
		RoundStateCreated:  RoundStateStarting,
		RoundStateStarting: RoundStateRunning,
		RoundStateRunning:  RoundStateCrashed,
	}
	return allowed[s] == next || (s == RoundStateRunning && next == RoundStateFinished)
}

// ─── Round entity ─────────────────────────────────────────────────────────────

type Round struct {
	ID             string
	GameType       string     // "outlaw_escape" | "granny_jet"
	State          RoundState
	ServerSeed     string     // concealed during round; revealed at crash
	ServerSeedHash string     // SHA256(serverSeed) — published before STARTING
	ClientSeed     string     // publicly known before round begins
	Nonce          int64
	RTPProfile     int        // 92 | 94 | 96 | 98 (percentage)
	HouseEdge      float64    // 1 - RTPProfile/100
	CrashPoint     float64    // pre-calculated, hidden until CRASHED
	StartedAt      *time.Time
	CrashedAt      *time.Time
	CreatedAt      time.Time
}

// ─── Round bet ────────────────────────────────────────────────────────────────

type BetStatus string

const (
	BetStatusPending   BetStatus = "PENDING"    // placed during STARTING phase
	BetStatusActive    BetStatus = "ACTIVE"     // round is RUNNING
	BetStatusCashedOut BetStatus = "CASHED_OUT"
	BetStatusLost      BetStatus = "LOST"       // did not cash out before crash
)

type RoundBet struct {
	ID            string
	RoundID       string
	OperatorID    int64
	WalletUserID  string          // "op{id}:{player_id}"
	PlayerID      string
	BetAmount     decimal.Decimal
	Currency      string
	AutoCashout   float64         // 0 = manual only
	CashoutAt     *float64        // multiplier captured at cashout
	PayoutAmount  *decimal.Decimal
	TransactionID string          // wallet debit tx
	PayoutTxID    string          // wallet credit tx
	Status        BetStatus
	CreatedAt     time.Time
}

// BetTxID returns a deterministic wallet debit transaction ID.
func (b *RoundBet) BetTxID() string {
	return fmt.Sprintf("crash:bet:%s", b.ID)
}

// PayoutTxIDFor returns a deterministic wallet credit transaction ID.
func (b *RoundBet) PayoutTxIDFor() string {
	return fmt.Sprintf("crash:payout:%s", b.ID)
}
