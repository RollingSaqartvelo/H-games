package admin

import (
	"context"
	_ "embed"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed dashboard.html
var dashboardHTML string

type Handler struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Handler {
	return &Handler{db: db}
}

// Dashboard GET /admin → HTML page
func (h *Handler) Dashboard(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, dashboardHTML)
}

// Stats GET /admin/v1/stats → JSON data for the dashboard
func (h *Handler) Stats(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	type RoundRow struct {
		ID         string     `json:"id"`
		State      string     `json:"state"`
		CrashPoint *float64   `json:"crash_point"`
		StartedAt  *time.Time `json:"started_at"`
		CrashedAt  *time.Time `json:"crashed_at"`
		CreatedAt  time.Time  `json:"created_at"`
	}

	roundRows, err := h.db.Query(ctx, `
		SELECT id, state, crash_point, started_at, crashed_at, created_at
		FROM rounds ORDER BY created_at DESC LIMIT 20
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer roundRows.Close()

	var rounds []RoundRow
	for roundRows.Next() {
		var r RoundRow
		if err := roundRows.Scan(&r.ID, &r.State, &r.CrashPoint, &r.StartedAt, &r.CrashedAt, &r.CreatedAt); err != nil {
			continue
		}
		rounds = append(rounds, r)
	}

	type BetRow struct {
		ID         string    `json:"id"`
		PlayerID   string    `json:"player_id"`
		BetAmount  float64   `json:"bet_amount"`
		Currency   string    `json:"currency"`
		Status     string    `json:"status"`
		CashoutAt  *float64  `json:"cashout_at"`
		Payout     float64   `json:"payout_amount"`
		CrashPoint *float64  `json:"crash_point"`
		RoundState string    `json:"round_state"`
		CreatedAt  time.Time `json:"created_at"`
	}

	betRows, err := h.db.Query(ctx, `
		SELECT rb.id, rb.player_id, rb.bet_amount::float8, rb.currency,
		       rb.status, rb.cashout_at, rb.payout_amount::float8,
		       r.crash_point, r.state, rb.created_at
		FROM round_bets rb
		JOIN rounds r ON r.id = rb.round_id
		ORDER BY rb.created_at DESC LIMIT 50
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer betRows.Close()

	var bets []BetRow
	for betRows.Next() {
		var b BetRow
		if err := betRows.Scan(
			&b.ID, &b.PlayerID, &b.BetAmount, &b.Currency,
			&b.Status, &b.CashoutAt, &b.Payout,
			&b.CrashPoint, &b.RoundState, &b.CreatedAt,
		); err != nil {
			continue
		}
		bets = append(bets, b)
	}

	type PlayerRow struct {
		PlayerID    string    `json:"player_id"`
		TotalBets   int       `json:"total_bets"`
		TotalWagered float64  `json:"total_wagered"`
		TotalWon    float64   `json:"total_won"`
		HouseProfit float64   `json:"house_profit"`
		Wins        int       `json:"wins"`
		Losses      int       `json:"losses"`
		LastBet     time.Time `json:"last_bet"`
	}

	playerRows, err := h.db.Query(ctx, `
		SELECT player_id,
		       COUNT(*)::int,
		       SUM(bet_amount)::float8,
		       COALESCE(SUM(payout_amount),0)::float8,
		       (SUM(bet_amount) - COALESCE(SUM(payout_amount),0))::float8,
		       COUNT(*) FILTER (WHERE status = 'CASHED_OUT')::int,
		       COUNT(*) FILTER (WHERE status = 'LOST')::int,
		       MAX(created_at)
		FROM round_bets
		GROUP BY player_id
		ORDER BY SUM(bet_amount) DESC
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer playerRows.Close()

	var players []PlayerRow
	for playerRows.Next() {
		var p PlayerRow
		if err := playerRows.Scan(
			&p.PlayerID, &p.TotalBets, &p.TotalWagered,
			&p.TotalWon, &p.HouseProfit, &p.Wins, &p.Losses, &p.LastBet,
		); err != nil {
			continue
		}
		players = append(players, p)
	}

	var totalWagered, totalWon float64
	var totalBets int
	_ = h.db.QueryRow(ctx, `
		SELECT COUNT(*)::int, COALESCE(SUM(bet_amount),0)::float8, COALESCE(SUM(payout_amount),0)::float8
		FROM round_bets
	`).Scan(&totalBets, &totalWagered, &totalWon)

	c.JSON(http.StatusOK, gin.H{
		"rounds":  rounds,
		"bets":    bets,
		"players": players,
		"totals": gin.H{
			"total_bets":    totalBets,
			"total_wagered": totalWagered,
			"total_won":     totalWon,
			"house_profit":  totalWagered - totalWon,
		},
		"generated_at": time.Now(),
	})
}

// Deploy POST /admin/v1/deploy → git pull + restart server
func (h *Handler) Deploy(c *gin.Context) {
	const repoDir = "/root/lava-platform"
	const binSrc  = "/root/lava-platform/lava-platform/server_linux"
	const binDst  = "/root/lava-platform/lava-platform/server"
	const logFile = "/root/server.log"

	steps := [][]string{
		{"git", "-C", repoDir, "fetch", "origin"},
		{"git", "-C", repoDir, "reset", "--hard", "origin/master"},
		{"cp", binSrc, binDst},
	}
	var log string
	for _, args := range steps {
		out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
		log += fmt.Sprintf("$ %v\n%s\n", args, out)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "log": log})
			return
		}
	}

	// Write restart script and launch it detached — returns before pkill fires
	script := "#!/bin/sh\nsleep 1\npkill -9 -f 'lava-platform/server' || true\nsleep 1\nnohup " + binDst + " >> " + logFile + " 2>&1 &\n"
	_ = os.WriteFile("/tmp/lava_restart.sh", []byte(script), 0755)
	if err := exec.Command("nohup", "sh", "/tmp/lava_restart.sh").Start(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "restart failed: " + err.Error(), "log": log})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "Deploy started — server will restart in ~2s", "log": log})
}

// CreditAll POST /admin/v1/credit-all → add balance to all wallets
func (h *Handler) CreditAll(c *gin.Context) {
	var body struct {
		Amount float64 `json:"amount"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Amount <= 0 {
		body.Amount = 5000
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	tag, err := h.db.Exec(ctx,
		`UPDATE wallets SET balance = balance + $1, updated_at = NOW()`,
		body.Amount,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"updated_wallets": tag.RowsAffected(), "credited": body.Amount})
}
