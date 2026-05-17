package streetcash

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math/big"
)

// ── Constants ─────────────────────────────────────────────────────────────────

const NumFrameCells = 26

// Symbol indices
const (
	SymDice    = 0 // ×0.3 per match — most common
	SymShades  = 1 // ×0.6
	SymSneaker = 2 // ×1.5
	SymChain   = 3 // ×4.0
	SymWatch   = 4 // ×10.0
	SymKeyFob  = 5 // ×25.0
	SymCard    = 6 // ×80.0 — jackpot
	NumSyms    = 7
	SymBlank   = -1 // decorative, never wins
)

// ── Payout multipliers (per match × bet) ─────────────────────────────────────
// Calibrated for 94% RTP, 62.6% no-win rate.
var Mults = [NumSyms]float64{0.30, 0.60, 1.50, 4.00, 10.00, 25.00, 80.00}

// ── Reel pool weights (out of 10000) ─────────────────────────────────────────
// miss=200, then sym-0..6
var reelWeights = [NumSyms + 1]int{
	200,  // miss (index 0 in this table, symIdx=-1)
	2800, // sym-0
	2200, // sym-1
	1800, // sym-2
	1200, // sym-3
	800,  // sym-4
	600,  // sym-5
	400,  // sym-6
} // total = 10000

// ── Frame cell weights per cell (out of 10000) ───────────────────────────────
// Each of 26 cells is independently assigned from this distribution.
// blank=9034, sym-0..6 as below.
var cellWeights = [NumSyms + 1]int{
	9034, // blank
	310,  // sym-0
	239,  // sym-1
	167,  // sym-2
	102,  // sym-3
	75,   // sym-4
	49,   // sym-5
	24,   // sym-6
} // total = 10000

// ── RNG (HMAC-SHA256, same as H-SLOTS) ───────────────────────────────────────

type rng struct {
	seed  []byte
	nonce int64
	pos   int
	buf   []byte
}

func newRNG(serverSeed string, nonce int64) *rng {
	mac := hmac.New(sha256.New, []byte(serverSeed))
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(nonce))
	mac.Write(b)
	return &rng{seed: []byte(serverSeed), nonce: nonce, buf: mac.Sum(nil)}
}

func (r *rng) nextBytes(n int) []byte {
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		if r.pos >= len(r.buf) {
			mac := hmac.New(sha256.New, r.seed)
			b := make([]byte, 16)
			binary.BigEndian.PutUint64(b[:8], uint64(r.nonce))
			binary.BigEndian.PutUint64(b[8:], uint64(r.pos/32))
			mac.Write(b)
			r.buf = mac.Sum(nil)
			r.pos = 0
		}
		out[i] = r.buf[r.pos]
		r.pos++
	}
	return out
}

func (r *rng) intn(n int) int {
	if n <= 0 {
		return 0
	}
	b := r.nextBytes(4)
	v := new(big.Int).SetBytes(b)
	return int(new(big.Int).Mod(v, big.NewInt(int64(n))).Int64())
}

func HashSeed(seed string) string {
	h := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(h[:])
}

// ── Spin result ───────────────────────────────────────────────────────────────

type SpinResult struct {
	ServerSeedHash string  `json:"server_seed_hash"`
	Nonce          int64   `json:"nonce"`
	Frame          []int   `json:"frame"`        // 26 symIdx or -1 (blank)
	Reel           int     `json:"reel"`          // symIdx or -1 (miss)
	Matches        int     `json:"matches"`
	Payout         float64 `json:"payout"`
	Bet            float64 `json:"bet"`
}

// ── Spin ──────────────────────────────────────────────────────────────────────

func Spin(serverSeed string, nonce int64, bet float64) *SpinResult {
	r := newRNG(serverSeed, nonce)

	// 1. Pick reel outcome
	reelTotal := 0
	for _, w := range reelWeights {
		reelTotal += w
	}
	reelRoll := r.intn(reelTotal)
	cum := 0
	reelSymIdx := -1 // miss
	for i, w := range reelWeights {
		cum += w
		if reelRoll < cum {
			if i > 0 {
				reelSymIdx = i - 1 // sym-0..6
			}
			break
		}
	}

	// 2. Fill 26 frame cells independently
	cellTotal := 0
	for _, w := range cellWeights {
		cellTotal += w
	}
	frame := make([]int, NumFrameCells)
	for i := 0; i < NumFrameCells; i++ {
		roll := r.intn(cellTotal)
		c := 0
		frame[i] = SymBlank
		for j, w := range cellWeights {
			c += w
			if roll < c {
				if j > 0 {
					frame[i] = j - 1 // sym-0..6
				}
				break
			}
		}
	}

	// 3. Count matches and calculate payout
	matches := 0
	if reelSymIdx >= 0 {
		for _, s := range frame {
			if s == reelSymIdx {
				matches++
			}
		}
	}

	payout := 0.0
	if matches > 0 {
		payout = float64(matches) * Mults[reelSymIdx] * bet
	}

	return &SpinResult{
		ServerSeedHash: HashSeed(serverSeed),
		Nonce:          nonce,
		Frame:          frame,
		Reel:           reelSymIdx,
		Matches:        matches,
		Payout:         payout,
		Bet:            bet,
	}
}
