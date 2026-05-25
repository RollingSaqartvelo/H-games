package pirate

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math/big"
)

// ── Grid dimensions ────────────────────────────────────────────────────────────

const (
	NumCols = 5
	NumRows = 3
)

// ── Symbol indices ─────────────────────────────────────────────────────────────

const (
	SymChest    = 0
	SymCoins    = 1
	SymRum      = 2
	SymAnchor   = 3
	SymWild     = 4
	SymScatter  = 5
	SymDoubloon = 6
	SymBoost    = 7
	SymMystery  = 8
	SymJ        = 9
	SymQ        = 10
	SymK        = 11
	SymA        = 12
	NumSymbols  = 13
)

var DefaultWeights = [NumSymbols]int{
	180, 200, 220, 220, 90, 35, 55, 15, 40, 180, 170, 160, 150,
}

var FreeSpinWeights = [NumSymbols]int{
	280, 300, 320, 320, 140, 50, 80, 20, 60, 0, 0, 0, 0,
}

// Doubloon cash value multipliers (index 0-14, multiply by bet)
var doubloonCashMults = [15]float64{
	0.75, 1.50, 2.25, 3.00, 3.75, 4.50, 5.25, 6.00,
	6.75, 7.50, 8.25, 9.00, 9.75, 10.50, 11.25,
}

// payouts: symbol → [3x multiplier] for 3-of-a-kind, 4-of-a-kind, 5-of-a-kind
var payouts = map[int][3]float64{
	SymChest:   {0.45, 1.50, 7.50},
	SymCoins:   {0.15, 1.20, 6.00},
	SymRum:     {0.15, 0.90, 4.50},
	SymAnchor:  {0.15, 0.75, 3.00},
	SymWild:    {0.60, 1.80, 9.00},
	SymScatter: {1.50, 0, 0},
	SymJ:       {0.05, 0.15, 0.40},
	SymQ:       {0.05, 0.20, 0.50},
	SymK:       {0.10, 0.25, 0.60},
	SymA:       {0.10, 0.30, 0.75},
}

// 25 fixed paylines for 5x3 grid — each is [row0, row1, row2, row3, row4]
var Paylines = [25][5]int{
	{1, 1, 1, 1, 1}, // line 1: middle row
	{0, 0, 0, 0, 0}, // line 2: top row
	{2, 2, 2, 2, 2}, // line 3: bottom row
	{0, 1, 2, 1, 0}, // line 4: V shape
	{2, 1, 0, 1, 2}, // line 5: inverted V
	{0, 0, 1, 2, 2}, // line 6
	{2, 2, 1, 0, 0}, // line 7
	{1, 0, 0, 0, 1}, // line 8
	{1, 2, 2, 2, 1}, // line 9
	{0, 1, 1, 1, 0}, // line 10
	{2, 1, 1, 1, 2}, // line 11
	{0, 1, 0, 1, 0}, // line 12
	{2, 1, 2, 1, 2}, // line 13
	{1, 0, 1, 0, 1}, // line 14
	{1, 2, 1, 2, 1}, // line 15
	{0, 0, 0, 1, 2}, // line 16
	{2, 2, 2, 1, 0}, // line 17
	{0, 1, 2, 2, 2}, // line 18
	{2, 1, 0, 0, 0}, // line 19
	{1, 1, 0, 1, 1}, // line 20
	{1, 1, 2, 1, 1}, // line 21
	{0, 0, 1, 1, 2}, // line 22
	{2, 2, 1, 1, 0}, // line 23
	{0, 1, 1, 0, 0}, // line 24
	{2, 1, 1, 2, 2}, // line 25
}

// ── Config ─────────────────────────────────────────────────────────────────────

type Config struct {
	Weights         [NumSymbols]int
	FreeSpinWeights [NumSymbols]int
	BonusBuyCost    float64
}

// ── Data types ─────────────────────────────────────────────────────────────────

type DoubloonCell struct {
	Value   float64 `json:"value"`
	Type    string  `json:"type"`    // "cash"|"MINI"|"MINOR"|"MAJOR"|"MEGA"|"GRAND"
	Boosted bool    `json:"boosted"`
}

type HoldWinState struct {
	Grid       [NumRows][NumCols]*DoubloonCell `json:"grid"`
	SpinsLeft  int                             `json:"spins_left"`
	TotalValue float64                         `json:"total_value"`
	IsBoosted  bool                            `json:"is_boosted"`
	Jackpot    string                          `json:"jackpot"`
	Complete   bool                            `json:"complete"`
	TriggerBet float64                         `json:"trigger_bet"`
}

type PaylineWin struct {
	Line   int     `json:"line"`
	Symbol int     `json:"symbol"`
	Count  int     `json:"count"`
	Payout float64 `json:"payout"`
}

type SpinResult struct {
	SpinID           string        `json:"spin_id"`
	ServerSeedHash   string        `json:"server_seed_hash"`
	Nonce            int64         `json:"nonce"`
	Grid             [NumRows][NumCols]int `json:"grid"`
	Paylines         []PaylineWin  `json:"paylines"`
	TotalPayout      float64       `json:"total_payout"`
	ScatterCount     int           `json:"scatter_count"`
	FreeSpinsAwarded int           `json:"free_spins_awarded"`
	HoldWinTriggered bool          `json:"hold_win_triggered"`
	HoldWinResult    *HoldWinState `json:"hold_win_result,omitempty"`
	Balance          float64       `json:"balance"`
	Bet              float64       `json:"bet"`
	FreeSpinsLeft    int           `json:"free_spins_left"`
}

// ── RNG (HMAC-SHA256) ─────────────────────────────────────────────────────────

type rng struct {
	seed  []byte
	nonce int64
	pos   int
	buf   []byte
}

func newRNG(serverSeed string, nonce int64) *rng {
	mac := hmac.New(sha256.New, []byte(serverSeed))
	nonceBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(nonceBytes, uint64(nonce))
	mac.Write(nonceBytes)
	hash := mac.Sum(nil)
	return &rng{seed: []byte(serverSeed), nonce: nonce, buf: hash}
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
	max := new(big.Int).SetInt64(int64(n))
	b := r.nextBytes(4)
	v := new(big.Int).SetBytes(b)
	return int(new(big.Int).Mod(v, max).Int64())
}

// ── Symbol picker ─────────────────────────────────────────────────────────────

func pickSymbolWeighted(weights [NumSymbols]int, r *rng) int {
	total := 0
	for _, w := range weights {
		total += w
	}
	if total == 0 {
		return 0
	}
	roll := r.intn(total)
	cumulative := 0
	for sym, w := range weights {
		cumulative += w
		if roll < cumulative {
			return sym
		}
	}
	return 0
}

// ── Grid generation ───────────────────────────────────────────────────────────

func generateGrid(weights [NumSymbols]int, r *rng) [NumRows][NumCols]int {
	var g [NumRows][NumCols]int
	for row := 0; row < NumRows; row++ {
		for col := 0; col < NumCols; col++ {
			g[row][col] = pickSymbolWeighted(weights, r)
		}
	}
	return g
}

// ── Payline evaluation ────────────────────────────────────────────────────────

// isPayingSymbol returns true if the symbol can appear on a paying line
// (not scatter/doubloon/boost/mystery)
func isPayingSymbol(sym int) bool {
	return sym != SymScatter && sym != SymDoubloon && sym != SymBoost && sym != SymMystery
}

func evaluatePaylines(grid [NumRows][NumCols]int, bet float64) []PaylineWin {
	var wins []PaylineWin

	for lineIdx, line := range Paylines {
		// Get first symbol (skip wilds to find base symbol)
		firstSym := -1
		for col := 0; col < NumCols; col++ {
			sym := grid[line[col]][col]
			if sym != SymWild && isPayingSymbol(sym) {
				firstSym = sym
				break
			}
		}
		if firstSym == -1 {
			// All wilds — count as wild win
			firstSym = SymWild
		}

		// Count consecutive matching symbols from left
		count := 0
		for col := 0; col < NumCols; col++ {
			sym := grid[line[col]][col]
			if sym == firstSym || sym == SymWild {
				count++
			} else {
				break
			}
		}

		if count < 3 {
			continue
		}

		// Look up payout (index: 3=0, 4=1, 5=2)
		pay, ok := payouts[firstSym]
		if !ok {
			continue
		}
		multIdx := count - 3
		if multIdx > 2 {
			multIdx = 2
		}
		mult := pay[multIdx]
		if mult == 0 {
			continue
		}

		wins = append(wins, PaylineWin{
			Line:   lineIdx + 1,
			Symbol: firstSym,
			Count:  count,
			Payout: mult * bet,
		})
	}

	return wins
}

// ── Scatter ───────────────────────────────────────────────────────────────────

func countScatter(grid [NumRows][NumCols]int) int {
	n := 0
	for row := 0; row < NumRows; row++ {
		for col := 0; col < NumCols; col++ {
			if grid[row][col] == SymScatter {
				n++
			}
		}
	}
	return n
}

func freeSpinsForScatter(n int) int {
	switch {
	case n >= 5:
		return 20
	case n >= 4:
		return 12
	case n >= 3:
		return 8
	default:
		return 0
	}
}

// ── Doubloon counting ─────────────────────────────────────────────────────────

func countDoubloon(grid [NumRows][NumCols]int) int {
	n := 0
	for row := 0; row < NumRows; row++ {
		for col := 0; col < NumCols; col++ {
			sym := grid[row][col]
			if sym == SymDoubloon || sym == SymBoost || sym == SymMystery {
				n++
			}
		}
	}
	return n
}

// ── Hold & Win ────────────────────────────────────────────────────────────────

func newDoubloonCell(r *rng, bet float64) *DoubloonCell {
	// Weighted choice: 88% cash, 6% MINI, 3.5% MINOR, 2% MAJOR, 0.4% MEGA, 0.1% GRAND
	roll := r.intn(1000)
	switch {
	case roll < 880: // cash
		idx := r.intn(len(doubloonCashMults))
		return &DoubloonCell{
			Value: doubloonCashMults[idx] * bet,
			Type:  "cash",
		}
	case roll < 940: // MINI
		return &DoubloonCell{Value: 20 * bet, Type: "MINI"}
	case roll < 975: // MINOR
		return &DoubloonCell{Value: 50 * bet, Type: "MINOR"}
	case roll < 995: // MAJOR
		return &DoubloonCell{Value: 150 * bet, Type: "MAJOR"}
	case roll < 999: // MEGA
		return &DoubloonCell{Value: 2000 * bet, Type: "MEGA"}
	default: // GRAND
		return &DoubloonCell{Value: 10000 * bet, Type: "GRAND"}
	}
}

// triggerHoldWin creates the initial HoldWinState from the triggering grid.
func triggerHoldWin(grid [NumRows][NumCols]int, r *rng, bet float64) *HoldWinState {
	state := &HoldWinState{
		SpinsLeft:  3,
		TriggerBet: bet,
	}

	total := 0.0
	for row := 0; row < NumRows; row++ {
		for col := 0; col < NumCols; col++ {
			sym := grid[row][col]
			if sym == SymDoubloon || sym == SymBoost || sym == SymMystery {
				cell := newDoubloonCell(r, bet)
				if sym == SymBoost {
					// Boost upgrades all existing values
					state.IsBoosted = true
					cell.Type = "cash"
					cell.Value = doubloonCashMults[r.intn(len(doubloonCashMults))] * bet
					cell.Boosted = true
				} else if sym == SymMystery {
					// Mystery transforms to random jackpot type or boost
					mystRoll := r.intn(100)
					switch {
					case mystRoll < 40:
						cell = &DoubloonCell{Value: 20 * bet, Type: "MINI"}
					case mystRoll < 65:
						cell = &DoubloonCell{Value: 50 * bet, Type: "MINOR"}
					case mystRoll < 82:
						cell = &DoubloonCell{Value: 150 * bet, Type: "MAJOR"}
					case mystRoll < 93:
						cell = &DoubloonCell{Value: 2000 * bet, Type: "MEGA"}
					default:
						cell = &DoubloonCell{Value: 10000 * bet, Type: "GRAND"}
					}
				}
				state.Grid[row][col] = cell
				total += cell.Value
			}
		}
	}

	// Apply boost multiplier if triggered
	if state.IsBoosted {
		boostMult := 2.5 + float64(r.intn(3))*1.25 // 2.5, 3.75, or 5.0
		newTotal := 0.0
		for row := 0; row < NumRows; row++ {
			for col := 0; col < NumCols; col++ {
				if state.Grid[row][col] != nil && !state.Grid[row][col].Boosted {
					state.Grid[row][col].Value *= boostMult
					state.Grid[row][col].Boosted = true
				}
				if state.Grid[row][col] != nil {
					newTotal += state.Grid[row][col].Value
				}
			}
		}
		total = newTotal
	}

	state.TotalValue = total
	return state
}

// runHoldWinRespin performs one respin step of the Hold&Win feature.
// Returns the updated state (Complete=true when done).
func runHoldWinRespin(state *HoldWinState, r *rng) *HoldWinState {
	if state.Complete {
		return state
	}

	bet := state.TriggerBet
	newCellAdded := false

	for row := 0; row < NumRows; row++ {
		for col := 0; col < NumCols; col++ {
			if state.Grid[row][col] != nil {
				continue // already locked
			}

			// 6% chance to land doubloon-type symbol on each empty cell
			roll := r.intn(100)
			if roll >= 6 {
				continue
			}

			// Pick what kind: doubloon(85%), boost(5%), mystery(10%)
			symRoll := r.intn(100)
			var cell *DoubloonCell
			if symRoll < 85 {
				cell = newDoubloonCell(r, bet)
			} else if symRoll < 90 {
				// Boost
				boostMult := 2.5 + float64(r.intn(3))*1.25
				state.IsBoosted = true
				// Boost all existing cells
				for rr := 0; rr < NumRows; rr++ {
					for cc := 0; cc < NumCols; cc++ {
						if state.Grid[rr][cc] != nil {
							state.Grid[rr][cc].Value *= boostMult
							state.Grid[rr][cc].Boosted = true
						}
					}
				}
				cell = &DoubloonCell{
					Value:   doubloonCashMults[r.intn(len(doubloonCashMults))] * bet,
					Type:    "cash",
					Boosted: true,
				}
			} else {
				// Mystery
				mystRoll := r.intn(100)
				switch {
				case mystRoll < 40:
					cell = &DoubloonCell{Value: 20 * bet, Type: "MINI"}
				case mystRoll < 65:
					cell = &DoubloonCell{Value: 50 * bet, Type: "MINOR"}
				case mystRoll < 82:
					cell = &DoubloonCell{Value: 150 * bet, Type: "MAJOR"}
				case mystRoll < 93:
					cell = &DoubloonCell{Value: 2000 * bet, Type: "MEGA"}
				default:
					cell = &DoubloonCell{Value: 10000 * bet, Type: "GRAND"}
				}
			}

			state.Grid[row][col] = cell
			newCellAdded = true
		}
	}

	if newCellAdded {
		// Reset spins counter
		state.SpinsLeft = 3
	} else {
		state.SpinsLeft--
	}

	// Recalculate total
	total := 0.0
	filledCells := 0
	for row := 0; row < NumRows; row++ {
		for col := 0; col < NumCols; col++ {
			if state.Grid[row][col] != nil {
				total += state.Grid[row][col].Value
				filledCells++
			}
		}
	}
	state.TotalValue = total

	// Check completion
	if state.SpinsLeft <= 0 || filledCells == NumRows*NumCols {
		state.Complete = true
		if filledCells == NumRows*NumCols {
			state.Jackpot = "GRAND"
			// значение уже накоплено в ячейках, двойное добавление убрано
		}
	}

	return state
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func HashSeed(seed string) string {
	h := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(h[:])
}

func serverSeed() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ── Main Spin function ─────────────────────────────────────────────────────────

func Spin(cfg *Config, seed string, nonce int64, bet float64, isFreeSpin bool, bonusBuy bool) *SpinResult {
	r := newRNG(seed, nonce)

	weights := cfg.Weights
	if isFreeSpin {
		weights = cfg.FreeSpinWeights
	}

	// For bonus buy: guarantee Hold&Win by forcing 3+ doubloons
	grid := generateGrid(weights, r)
	if bonusBuy {
		grid = forceDoubloons(grid, r, 3)
	}

	result := &SpinResult{
		ServerSeedHash: HashSeed(seed),
		Nonce:          nonce,
		Grid:           grid,
		Bet:            bet,
	}

	// Count scatters
	scatterCount := countScatter(grid)
	result.ScatterCount = scatterCount
	result.FreeSpinsAwarded = freeSpinsForScatter(scatterCount)

	// Evaluate paylines
	lineWins := evaluatePaylines(grid, bet)
	result.Paylines = lineWins
	totalPayout := 0.0
	for _, w := range lineWins {
		totalPayout += w.Payout
	}
	result.TotalPayout = totalPayout

	// Check Hold&Win trigger: 3+ doubloon-type symbols and roll chance
	doubloonCount := countDoubloon(grid)
	triggerThreshold := 0
	switch {
	case doubloonCount >= 6:
		triggerThreshold = 100 // guaranteed
	case doubloonCount >= 5:
		triggerThreshold = 90
	case doubloonCount >= 4:
		triggerThreshold = 75
	case doubloonCount >= 3:
		triggerThreshold = 60
	}

	if bonusBuy {
		triggerThreshold = 100
	}

	if doubloonCount >= 3 && r.intn(100) < triggerThreshold {
		result.HoldWinTriggered = true
		result.HoldWinResult = triggerHoldWin(grid, r, bet)
		result.TotalPayout += result.HoldWinResult.TotalValue
	}

	return result
}

// forceDoubloons ensures at least minCount doubloon symbols are on the grid.
func forceDoubloons(grid [NumRows][NumCols]int, r *rng, minCount int) [NumRows][NumCols]int {
	current := countDoubloon(grid)
	if current >= minCount {
		return grid
	}

	type pos struct{ row, col int }
	var available []pos
	for row := 0; row < NumRows; row++ {
		for col := 0; col < NumCols; col++ {
			sym := grid[row][col]
			if sym != SymDoubloon && sym != SymBoost && sym != SymMystery && sym != SymScatter && sym != SymWild {
				available = append(available, pos{row, col})
			}
		}
	}

	needed := minCount - current
	for i := 0; i < needed && len(available) > 0; i++ {
		idx := r.intn(len(available))
		p := available[idx]
		grid[p.row][p.col] = SymDoubloon
		available = append(available[:idx], available[idx+1:]...)
	}
	return grid
}
