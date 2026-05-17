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
	SymDice    = 0 // loss symbol (replaces bomb)
	SymShades  = 1
	SymSneaker = 2
	SymChain   = 3
	SymWatch   = 4
	SymKeyFob  = 5
	SymCard    = 6
	NumSyms    = 7
)

// ── Payout multipliers (× bet) ────────────────────────────────────────────────
// Dice = ×0 (loss). RTP ≈ 94%.
var RouletteMultipliers = [NumSyms]float64{0, 1.5, 3, 8, 20, 75, 500}

// ── Cursor weights (out of 100000) ───────────────────────────────────────────
// Cursor stops on these symbols; if it matches a reel symbol → win.
var rouletteWeights = [NumSyms]int{
	63280, // sym-0 Dice  — loss (63.28%)
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
	CursorSym      int     `json:"cursor_sym"` // symbol cursor lands on; 0=Dice=loss
	Reels          [3]int  `json:"reels"`       // center symbol on each of the 3 reels
	WinReel        int     `json:"win_reel"`    // which reel matched cursor (-1 = loss)
	Payout         float64 `json:"payout"`
	Bet            float64 `json:"bet"`
}

// ── Spin ──────────────────────────────────────────────────────────────────────

func Spin(serverSeed string, nonce int64, bet float64) *SpinResult {
	r := newRNG(serverSeed, nonce)

	// Pick cursor symbol
	total := 0
	for _, w := range rouletteWeights {
		total += w
	}
	roll := r.intn(total)
	cursorSym := 0
	cum := 0
	for i, w := range rouletteWeights {
		cum += w
		if roll < cum {
			cursorSym = i
			break
		}
	}

	payout := RouletteMultipliers[cursorSym] * bet

	var reels [3]int
	winReel := -1

	if cursorSym == SymDice {
		// Loss: random symbols on all reels (cursor landed on dice)
		for i := range reels {
			reels[i] = r.intn(NumSyms)
		}
	} else {
		// Win: place cursorSym on one random reel, others get different symbols
		winReel = r.intn(3)
		for i := range reels {
			if i == winReel {
				reels[i] = cursorSym
			} else {
				// Pick any symbol except cursorSym
				s := r.intn(NumSyms - 1)
				if s >= cursorSym {
					s++
				}
				reels[i] = s
			}
		}
	}

	return &SpinResult{
		ServerSeedHash: HashSeed(serverSeed),
		Nonce:          nonce,
		CursorSym:      cursorSym,
		Reels:          reels,
		WinReel:        winReel,
		Payout:         payout,
		Bet:            bet,
	}
}
