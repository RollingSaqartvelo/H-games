package admin

import (
	"context"
	_ "embed"
	"net/http"
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
		       COUNT(*) FILTER (WHERE status = 'WON')::int,
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
