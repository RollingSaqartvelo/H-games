// Package rtp defines Return-To-Player profiles for the crash game engine.
// Each profile controls the house edge and therefore the crash point distribution.
package rtp

import "fmt"

// Profile holds RTP configuration for a crash game round.
type Profile struct {
	RTPPercent int     // 92 | 94 | 96 | 98
	RTP        float64 // 0.92 … 0.98
	HouseEdge  float64 // 1 - RTP
}

var profiles = map[int]Profile{
	98: {RTPPercent: 98, RTP: 0.98, HouseEdge: 0.02},
	96: {RTPPercent: 96, RTP: 0.96, HouseEdge: 0.04},
	94: {RTPPercent: 94, RTP: 0.94, HouseEdge: 0.06},
	92: {RTPPercent: 92, RTP: 0.92, HouseEdge: 0.08},
}

// Default is the standard RTP profile used when none is specified.
const Default = 96

// Get returns the RTP profile for the given percentage.
// Valid values: 92, 94, 96, 98.
func Get(pct int) (Profile, error) {
	p, ok := profiles[pct]
	if !ok {
		return Profile{}, fmt.Errorf("unsupported RTP profile: %d (valid: 92, 94, 96, 98)", pct)
	}
	return p, nil
}

// MustGet panics if the profile is invalid — use only with constants.
func MustGet(pct int) Profile {
	p, err := Get(pct)
	if err != nil {
		panic(err)
	}
	return p
}

// All returns all available RTP profiles.
func All() []Profile {
	return []Profile{profiles[92], profiles[94], profiles[96], profiles[98]}
}
