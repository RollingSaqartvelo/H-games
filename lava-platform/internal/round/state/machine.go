// Package state provides a validated state machine for round lifecycle.
package state

import (
	"fmt"

	"github.com/lava-platform/internal/domain"
)

// Transition validates and applies a state transition.
// Returns an error if the transition is not allowed.
func Transition(current, next domain.RoundState) error {
	if !current.CanTransitionTo(next) {
		return fmt.Errorf("invalid transition: %s → %s", current, next)
	}
	return nil
}

// MustTransition panics on invalid transition — use in tests only.
func MustTransition(current, next domain.RoundState) {
	if err := Transition(current, next); err != nil {
		panic(err)
	}
}
