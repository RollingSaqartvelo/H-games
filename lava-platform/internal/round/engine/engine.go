// Package engine implements the crash game round engine.
//
// The Engine is the single source of truth for in-flight round state.
// One goroutine (the Scheduler) drives the round lifecycle; all other
// goroutines read state or submit cashout requests concurrently.
//
// Concurrency model:
//   - mu       RWMutex protects activeRound + activeBets map
//   - atomic   int64 holds current multiplier*100 (lock-free reads from HTTP handlers)
//   - Redis lock per bet prevents double-cashout under concurrent requests
package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/lava-platform/internal/domain"
	"github.com/lava-platform/internal/lock"
	"github.com/lava-platform/internal/realtime"
	"github.com/lava-platform/internal/round/fair"
	"github.com/lava-platform/internal/round/repository"
	"github.com/lava-platform/internal/round/rtp"
	"github.com/lava-platform/internal/round/state"
	"github.com/rs/zerolog/log"
	"github.com/shopspring/decimal"
)

// ─── Config ───────────────────────────────────────────────────────────────────

type Config struct {
	BettingDuration time.Duration // how long the STARTING phase lasts
	CrashCooldown   time.Duration // pause between rounds
	GrowthRate      float64       // multiplier = exp(growthRate * t)
	TickInterval    time.Duration // how often multiplier is broadcast
	DefaultRTP      int           // 92 | 94 | 96 | 98
	MaxCrashPoint   float64       // cap for extremely long rounds
}

func DefaultConfig() Config {
	return Config{
		BettingDuration: 10 * time.Second,
		CrashCooldown:   5 * time.Second,
		GrowthRate:      0.06,
		TickInterval:    100 * time.Millisecond,
		DefaultRTP:      96,
		MaxCrashPoint:   1000.0,
	}
}

// ─── In-memory bet tracking ───────────────────────────────────────────────────

type activeBet struct {
	bet         *domain.RoundBet
	mu          sync.Mutex
	cashedOut   bool
}

// ─── Engine ───────────────────────────────────────────────────────────────────

type Engine struct {
	cfg       Config
	repo      repository.Repository
	wallet    domain.WalletProvider
	pub       *realtime.Publisher
	hub       *realtime.Hub
	locker    *lock.Locker

	// Protected by mu
	mu          sync.RWMutex
	round       *domain.Round
	bets        map[string]*activeBet // betID → activeBet

	// Atomic: multiplier * 100 (e.g. 150 = 1.50x). Lock-free reads.
	multiplier  int64
}

func New(cfg Config, repo repository.Repository, wallet domain.WalletProvider,
	pub *realtime.Publisher, hub *realtime.Hub, locker *lock.Locker) *Engine {
	return &Engine{
		cfg:    cfg,
		repo:   repo,
		wallet: wallet,
		pub:    pub,
		hub:    hub,
		locker: locker,
		bets:   make(map[string]*activeBet),
	}
}

// ─── Round lifecycle (called by Scheduler) ────────────────────────────────────

// CreateRound generates a new sealed round (crash point hidden).
func (e *Engine) CreateRound(ctx context.Context) (*domain.Round, error) {
	rtpProfile, err := rtp.Get(e.cfg.DefaultRTP)
	if err != nil {
		return nil, err
	}

	serverSeed, err := fair.ServerSeed()
	if err != nil {
		return nil, err
	}

	// client_seed: in production, use last 3 BTC block hashes combined.
	clientSeed := uuid.NewString()
	nonce := time.Now().UnixNano()

	crashPoint := fair.CrashPoint(serverSeed, clientSeed, nonce, rtpProfile.HouseEdge)
	if crashPoint > e.cfg.MaxCrashPoint {
		crashPoint = e.cfg.MaxCrashPoint
	}

	round := &domain.Round{
		ID:             uuid.NewString(),
		State:          domain.RoundStateCreated,
		ServerSeed:     serverSeed,
		ServerSeedHash: fair.ServerSeedHash(serverSeed),
		ClientSeed:     clientSeed,
		Nonce:          nonce,
		RTPProfile:     rtpProfile.RTPPercent,
		HouseEdge:      rtpProfile.HouseEdge,
		CrashPoint:     crashPoint,
	}

	if _, err := e.repo.CreateRound(ctx, round); err != nil {
		return nil, fmt.Errorf("create round: %w", err)
	}

	e.mu.Lock()
	e.round = round
	e.bets = make(map[string]*activeBet)
	e.mu.Unlock()
	atomic.StoreInt64(&e.multiplier, 100) // 1.00x

	log.Info().
		Str("round_id", round.ID).
		Str("server_seed_hash", round.ServerSeedHash).
		Float64("crash_point_sealed", round.CrashPoint).
		Msg("round created")

	return round, nil
}

// OpenBetting transitions round CREATED → STARTING and broadcasts state.
func (e *Engine) OpenBetting(ctx context.Context) error {
	e.mu.Lock()
	r := e.round
	if err := state.Transition(r.State, domain.RoundStateStarting); err != nil {
		e.mu.Unlock()
		return err
	}
	r.State = domain.RoundStateStarting
	e.mu.Unlock()

	if err := e.repo.UpdateRoundState(ctx, r.ID, domain.RoundStateStarting, nil, nil); err != nil {
		return err
	}

	e.broadcastState(ctx)
	log.Info().Str("round_id", r.ID).Msg("betting open")
	return nil
}

// RunRound transitions STARTING → RUNNING and drives the multiplier loop.
// Blocks until the round crashes or ctx is cancelled.
func (e *Engine) RunRound(ctx context.Context) error {
	e.mu.Lock()
	r := e.round
	if err := state.Transition(r.State, domain.RoundStateRunning); err != nil {
		e.mu.Unlock()
		return err
	}
	r.State = domain.RoundStateRunning
	now := time.Now()
	r.StartedAt = &now

	// Activate all pending bets
	for _, ab := range e.bets {
		ab.bet.Status = domain.BetStatusActive
	}
	e.mu.Unlock()

	if err := e.repo.UpdateRoundState(ctx, r.ID, domain.RoundStateRunning, &now, nil); err != nil {
		return err
	}

	e.broadcastState(ctx)
	log.Info().Str("round_id", r.ID).Float64("crash_at", r.CrashPoint).Msg("round running")

	return e.runMultiplierLoop(ctx, r, now)
}

// runMultiplierLoop ticks every cfg.TickInterval, broadcasting the multiplier.
// Returns when the crash point is hit.
func (e *Engine) runMultiplierLoop(ctx context.Context, r *domain.Round, startedAt time.Time) error {
	ticker := time.NewTicker(e.cfg.TickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-ticker.C:
			elapsed := time.Since(startedAt).Seconds()
			mult := math.Exp(e.cfg.GrowthRate * elapsed)
			mult = math.Floor(mult*100) / 100
			atomic.StoreInt64(&e.multiplier, int64(mult*100))

			// Broadcast tick
			realtime.LogPublishErr(e.pub.Publish(ctx, realtime.MsgTypeTick, realtime.TickData{
				RoundID:    r.ID,
				Multiplier: mult,
				ElapsedMs:  time.Since(startedAt).Milliseconds(),
			}), realtime.MsgTypeTick)

			// Process auto-cashouts
			e.processAutoCashouts(ctx, r.ID, mult)

			// Crash condition
			if mult >= r.CrashPoint {
				return e.crashRound(ctx, r)
			}
		}
	}
}

func (e *Engine) crashRound(ctx context.Context, r *domain.Round) error {
	now := time.Now()

	e.mu.Lock()
	r.State = domain.RoundStateCrashed
	r.CrashedAt = &now
	// Lose all bets that didn't cash out
	for _, ab := range e.bets {
		ab.mu.Lock()
		if !ab.cashedOut {
			ab.bet.Status = domain.BetStatusLost
			go e.settleLost(context.Background(), ab.bet)
		}
		ab.mu.Unlock()
	}
	e.mu.Unlock()

	if err := e.repo.UpdateRoundState(ctx, r.ID, domain.RoundStateCrashed, r.StartedAt, &now); err != nil {
		log.Error().Err(err).Str("round_id", r.ID).Msg("update crashed state")
	}
	if err := e.repo.RevealServerSeed(ctx, r.ID, r.ServerSeed); err != nil {
		log.Error().Err(err).Str("round_id", r.ID).Msg("reveal server seed")
	}

	realtime.LogPublishErr(e.pub.Publish(ctx, realtime.MsgTypeCrashed, realtime.CrashedData{
		RoundID:    r.ID,
		CrashPoint: r.CrashPoint,
		ServerSeed: r.ServerSeed,
		ClientSeed: r.ClientSeed,
		Nonce:      r.Nonce,
	}), realtime.MsgTypeCrashed)

	log.Info().
		Str("round_id", r.ID).
		Float64("crash_point", r.CrashPoint).
		Msg("round crashed")

	return nil
}

// ─── Bet placement ────────────────────────────────────────────────────────────

type PlaceBetRequest struct {
	OperatorID   int64
	WalletUserID string
	PlayerID     string
	Currency     string
	Amount       decimal.Decimal
	AutoCashout  float64
}

type PlaceBetResponse struct {
	BetID         string `json:"bet_id"`
	RoundID       string `json:"round_id"`
	TransactionID string `json:"transaction_id"`
}

func (e *Engine) PlaceBet(ctx context.Context, req *PlaceBetRequest) (*PlaceBetResponse, error) {
	e.mu.RLock()
	r := e.round
	e.mu.RUnlock()

	if r == nil || r.State != domain.RoundStateStarting {
		return nil, errors.New("betting is closed")
	}

	betID := uuid.NewString()
	txID := fmt.Sprintf("crash:bet:%s", betID)

	// Debit wallet — idempotent via txID
	_, err := e.wallet.Debit(ctx, &domain.DebitRequest{
		UserID:        req.WalletUserID,
		Amount:        req.Amount,
		Currency:      req.Currency,
		TransactionID: txID,
		RoundID:       r.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("debit bet: %w", err)
	}

	bet := &domain.RoundBet{
		ID:            betID,
		RoundID:       r.ID,
		OperatorID:    req.OperatorID,
		WalletUserID:  req.WalletUserID,
		PlayerID:      req.PlayerID,
		BetAmount:     req.Amount,
		Currency:      req.Currency,
		AutoCashout:   req.AutoCashout,
		TransactionID: txID,
		Status:        domain.BetStatusPending,
	}
	if _, err := e.repo.CreateBet(ctx, bet); err != nil {
		return nil, fmt.Errorf("save bet: %w", err)
	}

	e.mu.Lock()
	e.bets[betID] = &activeBet{bet: bet}
	e.mu.Unlock()

	realtime.LogPublishErr(e.pub.Publish(ctx, realtime.MsgTypeBetPlaced, realtime.BetPlacedData{
		RoundID:  r.ID,
		PlayerID: req.PlayerID,
		Amount:   req.Amount.StringFixed(2),
		Currency: req.Currency,
	}), realtime.MsgTypeBetPlaced)

	log.Debug().
		Str("bet_id", betID).
		Str("player_id", req.PlayerID).
		Str("amount", req.Amount.StringFixed(2)).
		Msg("bet placed")

	return &PlaceBetResponse{BetID: betID, RoundID: r.ID, TransactionID: txID}, nil
}

// ─── Cashout ──────────────────────────────────────────────────────────────────

type CashoutRequest struct {
	BetID        string
	WalletUserID string
	PlayerID     string
	Currency     string
}

type CashoutResponse struct {
	Multiplier float64         `json:"multiplier"`
	Payout     decimal.Decimal `json:"payout"`
}

func (e *Engine) Cashout(ctx context.Context, req *CashoutRequest) (*CashoutResponse, error) {
	// Capture multiplier atomically FIRST — timing matters for fairness
	mult := float64(atomic.LoadInt64(&e.multiplier)) / 100.0

	e.mu.RLock()
	r := e.round
	ab := e.bets[req.BetID]
	e.mu.RUnlock()

	if r == nil || r.State != domain.RoundStateRunning {
		return nil, errors.New("round is not running")
	}
	if ab == nil {
		return nil, errors.New("bet not found in active round")
	}

	// Redis lock: prevents double-cashout under concurrent requests
	lk, err := e.locker.Acquire(ctx, "cashout:"+req.BetID)
	if err != nil {
		return nil, errors.New("cashout already in progress")
	}
	defer lk.Release(ctx)

	// Idempotency check under lock
	ab.mu.Lock()
	if ab.cashedOut {
		ab.mu.Unlock()
		return nil, errors.New("already cashed out")
	}
	ab.cashedOut = true
	ab.mu.Unlock()

	payout := ab.bet.BetAmount.Mul(decimal.NewFromFloat(mult))
	payout = payout.Round(2)
	payoutTxID := fmt.Sprintf("crash:payout:%s", req.BetID)

	// Credit wallet
	_, err = e.wallet.Credit(ctx, &domain.CreditRequest{
		UserID:        req.WalletUserID,
		Amount:        payout,
		Currency:      req.Currency,
		TransactionID: payoutTxID,
		RoundID:       r.ID,
	})
	if err != nil {
		// Roll back in-memory state so player can retry
		ab.mu.Lock()
		ab.cashedOut = false
		ab.mu.Unlock()
		return nil, fmt.Errorf("credit payout: %w", err)
	}

	ab.bet.Status = domain.BetStatusCashedOut
	if err := e.repo.SettleBet(ctx, req.BetID, domain.BetStatusCashedOut, &mult, &payout, payoutTxID); err != nil {
		log.Error().Err(err).Str("bet_id", req.BetID).Msg("settle cashout bet")
	}

	realtime.LogPublishErr(e.pub.Publish(ctx, realtime.MsgTypeCashout, realtime.CashoutData{
		RoundID:    r.ID,
		PlayerID:   req.PlayerID,
		Multiplier: mult,
		Payout:     payout.StringFixed(2),
		Currency:   req.Currency,
	}), realtime.MsgTypeCashout)

	log.Info().
		Str("bet_id", req.BetID).
		Float64("multiplier", mult).
		Str("payout", payout.StringFixed(2)).
		Msg("cashout")

	return &CashoutResponse{Multiplier: mult, Payout: payout}, nil
}

// ─── Auto-cashout ─────────────────────────────────────────────────────────────

func (e *Engine) processAutoCashouts(ctx context.Context, roundID string, mult float64) {
	e.mu.RLock()
	var toProcess []*activeBet
	for _, ab := range e.bets {
		if ab.bet.AutoCashout > 0 && mult >= ab.bet.AutoCashout && !ab.cashedOut {
			toProcess = append(toProcess, ab)
		}
	}
	e.mu.RUnlock()

	for _, ab := range toProcess {
		go func(ab *activeBet) {
			_, err := e.Cashout(ctx, &CashoutRequest{
				BetID:        ab.bet.ID,
				WalletUserID: ab.bet.WalletUserID,
				PlayerID:     ab.bet.PlayerID,
				Currency:     ab.bet.Currency,
			})
			if err != nil {
				log.Warn().Err(err).Str("bet_id", ab.bet.ID).Msg("auto-cashout failed")
			}
		}(ab)
	}
}

// ─── Lost bet settlement ──────────────────────────────────────────────────────

func (e *Engine) settleLost(ctx context.Context, bet *domain.RoundBet) {
	zero := decimal.Zero
	if err := e.repo.SettleBet(ctx, bet.ID, domain.BetStatusLost, nil, &zero, ""); err != nil {
		log.Error().Err(err).Str("bet_id", bet.ID).Msg("settle lost bet")
	}
}

// ─── Broadcast helpers ────────────────────────────────────────────────────────

func (e *Engine) broadcastState(ctx context.Context) {
	e.mu.RLock()
	r := e.round
	e.mu.RUnlock()
	if r == nil {
		return
	}

	data := realtime.StateData{
		ID:             r.ID,
		State:          string(r.State),
		ServerSeedHash: r.ServerSeedHash,
		ClientSeed:     r.ClientSeed,
		RTPProfile:     r.RTPProfile,
	}
	if r.StartedAt != nil {
		ts := r.StartedAt.Unix()
		data.StartedAt = &ts
	}

	msg, _ := json.Marshal(realtime.Msg{Type: realtime.MsgTypeState, Data: data})
	e.hub.SetLastState(msg)
	realtime.LogPublishErr(e.pub.Publish(ctx, realtime.MsgTypeState, data), realtime.MsgTypeState)
}

// CurrentMultiplier returns the current multiplier (safe for concurrent reads).
func (e *Engine) CurrentMultiplier() float64 {
	return float64(atomic.LoadInt64(&e.multiplier)) / 100.0
}

// CurrentRound returns the active round (may be nil between rounds).
func (e *Engine) CurrentRound() *domain.Round {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.round
}

// Cfg returns the engine configuration (used by the Scheduler).
func (e *Engine) Cfg() Config {
	return e.cfg
}
