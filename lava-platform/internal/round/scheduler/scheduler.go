package scheduler

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/lava-platform/internal/round/engine"
)

// Scheduler drives the round lifecycle loop:
// CreateRound → OpenBetting (wait BettingDuration) → RunRound (blocks until crash) → CrashCooldown → repeat
type Scheduler struct {
	eng *engine.Engine
	cfg engine.Config
}

func New(eng *engine.Engine, cfg engine.Config) *Scheduler {
	return &Scheduler{eng: eng, cfg: cfg}
}

// Run starts the perpetual round loop until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) {
	log.Info().Msg("round scheduler started")
	for {
		if err := s.runOne(ctx); err != nil {
			if ctx.Err() != nil {
				log.Info().Msg("round scheduler stopped")
				return
			}
			log.Error().Err(err).Msg("round cycle error — restarting after 2s")
			select {
			case <-time.After(2 * time.Second):
			case <-ctx.Done():
				return
			}
			continue
		}
		// Cooldown between rounds.
		select {
		case <-time.After(s.cfg.CrashCooldown):
		case <-ctx.Done():
			log.Info().Msg("round scheduler stopped")
			return
		}
	}
}

func (s *Scheduler) runOne(ctx context.Context) error {
	// 1. Create round (CREATED state, seeds pre-generated).
	if _, err := s.eng.CreateRound(ctx); err != nil {
		return err
	}

	// 2. Open betting window (STARTING state).
	if err := s.eng.OpenBetting(ctx); err != nil {
		return err
	}

	// Wait for betting window to close.
	select {
	case <-time.After(s.cfg.BettingDuration):
	case <-ctx.Done():
		return ctx.Err()
	}

	// 3. Launch round (RUNNING state); blocks until crash.
	return s.eng.RunRound(ctx)
}
