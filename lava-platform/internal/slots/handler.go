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

// Handler handles H-SLOTS HTTP requests.
type Handler struct {
	wallet *walletSvc.InternalWalletProvider
	cfg    *Config

	mu      sync.Mutex
	nonces  map[string]int64 // userID → next nonce
}

func NewHandler(wallet *walletSvc.InternalWalletProvider) *Handler {
	return &Handler{
		wallet: wallet,
		cfg:    DefaultConfig(),
		nonces: make(map[string]int64),
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

type spinReq struct {
	Bet      float64 `json:"bet"      binding:"required,min=0.01,max=10000"`
	FreeSpin bool    `json:"free_spin"`
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

	betDec := decimal.NewFromFloat(req.Bet)
	betTxID := fmt.Sprintf("slot-bet-%s-%d", sess.UserID, time.Now().UnixNano())
	roundID := betTxID // slots use spin ID as round ID

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
		// Free spin: just fetch balance
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

	// ── Run spin ──────────────────────────────────────────────────────────────
	seed := serverSeed()
	nonce := h.nextNonce(sess.UserID)
	bet := req.Bet
	if req.FreeSpin {
		bet = req.Bet // bet amount still used for payout scaling in free spins
	}

	result := Spin(h.cfg, seed, nonce, bet)

	// ── Credit winnings ───────────────────────────────────────────────────────
	finalBalance := balanceAfterBet
	if result.TotalPayout > 0 && !req.FreeSpin {
		winDec := decimal.NewFromFloat(result.TotalPayout)
		winTxID := fmt.Sprintf("slot-win-%s-%d", sess.UserID, time.Now().UnixNano())
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
	} else if result.TotalPayout > 0 && req.FreeSpin {
		// Free spin win: credit the full win
		winDec := decimal.NewFromFloat(result.TotalPayout)
		winTxID := fmt.Sprintf("slot-freewin-%s-%d", sess.UserID, time.Now().UnixNano())
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
		"spin_id":            roundID,
		"server_seed_hash":   result.ServerSeedHash,
		"nonce":              result.Nonce,
		"initial_grid":       gridToSlice(result.InitialGrid),
		"cascades":           cascadesToJSON(result.Cascades),
		"total_payout":       result.TotalPayout,
		"scatter_count":      result.ScatterCount,
		"free_spins_awarded": result.FreeSpinsAwarded,
		"balance":            balF,
		"bet":                req.Bet,
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

	c.JSON(http.StatusOK, gin.H{
		"balance":      bal,
		"currency":     sess.Currency,
		"min_bet":      0.10,
		"max_bet":      1000.0,
		"default_bet":  1.00,
		"grid_cols":    NumCols,
		"grid_rows":    NumRows,
		"cluster_min":  ClusterMin,
		"symbols":      symbolsInfo(),
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
