package streetcash

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math/big"
)

// ── Symbols ───────────────────────────────────────────────────────────────────

const (
	SymDice    = 0
	SymShades  = 1
	SymSneaker = 2
	SymChain   = 3
	SymWatch   = 4
	SymKeyFob  = 5
	SymCard    = 6
	NumSyms    = 7
	SymBlank   = -1
)

// ── Payout multipliers ────────────────────────────────────────────────────────
// Calibrated for 94.3% RTP.
// 3-of-a-kind: all 3 center reels match same symbol  (× bet)
// 2-of-a-kind: exactly 2 center reels match          (× bet)

var Mult3 = [NumSyms]float64{2.0, 5.0, 10.0, 25.0, 60.0, 150.0, 500.0}
var Mult2 = [NumSyms]float64{0.6, 1.2, 2.0, 4.0, 9.0, 23.0, 62.0}

// ── Reel stop weights (out of 1000) ──────────────────────────────────────────
// [blank, sym-0..6]
// P(sym-0)=0.450, P(sym-1)=0.250, P(sym-2)=0.150,
// P(sym-3)=0.080, P(sym-4)=0.035, P(sym-5)=0.020, P(sym-6)=0.010
// RTP: 3× sum ≈ 31.1%, 2× sum ≈ 63.2% → total ≈ 94.3%
var reelWeights = [NumSyms + 1]int{
	5,   // blank
	450, // sym-0
	250, // sym-1
	150, // sym-2
	80,  // sym-3
	35,  // sym-4
	20,  // sym-5
	10,  // sym-6
} // total = 1000

// ── RNG (HMAC-SHA256) ─────────────────────────────────────────────────────────

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

// ── Helpers ───────────────────────────────────────────────────────────────────

func pickStop(r *rng) int {
	total := 0
	for _, w := range reelWeights {
		total += w
	}
	roll := r.intn(total)
	cum := 0
	for i, w := range reelWeights {
		cum += w
		if roll < cum {
			if i == 0 {
				return SymBlank
			}
			return i - 1 // sym-0..6
		}
	}
	return SymBlank
}

// ── Spin result ───────────────────────────────────────────────────────────────

type SpinResult struct {
	ServerSeedHash string    `json:"server_seed_hash"`
	Nonce          int64     `json:"nonce"`
	Reels          [3][3]int `json:"reels"`   // [reel 0-2][row 0=top,1=center,2=bot]
	Center         [3]int    `json:"center"`  // reels[0][1], reels[1][1], reels[2][1]
	WinSym         int       `json:"win_sym"` // -1 if no win
	WinType        int       `json:"win_type"` // 0=none, 2=pair, 3=triple
	Payout         float64   `json:"payout"`
	Bet            float64   `json:"bet"`
}

// ── Spin ──────────────────────────────────────────────────────────────────────

func Spin(serverSeed string, nonce int64, bet float64) *SpinResult {
	r := newRNG(serverSeed, nonce)

	// Pick 3 reels × 3 rows (top/center/bottom)
	var reels [3][3]int
	for reel := 0; reel < 3; reel++ {
		for row := 0; row < 3; row++ {
			reels[reel][row] = pickStop(r)
		}
	}

	// Center payline: middle row of each reel
	center := [3]int{reels[0][1], reels[1][1], reels[2][1]}

	// Count center symbols
	counts := [NumSyms]int{}
	for _, s := range center {
		if s >= 0 {
			counts[s]++
		}
	}

	// Find best match
	winSym := -1
	maxCount := 0
	for s, cnt := range counts {
		if cnt > maxCount {
			maxCount = cnt
			winSym = s
		}
	}
	if maxCount < 2 {
		winSym = -1
	}

	winType := 0
	if maxCount >= 2 {
		winType = maxCount
	}

	payout := 0.0
	switch winType {
	case 3:
		payout = Mult3[winSym] * bet
	case 2:
		payout = Mult2[winSym] * bet
	}

	return &SpinResult{
		ServerSeedHash: HashSeed(serverSeed),
		Nonce:          nonce,
		Reels:          reels,
		Center:         center,
		WinSym:         winSym,
		WinType:        winType,
		Payout:         payout,
		Bet:            bet,
	}
}
