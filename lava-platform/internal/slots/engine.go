package slots

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math/big"
	"sort"
)

// ── Grid dimensions ────────────────────────────────────────────────────────────

const (
	NumCols    = 6
	NumRows    = 5
	ClusterMin = 8 // minimum cluster size for a payout
)

// ── Symbol indices ─────────────────────────────────────────────────────────────

const (
	SymHorseshoe = 0
	SymWhiskey   = 1
	SymBullet    = 2
	SymBadge     = 3
	SymLantern   = 4
	SymRevolver  = 5
	SymCowboyHat = 6
	SymGoldBag   = 7
	SymDynamite  = 8
	SymOutlaw    = 9
	SymSheriff   = 10
	SymWild      = 11
	SymScatter   = 12
	NumSymbols   = 13
)

var SymbolNames = [NumSymbols]string{
	"horseshoe", "whiskey", "bullet", "badge", "lantern",
	"revolver", "cowboy_hat", "gold_bag", "dynamite",
	"outlaw", "sheriff", "wild", "scatter",
}

// ── Config ─────────────────────────────────────────────────────────────────────

type Config struct {
	// Symbol weights — higher = more common. Index = symbol ID.
	Weights [NumSymbols]int
	// Multipliers available during free spins (placed randomly on winning tumbles)
	MultiplierValues []int
}

func DefaultConfig() *Config {
	return &Config{
		Weights: [NumSymbols]int{
			200, // horseshoe  (most common low)
			175, // whiskey
			160, // bullet
			145, // badge
			130, // lantern
			80,  // revolver  (mid)
			65,  // cowboy_hat
			55,  // gold_bag
			35,  // dynamite  (high)
			18,  // outlaw
			14,  // sheriff
			10,  // wild
			8,   // scatter
		},
		MultiplierValues: []int{2, 3, 5, 8, 10, 15},
	}
}

// ── Payout table ───────────────────────────────────────────────────────────────
// payoutEntry: minimum cluster size → multiplier of total_bet
type payoutEntry struct{ minSize int; mult float64 }

var payoutTable = map[int][]payoutEntry{
	SymHorseshoe: {{8, 0.30}, {10, 0.60}, {12, 1.00}, {15, 1.60}, {20, 2.50}, {30, 4.00}},
	SymWhiskey:   {{8, 0.40}, {10, 0.80}, {12, 1.20}, {15, 2.00}, {20, 3.50}, {30, 6.00}},
	SymBullet:    {{8, 0.50}, {10, 1.00}, {12, 1.60}, {15, 2.50}, {20, 4.50}, {30, 8.00}},
	SymBadge:     {{8, 0.60}, {10, 1.20}, {12, 2.00}, {15, 3.20}, {20, 5.50}, {30, 10.00}},
	SymLantern:   {{8, 0.80}, {10, 1.60}, {12, 2.50}, {15, 4.00}, {20, 7.00}, {30, 14.00}},
	SymRevolver:  {{8, 1.00}, {10, 2.00}, {12, 3.50}, {15, 6.00}, {20, 10.00}, {30, 20.00}},
	SymCowboyHat: {{8, 1.50}, {10, 3.00}, {12, 5.00}, {15, 8.00}, {20, 14.00}, {30, 30.00}},
	SymGoldBag:   {{8, 2.00}, {10, 4.00}, {12, 7.00}, {15, 12.00}, {20, 20.00}, {30, 50.00}},
	SymDynamite:  {{8, 3.00}, {10, 6.00}, {12, 10.00}, {15, 18.00}, {20, 35.00}, {30, 80.00}},
	SymOutlaw:    {{8, 5.00}, {10, 10.00}, {12, 16.00}, {15, 30.00}, {20, 60.00}, {30, 200.00}},
	SymSheriff:   {{8, 8.00}, {10, 16.00}, {12, 25.00}, {15, 50.00}, {20, 100.00}, {30, 500.00}},
}

// ── Data types ─────────────────────────────────────────────────────────────────

type Grid [NumRows][NumCols]int

type Cell struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

type Cluster struct {
	Symbol int     `json:"symbol"`
	Size   int     `json:"size"`
	Cells  []Cell  `json:"cells"`
	Mult   float64 `json:"mult"` // payout multiplier (before bet scaling)
}

type CascadeStep struct {
	Grid     Grid      `json:"grid"`
	Clusters []Cluster `json:"clusters"`
	Payout   float64   `json:"payout"` // sum of cluster payouts * bet
}

type SpinResult struct {
	ServerSeedHash string        `json:"server_seed_hash"`
	Nonce          int64         `json:"nonce"`
	InitialGrid    Grid          `json:"initial_grid"`
	Cascades       []CascadeStep `json:"cascades"`
	TotalPayout    float64       `json:"total_payout"`
	ScatterCount   int           `json:"scatter_count"`
	FreeSpinsAwarded int         `json:"free_spins_awarded"`
	Bet            float64       `json:"bet"`
}

// ── RNG from HMAC-SHA256 ───────────────────────────────────────────────────────

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
			// Extend: HMAC(seed, nonce || pos)
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

// intn returns a uniformly random int in [0, n)
func (r *rng) intn(n int) int {
	if n <= 0 {
		return 0
	}
	max := new(big.Int).SetInt64(int64(n))
	b := r.nextBytes(4)
	v := new(big.Int).SetBytes(b)
	return int(new(big.Int).Mod(v, max).Int64())
}

// ── Symbol picker (weighted random) ───────────────────────────────────────────

func pickSymbol(cfg *Config, r *rng) int {
	total := 0
	for _, w := range cfg.Weights {
		total += w
	}
	roll := r.intn(total)
	cumulative := 0
	for sym, w := range cfg.Weights {
		cumulative += w
		if roll < cumulative {
			return sym
		}
	}
	return 0
}

// ── Grid generation ────────────────────────────────────────────────────────────

func generateGrid(cfg *Config, r *rng) Grid {
	var g Grid
	for row := 0; row < NumRows; row++ {
		for col := 0; col < NumCols; col++ {
			g[row][col] = pickSymbol(cfg, r)
		}
	}
	return g
}

// ── Cluster detection (BFS, 4-directional adjacency) ──────────────────────────

func findClusters(g Grid) []Cluster {
	visited := [NumRows][NumCols]bool{}
	var clusters []Cluster

	for row := 0; row < NumRows; row++ {
		for col := 0; col < NumCols; col++ {
			if visited[row][col] {
				continue
			}
			sym := g[row][col]
			// Scatter and Wild don't form standard clusters
			if sym == SymScatter {
				visited[row][col] = true
				continue
			}

			// BFS
			type pos struct{ r, c int }
			queue := []pos{{row, col}}
			var cells []Cell
			bfsVisited := [NumRows][NumCols]bool{}
			bfsVisited[row][col] = true

			for len(queue) > 0 {
				cur := queue[0]
				queue = queue[1:]

				cellSym := g[cur.r][cur.c]
				// Wild counts as the target symbol for connectivity
				if cellSym != sym && cellSym != SymWild {
					continue
				}

				cells = append(cells, Cell{Row: cur.r, Col: cur.c})
				visited[cur.r][cur.c] = true

				dirs := []pos{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}
				for _, d := range dirs {
					nr, nc := cur.r+d.r, cur.c+d.c
					if nr < 0 || nr >= NumRows || nc < 0 || nc >= NumCols {
						continue
					}
					if bfsVisited[nr][nc] {
						continue
					}
					nSym := g[nr][nc]
					if nSym == sym || nSym == SymWild {
						bfsVisited[nr][nc] = true
						queue = append(queue, pos{nr, nc})
					}
				}
			}

			if len(cells) >= ClusterMin {
				mult := clusterMult(sym, len(cells))
				clusters = append(clusters, Cluster{
					Symbol: sym,
					Size:   len(cells),
					Cells:  cells,
					Mult:   mult,
				})
			}
		}
	}

	// Sort clusters by symbol value descending (premium first) for display
	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].Symbol > clusters[j].Symbol
	})
	return clusters
}

// clusterMult looks up the payout multiplier for a symbol + cluster size.
func clusterMult(sym, size int) float64 {
	entries, ok := payoutTable[sym]
	if !ok {
		return 0
	}
	mult := 0.0
	for _, e := range entries {
		if size >= e.minSize {
			mult = e.mult
		}
	}
	return mult
}

// ── Tumble ─────────────────────────────────────────────────────────────────────

// applyTumble removes all winning cells and fills columns from the top.
func applyTumble(g Grid, clusters []Cluster, cfg *Config, r *rng) Grid {
	remove := [NumRows][NumCols]bool{}
	for _, cl := range clusters {
		for _, c := range cl.Cells {
			remove[c.Row][c.Col] = true
		}
	}

	var result Grid
	for col := 0; col < NumCols; col++ {
		// Collect non-removed symbols in this column (bottom to top)
		remaining := make([]int, 0, NumRows)
		for row := NumRows - 1; row >= 0; row-- {
			if !remove[row][col] {
				remaining = append(remaining, g[row][col])
			}
		}
		// Fill the rest with new symbols
		for len(remaining) < NumRows {
			remaining = append(remaining, pickSymbol(cfg, r))
		}
		// Write back (bottom to top)
		for i := 0; i < NumRows; i++ {
			result[NumRows-1-i][col] = remaining[i]
		}
	}
	return result
}

// ── Scatter counter ────────────────────────────────────────────────────────────

func countScatter(g Grid) int {
	n := 0
	for row := 0; row < NumRows; row++ {
		for col := 0; col < NumCols; col++ {
			if g[row][col] == SymScatter {
				n++
			}
		}
	}
	return n
}

func freeSpinsForScatter(n int) int {
	switch {
	case n >= 6:
		return 20
	case n >= 5:
		return 15
	case n >= 4:
		return 10
	default:
		return 0
	}
}

// ── Hash helper ────────────────────────────────────────────────────────────────

func HashSeed(seed string) string {
	h := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(h[:])
}

// ── Main spin function ─────────────────────────────────────────────────────────

func Spin(cfg *Config, serverSeed string, nonce int64, bet float64) *SpinResult {
	r := newRNG(serverSeed, nonce)
	grid := generateGrid(cfg, r)

	result := &SpinResult{
		ServerSeedHash: HashSeed(serverSeed),
		Nonce:          nonce,
		InitialGrid:    grid,
		Bet:            bet,
	}

	// Count scatters on initial grid
	result.ScatterCount = countScatter(grid)
	result.FreeSpinsAwarded = freeSpinsForScatter(result.ScatterCount)

	// Cascade loop
	current := grid
	for {
		clusters := findClusters(current)
		if len(clusters) == 0 {
			break
		}

		stepPayout := 0.0
		for _, cl := range clusters {
			stepPayout += cl.Mult * bet
		}

		result.Cascades = append(result.Cascades, CascadeStep{
			Grid:     current,
			Clusters: clusters,
			Payout:   stepPayout,
		})
		result.TotalPayout += stepPayout

		current = applyTumble(current, clusters, cfg, r)
	}

	// If no cascades, store the final grid as the only step (for frontend reference)
	if len(result.Cascades) == 0 {
		result.Cascades = append(result.Cascades, CascadeStep{
			Grid:     grid,
			Clusters: nil,
			Payout:   0,
		})
	}

	return result
}
