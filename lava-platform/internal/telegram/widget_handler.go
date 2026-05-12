package telegram

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lava-platform/internal/domain"
	"github.com/lava-platform/internal/session/service"
	"github.com/rs/zerolog/log"
	"github.com/shopspring/decimal"
)

const (
	widgetSignupBonus  = "1000.00"
	widgetAuthMaxAge   = 24 * time.Hour
)

// WidgetHandler handles Telegram Login Widget OAuth.
type WidgetHandler struct {
	botToken      string
	sessionSvc    service.SessionService
	walletSvc     domain.WalletProvider
	tmaOperatorID int64
	bot           *botClient
	db            testerDB
}

// testerDB is a minimal interface for the testers table operations.
// Implemented by *testerRepository below.
type testerDB interface {
	Upsert(telegramID int64, firstName, username string) error
	GetGameUsername(telegramID int64) (string, bool, error) // username, exists, err
	SetGameUsername(telegramID int64, gameUsername string) error
}

func NewWidgetHandler(
	botToken string,
	svc service.SessionService,
	wallet domain.WalletProvider,
	operatorID int64,
	bot *botClient,
	db testerDB,
) *WidgetHandler {
	return &WidgetHandler{
		botToken:      botToken,
		sessionSvc:    svc,
		walletSvc:     wallet,
		tmaOperatorID: operatorID,
		bot:           bot,
		db:            db,
	}
}

// WidgetAuthRequest is the JSON body sent by the frontend after the Login Widget callback.
type WidgetAuthRequest struct {
	ID        string `json:"id"         binding:"required"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
	PhotoURL  string `json:"photo_url"`
	AuthDate  string `json:"auth_date"  binding:"required"`
	Hash      string `json:"hash"       binding:"required"`
}

// WidgetAuthResponse is returned on successful widget auth.
type WidgetAuthResponse struct {
	Token        string `json:"token"`
	PlayerID     string `json:"player_id"`
	FirstName    string `json:"first_name"`
	Username     string `json:"username"`
	Balance      string `json:"balance"`
	NeedsUsername bool  `json:"needs_username"`
	GameUsername  string `json:"game_username,omitempty"`
}

// WidgetAuth handles POST /tma/widget-auth
// Verifies Telegram Login Widget data, creates session, sends welcome bot message.
func (h *WidgetHandler) WidgetAuth(c *gin.Context) {
	var req WidgetAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing required fields"})
		return
	}

	// Verify Telegram Login Widget hash
	user, err := h.validateWidget(req)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid telegram auth"})
		return
	}

	playerID := strconv.FormatInt(user.ID, 10)

	// Upsert tester record
	if dbErr := h.db.Upsert(user.ID, user.FirstName, user.Username); dbErr != nil {
		log.Warn().Err(dbErr).Str("player", playerID).Msg("widget auth: tester upsert")
	}

	// Create platform session
	sess, err := h.sessionSvc.Create(
		c.Request.Context(),
		h.tmaOperatorID,
		playerID,
		"USD",
		c.ClientIP(),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "session creation failed"})
		return
	}

	// Credit signup bonus (idempotent)
	walletUserID := fmt.Sprintf("op%d:%s", h.tmaOperatorID, playerID)
	bonus, _ := decimal.NewFromString(widgetSignupBonus)
	balance := ""

	creditResp, err := h.walletSvc.Credit(c.Request.Context(), &domain.CreditRequest{
		UserID:        walletUserID,
		Amount:        bonus,
		Currency:      "USD",
		TransactionID: fmt.Sprintf("widget_signup_%s", walletUserID),
	})
	if err != nil {
		// Already exists — fetch balance
		balResp, berr := h.walletSvc.GetBalance(c.Request.Context(), &domain.BalanceRequest{
			UserID: walletUserID, Currency: "USD",
		})
		if berr == nil {
			balance = balResp.Balance.StringFixed(2)
		}
	} else {
		balance = creditResp.Balance.StringFixed(2)
	}

	// Check if game_username is set
	gameUsername, _, _ := h.db.GetGameUsername(user.ID)
	needsUsername := gameUsername == ""

	// Send welcome bot message (fire-and-forget)
	go h.bot.SendMessage(user.ID, "✅ You successfully authorized in H-GAMES TEST ACCESS\n\nYour test balance: $"+balance+"\n\nUse the link to play: https://h-games.io")

	c.JSON(http.StatusOK, WidgetAuthResponse{
		Token:        sess.SessionToken,
		PlayerID:     playerID,
		FirstName:    user.FirstName,
		Username:     user.Username,
		Balance:      balance,
		NeedsUsername: needsUsername,
		GameUsername:  gameUsername,
	})
}

// SetGameUsernameRequest is the JSON body for setting a game username.
type SetGameUsernameRequest struct {
	PlayerID     string `json:"player_id"     binding:"required"`
	GameUsername string `json:"game_username"  binding:"required"`
}

// SetGameUsername handles POST /tma/set-username
func (h *WidgetHandler) SetGameUsername(c *gin.Context) {
	var req SetGameUsernameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing required fields"})
		return
	}

	gameUsername := strings.TrimSpace(req.GameUsername)
	if len(gameUsername) < 3 || len(gameUsername) > 20 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username must be 3-20 characters"})
		return
	}

	telegramID, err := strconv.ParseInt(req.PlayerID, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid player_id"})
		return
	}

	if err := h.db.SetGameUsername(telegramID, gameUsername); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save username"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"game_username": gameUsername})
}

// validateWidget verifies the Telegram Login Widget hash.
// Algorithm: secret_key = SHA256(bot_token); hash = HMAC-SHA256(data_check_string, secret_key)
func (h *WidgetHandler) validateWidget(req WidgetAuthRequest) (*User, error) {
	// Build field map for verification
	fields := url.Values{
		"id":         {req.ID},
		"first_name": {req.FirstName},
		"auth_date":  {req.AuthDate},
	}
	if req.LastName != "" {
		fields["last_name"] = []string{req.LastName}
	}
	if req.Username != "" {
		fields["username"] = []string{req.Username}
	}
	if req.PhotoURL != "" {
		fields["photo_url"] = []string{req.PhotoURL}
	}

	// Build sorted data_check_string
	pairs := make([]string, 0, len(fields))
	for k, vals := range fields {
		pairs = append(pairs, k+"="+vals[0])
	}
	sort.Strings(pairs)
	dcs := strings.Join(pairs, "\n")

	// secret_key = SHA256(bot_token)
	sum := sha256.Sum256([]byte(h.botToken))
	secretKey := sum[:]

	// expected hash = HMAC-SHA256(dcs, secretKey)
	mac := hmac.New(sha256.New, secretKey)
	mac.Write([]byte(dcs))
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(req.Hash), []byte(expected)) {
		return nil, ErrInvalidHash
	}

	// Check auth_date freshness
	authDate, err := strconv.ParseInt(req.AuthDate, 10, 64)
	if err != nil || time.Since(time.Unix(authDate, 0)) > widgetAuthMaxAge {
		return nil, ErrAuthExpired
	}

	id, _ := strconv.ParseInt(req.ID, 10, 64)
	return &User{
		ID:        id,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Username:  req.Username,
	}, nil
}
