// Package fair implements the provably-fair crash point algorithm.
//
// Fairness protocol:
//  1. Before STARTING: publish ServerSeedHash = SHA256(serverSeed)
//  2. ClientSeed is publicly known (e.g. last BTC block hash)
//  3. At CRASHED: reveal serverSeed — anyone can verify:
//     verify.CrashPoint(serverSeed, clientSeed, nonce, houseEdge) == round.CrashPoint
package fair

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
)

// ServerSeed generates a cryptographically random 32-byte server seed (hex).
func ServerSeed() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate server seed: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// ServerSeedHash returns SHA256(serverSeed) published before the round starts.
func ServerSeedHash(serverSeed string) string {
	h := sha256.Sum256([]byte(serverSeed))
	return hex.EncodeToString(h[:])
}

// CrashPoint generates a provably-fair crash point.
//
// Algorithm:
//  1. mac = HMAC-SHA256(key=serverSeed, data="clientSeed:nonce")
//  2. r   = first 52 bits of mac / 2^52  →  uniform [0, 1)
//  3. if r < houseEdge: crash at 1.00x (instant)
//  4. else: crash = floor((1-houseEdge) / (1-r) * 100) / 100
//
// RTP proof: P(crash > x) = (1-houseEdge)/x for x ≥ 1
// Expected return when cashing out at any x: (1-houseEdge) = RTP ✓
func CrashPoint(serverSeed, clientSeed string, nonce int64, houseEdge float64) float64 {
	msg := fmt.Sprintf("%s:%d", clientSeed, nonce)

	mac := hmac.New(sha256.New, []byte(serverSeed))
	mac.Write([]byte(msg))
	h := mac.Sum(nil)

	// 52-bit precision: highest 52 bits of first uint64
	n := binary.BigEndian.Uint64(h[:8])
	r := float64(n>>12) / float64(uint64(1)<<52) // [0, 1)

	if r < houseEdge {
		return 1.00
	}

	cp := (1.0 - houseEdge) / (1.0 - r)
	return math.Floor(cp*100) / 100
}

// Verify allows anyone to reproduce and verify a round's crash point.
func Verify(serverSeed, clientSeed string, nonce int64, houseEdge, claimedCrashPoint float64) bool {
	return CrashPoint(serverSeed, clientSeed, nonce, houseEdge) == claimedCrashPoint
}
