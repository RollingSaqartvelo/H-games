package pirate

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lava-platform/internal/domain"
	"github.com/lava-platform/internal/middleware"
	walletSvc "github.com/lava-platform/internal/wallet/service"
	"github.com/shopspring/decimal"
)

// freeSpinSession tracks persistent free-spin state per player.
type freeSpinSession struct {
	SpinsLeft  int
	TriggerBet float64
}

// Handler handles Pirate Treasure Hold HTTP requests.
type Handler struct {
	wallet *walletSvc.InternalWalletProvider
	cfg    *Config

	mu     sync.Mutex
	nonces map[string]int64 // userID → next nonce

	freeSessions sync.Map // userID → *freeSpinSession
	holdWinSess  sync.Map // userID → *HoldWinState
}

func NewHandler(wallet *walletSvc.InternalWalletProvider, cfg *Config) *Handler {
	return &Handler{
		wallet: wallet,
		cfg:    cfg,
		nonces: make(map[string]int64),
	}
}

func (h *Handler) nextNonce(userID string) int64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.nonces[userID]++
	return h.nonces[userID]
}

func pirateRandSeed() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (h *Handler) getFreeSession(userID string) *freeSpinSession {
	val, ok := h.freeSessions.Load(userID)
	if !ok {
		return nil
	}
	return val.(*freeSpinSession)
}

func (h *Handler) setFreeSession(userID string, fs *freeSpinSession) {
	if fs == nil || fs.SpinsLeft <= 0 {
		h.freeSessions.Delete(userID)
	} else {
		h.freeSessions.Store(userID, fs)
	}
}

func (h *Handler) getHoldWin(userID string) *HoldWinState {
	val, ok := h.holdWinSess.Load(userID)
	if !ok {
		return nil
	}
	return val.(*HoldWinState)
}

func (h *Handler) setHoldWin(userID string, state *HoldWinState) {
	if state == nil || state.Complete {
		h.holdWinSess.Delete(userID)
	} else {
		h.holdWinSess.Store(userID, state)
	}
}

// SpinRequest is the POST /spin body.
type SpinRequest struct {
	Bet        float64 `json:"bet"        binding:"required,min=0.25,max=30.00"`
	FreeSpin   bool    `json:"free_spin"`
	HoldRespin bool    `json:"hold_respin"`
	BonusBuy   bool    `json:"bonus_buy"`
}

// POST /tma/v1/pirate/spin
func (h *Handler) Spin(c *gin.Context) {
	sess := middleware.SessionFromCtx(c)
	if sess == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req SpinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := sess.UserID

	// ── 1. HoldRespin: continue existing HoldWinState, no wallet debit ──────────
	if req.HoldRespin {
		hwState := h.getHoldWin(userID)
		if hwState == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "no active hold&win session"})
			return
		}

		// Get current balance
		balResp, err := h.wallet.GetBalance(c.Request.Context(), &domain.BalanceRequest{
			UserID:   userID,
			Currency: sess.Currency,
		})
		bal := 0.0
		if err == nil {
			bal, _ = balResp.Balance.Float64()
		}

		seed := pirateRandSeed()
		nonce := h.nextNonce(userID)
		r := newRNG(seed, nonce)
		hwState = runHoldWinRespin(hwState, r)

		var finalBalance decimal.Decimal
		if balResp != nil {
			finalBalance = balResp.Balance
		}

		if hwState.Complete {
			// Credit total value
			winDec := decimal.NewFromFloat(hwState.TotalValue)
			roundID := fmt.Sprintf("pirate-hold-%s-%d", userID, time.Now().UnixNano())
			winTxID := fmt.Sprintf("pirate-win-%s-%d", userID, time.Now().UnixNano())
			creditResp, err := h.wallet.Credit(c.Request.Context(), &domain.CreditRequest{
				UserID:        userID,
				Amount:        winDec,
				Currency:      sess.Currency,
				TransactionID: winTxID,
				RoundID:       roundID,
			})
			if err == nil {
				finalBalance = creditResp.Balance
				bal, _ = finalBalance.Float64()
			}
			h.setHoldWin(userID, nil)
		} else {
			h.setHoldWin(userID, hwState)
		}

		c.JSON(http.StatusOK, gin.H{
			"server_seed_hash":   HashSeed(seed),
			"nonce":              nonce,
			"hold_win_result":    hwState,
			"hold_win_triggered": true,
			"balance":            bal,
			"bet":                hwState.TriggerBet,
			"free_spins_left":    h.freeSpinsLeft(userID),
		})
		return
	}

	// ── 2. FreeSpin: use freeSpinSession, no wallet debit ────────────────────────
	isFree := false
	freeSess := h.getFreeSession(userID)
	if req.FreeSpin && freeSess != nil && freeSess.SpinsLeft > 0 {
		isFree = true
	}

	// ── 3. BonusBuy: debit Bet×65 ────────────────────────────────────────────────
	effectiveBet := req.Bet
	isBonusBuy := req.BonusBuy && !isFree

	betDec := decimal.NewFromFloat(effectiveBet)
	txLabel := "pirate-bet"
	if isBonusBuy {
		betDec = decimal.NewFromFloat(req.Bet * h.cfg.BonusBuyCost)
		txLabel = "pirate-bonus"
	}

	betTxID := fmt.Sprintf("%s-%s-%d", txLabel, userID, time.Now().UnixNano())
	roundID := betTxID

	var balanceAfterBet decimal.Decimal

	if isFree {
		// No debit — just read balance
		balResp, err := h.wallet.GetBalance(c.Request.Context(), &domain.BalanceRequest{
			UserID:   userID,
			Currency: sess.Currency,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "wallet error"})
			return
		}
		balanceAfterBet = balResp.Balance
		freeSess.SpinsLeft--
		h.setFreeSession(userID, freeSess)
	} else {
		// Debit wallet
		debitResp, err := h.wallet.Debit(c.Request.Context(), &domain.DebitRequest{
			UserID:        userID,
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
	}

	// ── Run spin ──────────────────────────────────────────────────────────────
	seed := pirateRandSeed()
	nonce := h.nextNonce(userID)

	result := Spin(h.cfg, seed, nonce, req.Bet, isFree, isBonusBuy)
	result.SpinID = roundID

	// ── Award new free spins (scatter trigger) ────────────────────────────────
	if result.FreeSpinsAwarded > 0 {
		existing := h.getFreeSession(userID)
		newFS := &freeSpinSession{
			SpinsLeft:  result.FreeSpinsAwarded,
			TriggerBet: req.Bet,
		}
		if existing != nil {
			newFS.SpinsLeft += existing.SpinsLeft
		}
		h.setFreeSession(userID, newFS)
	}

	// ── Handle Hold&Win trigger ───────────────────────────────────────────────
	if result.HoldWinTriggered && result.HoldWinResult != nil {
		hwState := result.HoldWinResult
		if hwState.Complete {
			// One-shot (all 15 cells filled immediately or similar)
			winDec := decimal.NewFromFloat(hwState.TotalValue)
			winTxID := fmt.Sprintf("pirate-win-%s-%d", userID, time.Now().UnixNano())
			h.wallet.Credit(c.Request.Context(), &domain.CreditRequest{
				UserID:        userID,
				Amount:        winDec,
				Currency:      sess.Currency,
				TransactionID: winTxID,
				RoundID:       roundID,
			})
			// result.TotalPayout already includes hwState.TotalValue
		} else {
			// Save for respins
			h.setHoldWin(userID, hwState)
			result.TotalPayout = 0 // will be credited at end of respins
		}
	}

	// ── Credit normal payline wins ────────────────────────────────────────────
	finalBalance := balanceAfterBet
	if result.TotalPayout > 0 {
		winDec := decimal.NewFromFloat(result.TotalPayout)
		txSuffix := "win"
		if isFree {
			txSuffix = "freewin"
		}
		winTxID := fmt.Sprintf("pirate-%s-%s-%d", txSuffix, userID, time.Now().UnixNano())
		creditResp, err := h.wallet.Credit(c.Request.Context(), &domain.CreditRequest{
			UserID:        userID,
			Amount:        winDec,
			Currency:      sess.Currency,
			TransactionID: winTxID,
			RoundID:       roundID,
		})
		if err == nil {
			finalBalance = creditResp.Balance
		}
	}

	balF, _ := finalBalance.Float64()
	result.Balance = balF
	result.FreeSpinsLeft = h.freeSpinsLeft(userID)

	// Convert grid to slice for JSON
	gridSlice := make([][]int, NumRows)
	for row := 0; row < NumRows; row++ {
		gridSlice[row] = make([]int, NumCols)
		for col := 0; col < NumCols; col++ {
			gridSlice[row][col] = result.Grid[row][col]
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"spin_id":            result.SpinID,
		"server_seed_hash":   result.ServerSeedHash,
		"nonce":              result.Nonce,
		"grid":               gridSlice,
		"paylines":           result.Paylines,
		"total_payout":       result.TotalPayout,
		"scatter_count":      result.ScatterCount,
		"free_spins_awarded": result.FreeSpinsAwarded,
		"free_spins_left":    result.FreeSpinsLeft,
		"hold_win_triggered": result.HoldWinTriggered,
		"hold_win_result":    result.HoldWinResult,
		"balance":            balF,
		"bet":                req.Bet,
	})
}

// GET /tma/v1/pirate/config
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

	freeLeft := h.freeSpinsLeft(sess.UserID)
	hwState := h.getHoldWin(sess.UserID)

	c.JSON(http.StatusOK, gin.H{
		"balance":          bal,
		"currency":         sess.Currency,
		"min_bet":          0.25,
		"max_bet":          30.00,
		"default_bet":      1.00,
		"grid_cols":        NumCols,
		"grid_rows":        NumRows,
		"num_paylines":     25,
		"free_spins_left":  freeLeft,
		"hold_win_active":  hwState != nil,
		"hold_win_state":   hwState,
		"bonus_buy_cost":   h.cfg.BonusBuyCost,
	})
}

func (h *Handler) freeSpinsLeft(userID string) int {
	fs := h.getFreeSession(userID)
	if fs == nil {
		return 0
	}
	return fs.SpinsLeft
}
