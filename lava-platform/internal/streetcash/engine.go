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
	SymDice    = 0 // loss symbol
	SymShades  = 1
	SymSneaker = 2
	SymChain   = 3
	SymWatch   = 4
	SymKeyFob  = 5
	SymCard    = 6
	NumSyms    = 7
)

// ── Payout multipliers (× bet) ────────────────────────────────────────────────
// Calibrated for 94% RTP.
// Dice = ×0 (loss). Shades–Card scale up.
// RTP = 0.319×1.5 + 0.030×3 + 0.012×8 + 0.005×20 + 0.001×75 + 0.0002×500 ≈ 0.940
var RouletteMultipliers = [NumSyms]float64{0, 1.5, 3, 8, 20, 75, 500}

// ── Reel weights (out of 100000) ─────────────────────────────────────────────
var rouletteWeights = [NumSyms]int{
	63280, // sym-0 Dice  (63.28% loss)
	31900, // sym-1 Shades
	3000,  // sym-2 Sneaker
	1200,  // sym-3 Chain
	500,   // sym-4 Watch
	100,   // sym-5 KeyFob
	20,    // sym-6 Card
} // total = 100000

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

// ── Spin result ───────────────────────────────────────────────────────────────

type SpinResult struct {
	ServerSeedHash string  `json:"server_seed_hash"`
	Nonce          int64   `json:"nonce"`
	WinSym         int     `json:"win_sym"` // 0-6; Dice = loss (×0)
	Payout         float64 `json:"payout"`
	Bet            float64 `json:"bet"`
}

// ── Spin ──────────────────────────────────────────────────────────────────────

func Spin(serverSeed string, nonce int64, bet float64) *SpinResult {
	r := newRNG(serverSeed, nonce)

	total := 0
	for _, w := range rouletteWeights {
		total += w
	}
	roll := r.intn(total)
	winSym := 0
	cum := 0
	for i, w := range rouletteWeights {
		cum += w
		if roll < cum {
			winSym = i
			break
		}
	}

	payout := RouletteMultipliers[winSym] * bet

	return &SpinResult{
		ServerSeedHash: HashSeed(serverSeed),
		Nonce:          nonce,
		WinSym:         winSym,
		Payout:         payout,
		Bet:            bet,
	}
}
