package slots

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lava-platform/internal/domain"
	"github.com/lava-platform/internal/middleware"
	walletSvc "github.com/lava-platform/internal/wallet/service"
	"github.com/shopspring/decimal"
)

// bonusSession tracks persistent bonus-round state per player.
type bonusSession struct {
	SpinsLeft int
	MultTotal int // accumulated bonus multiplier (starts at 0, grows with wins)
}

// Handler handles H-SLOTS HTTP requests.
type Handler struct {
	wallet *walletSvc.InternalWalletProvider
	cfg    *Config

	mu      sync.Mutex
	nonces  map[string]int64 // userID → next nonce

	bonusSessions sync.Map // userID → *bonusSession
}

func NewHandler(wallet *walletSvc.InternalWalletProvider) *Handler {
	return &Handler{
		wallet: wallet,
		cfg:    DefaultConfig(),
		nonces: make(map[string]int64),
	}
}

func (h *Handler) getBonusSession(userID string) *bonusSession {
	val, ok := h.bonusSessions.Load(userID)
	if !ok {
		return nil
	}
	return val.(*bonusSession)
}

func (h *Handler) setBonusSession(userID string, bs *bonusSession) {
	if bs == nil || bs.SpinsLeft <= 0 {
		h.bonusSessions.Delete(userID)
	} else {
		h.bonusSessions.Store(userID, bs)
	}
}

func (h *Handler) nextNonce(userID string) int64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.nonces[userID]++
	return h.nonces[userID]
}

// serverSeed generates a random server seed for each spin.
func serverSeed() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ── POST /tma/v1/slots/spin ───────────────────────────────────────────────────

// BonusBuy tiers: 0=disabled, 1=standard(100×), 2=enhanced(125×), 3=super(150×)
var bonusBuyCost = [4]float64{0, 100, 125, 150}

// minScattersForTier guarantees free-spin triggers: tier1→4, tier2→5, tier3→6
var minScattersForTier = [4]int{0, 4, 5, 6}

type spinReq struct {
	Bet           float64 `json:"bet"            binding:"required,min=0.01,max=10000"`
	FreeSpin      bool    `json:"free_spin"`
	BonusBuyTier  int     `json:"bonus_buy_tier"` // 0=normal, 1/2/3=bonus tiers
}

func (h *Handler) Spin(c *gin.Context) {
	sess := middleware.SessionFromCtx(c)
	if sess == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req spinReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// ── Validate bonus buy tier ───────────────────────────────────────────────
	tier := req.BonusBuyTier
	if tier < 0 || tier > 3 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid bonus_buy_tier"})
		return
	}

	// For bonus buy the actual debit amount = bet × cost multiplier
	effectiveBet := req.Bet
	if tier > 0 {
		effectiveBet = req.Bet * bonusBuyCost[tier]
	}

	betDec := decimal.NewFromFloat(effectiveBet)
	txLabel := "slot-bet"
	if tier > 0 {
		txLabel = fmt.Sprintf("slot-bonus-%d", tier)
	}
	betTxID := fmt.Sprintf("%s-%s-%d", txLabel, sess.UserID, time.Now().UnixNano())
	roundID := betTxID

	// ── Debit wallet (skip for free spin) ────────────────────────────────────
	var balanceAfterBet decimal.Decimal
	if !req.FreeSpin {
		debitResp, err := h.wallet.Debit(c.Request.Context(), &domain.DebitRequest{
			UserID:        sess.UserID,
			Amount:        betDec,
			Currency:      sess.Currency,
			TransactionID: betTxID,
			RoundID:       roundID,
		})
		if err != nil {
			if err == domain.ErrInsufficientFunds {
				c.JSON(http.StatusPaymentRequired, gin.H{"error": "insufficient funds"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "wallet error"})
			return
		}
		balanceAfterBet = debitResp.Balance
	} else {
		balResp, err := h.wallet.GetBalance(c.Request.Context(), &domain.BalanceRequest{
			UserID:   sess.UserID,
			Currency: sess.Currency,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "wallet error"})
			return
		}
		balanceAfterBet = balResp.Balance
	}

	// ── Bonus session lookup ──────────────────────────────────────────────────
	bs := h.getBonusSession(sess.UserID)
	isFree := req.FreeSpin && bs != nil && bs.SpinsLeft > 0
	currentMult := 0
	if isFree && bs != nil {
		currentMult = bs.MultTotal
		bs.SpinsLeft--
	}

	// ── Run spin ──────────────────────────────────────────────────────────────
	seed := serverSeed()
	nonce := h.nextNonce(sess.UserID)

	minScatters := 0
	if tier > 0 {
		minScatters = minScattersForTier[tier]
	}

	result := Spin(h.cfg, seed, nonce, req.Bet, minScatters, isFree, currentMult)

	// ── Update bonus session with collected multipliers ───────────────────────
	if isFree && bs != nil {
		bs.MultTotal = currentMult + result.MultCollected
		h.setBonusSession(sess.UserID, bs)
	}

	// ── Award new free spins (scatter trigger or re-trigger) ──────────────────
	bonusSpinsLeft := 0
	bonusMultTotal := 0
	if result.FreeSpinsAwarded > 0 {
		existing := h.getBonusSession(sess.UserID)
		newBS := &bonusSession{SpinsLeft: result.FreeSpinsAwarded, MultTotal: 0}
		if existing != nil {
			newBS.SpinsLeft += existing.SpinsLeft
			newBS.MultTotal = existing.MultTotal
		}
		h.setBonusSession(sess.UserID, newBS)
	}
	if final := h.getBonusSession(sess.UserID); final != nil {
		bonusSpinsLeft = final.SpinsLeft
		bonusMultTotal = final.MultTotal
	}

	// ── Credit winnings ───────────────────────────────────────────────────────
	finalBalance := balanceAfterBet
	if result.TotalPayout > 0 {
		winDec := decimal.NewFromFloat(result.TotalPayout)
		txSuffix := "win"
		if isFree {
			txSuffix = "freewin"
		}
		winTxID := fmt.Sprintf("slot-%s-%s-%d", txSuffix, sess.UserID, time.Now().UnixNano())
		creditResp, err := h.wallet.Credit(c.Request.Context(), &domain.CreditRequest{
			UserID:        sess.UserID,
			Amount:        winDec,
			Currency:      sess.Currency,
			TransactionID: winTxID,
			RoundID:       roundID,
		})
		if err == nil {
			finalBalance = creditResp.Balance
		}
	}

	// ── Build response ────────────────────────────────────────────────────────
	balF, _ := finalBalance.Float64()
	c.JSON(http.StatusOK, gin.H{
		"spin_id":              roundID,
		"server_seed_hash":     result.ServerSeedHash,
		"nonce":                result.Nonce,
		"initial_grid":         gridToSlice(result.InitialGrid),
		"cascades":             cascadesToJSON(result.Cascades),
		"total_payout":         result.TotalPayout,
		"scatter_count":        result.ScatterCount,
		"free_spins_awarded":   result.FreeSpinsAwarded,
		"multiplier_cells":     result.MultiplierCells,
		"mult_collected":       result.MultCollected,
		"bonus_mult_total":     bonusMultTotal,
		"bonus_spins_left":     bonusSpinsLeft,
		"balance":              balF,
		"bet":                  req.Bet,
		"bonus_buy_tier":       tier,
		"effective_cost":       effectiveBet,
	})
}

// ── GET /tma/v1/slots/config ──────────────────────────────────────────────────

func (h *Handler) Config(c *gin.Context) {
	sess := middleware.SessionFromCtx(c)
	if sess == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	balResp, err := h.wallet.GetBalance(c.Request.Context(), &domain.BalanceRequest{
		UserID:   sess.UserID,
		Currency: sess.Currency,
	})
	bal := 0.0
	if err == nil {
		bal, _ = balResp.Balance.Float64()
	}

	// Include active bonus session state so the frontend can restore auto-spin on reload
	bsLeft := 0
	bsMult := 0
	if bs := h.getBonusSession(sess.UserID); bs != nil {
		bsLeft = bs.SpinsLeft
		bsMult = bs.MultTotal
	}

	c.JSON(http.StatusOK, gin.H{
		"balance":          bal,
		"currency":         sess.Currency,
		"min_bet":          0.10,
		"max_bet":          1000.0,
		"default_bet":      1.00,
		"grid_cols":        NumCols,
		"grid_rows":        NumRows,
		"cluster_min":      ClusterMin,
		"symbols":          symbolsInfo(),
		"bonus_spins_left": bsLeft,
		"bonus_mult_total": bsMult,
	})
}

// ── Admin: update symbol weights ──────────────────────────────────────────────

func (h *Handler) UpdateWeights(c *gin.Context) {
	var body struct {
		Weights [NumSymbols]int `json:"weights" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.mu.Lock()
	h.cfg.Weights = body.Weights
	h.mu.Unlock()
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func gridToSlice(g Grid) [][]int {
	out := make([][]int, NumRows)
	for r := 0; r < NumRows; r++ {
		out[r] = make([]int, NumCols)
		for col := 0; col < NumCols; col++ {
			out[r][col] = g[r][col]
		}
	}
	return out
}

func cascadesToJSON(cascades []CascadeStep) []map[string]interface{} {
	out := make([]map[string]interface{}, len(cascades))
	for i, step := range cascades {
		clusters := make([]map[string]interface{}, len(step.Clusters))
		for j, cl := range step.Clusters {
			cells := make([]map[string]int, len(cl.Cells))
			for k, cell := range cl.Cells {
				cells[k] = map[string]int{"row": cell.Row, "col": cell.Col}
			}
			clusters[j] = map[string]interface{}{
				"symbol": cl.Symbol,
				"size":   cl.Size,
				"cells":  cells,
				"mult":   cl.Mult,
			}
		}
		out[i] = map[string]interface{}{
			"grid":     gridToSlice(step.Grid),
			"clusters": clusters,
			"payout":   step.Payout,
		}
	}
	return out
}

func symbolsInfo() []map[string]interface{} {
	out := make([]map[string]interface{}, NumSymbols)
	for i := 0; i < NumSymbols; i++ {
		tier := "low"
		switch {
		case i >= SymRevolver && i <= SymGoldBag:
			tier = "mid"
		case i >= SymDynamite && i <= SymSheriff:
			tier = "high"
		case i == SymWild:
			tier = "wild"
		case i == SymScatter:
			tier = "scatter"
		}
		payouts := []map[string]interface{}{}
		if entries, ok := payoutTable[i]; ok {
			for _, e := range entries {
				payouts = append(payouts, map[string]interface{}{
					"min_size": e.minSize,
					"mult":     e.mult,
				})
			}
		}
		out[i] = map[string]interface{}{
			"id":      i,
			"name":    SymbolNames[i],
			"tier":    tier,
			"img":     "/assets/h-slots/outlaw-gold/symbols/" + SymbolNames[i] + ".png",
			"payouts": payouts,
		}
	}
	return out
}

// unused import guard
var _ = strconv.Itoa
