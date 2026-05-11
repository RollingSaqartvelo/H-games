package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

const telegramAPI = "https://api.telegram.org/bot"

// botClient makes calls to the Telegram Bot API.
type botClient struct {
	token      string
	appURL     string
	httpClient *http.Client
}

// BotHandler handles incoming Telegram webhook updates and manages webhook registration.
type BotHandler struct {
	bot *botClient
}

func NewBotHandler(botToken, appURL string) *BotHandler {
	return &BotHandler{
		bot: &botClient{
			token:      botToken,
			appURL:     appURL,
			httpClient: &http.Client{Timeout: 10 * time.Second},
		},
	}
}

// ── Telegram update types ─────────────────────────────────────────────────────

type Update struct {
	UpdateID int      `json:"update_id"`
	Message  *Message `json:"message"`
}

type Message struct {
	MessageID int    `json:"message_id"`
	From      *User  `json:"from"`
	Chat      Chat   `json:"chat"`
	Text      string `json:"text"`
}

type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

// ── Webhook endpoint ──────────────────────────────────────────────────────────

// Webhook receives POST updates from Telegram.
// POST /tma/webhook
func (h *BotHandler) Webhook(c *gin.Context) {
	var update Update
	if err := c.ShouldBindJSON(&update); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	// Acknowledge immediately — process async so Telegram doesn't retry
	c.Status(http.StatusOK)

	go h.processUpdate(update)
}

func (h *BotHandler) processUpdate(update Update) {
	if update.Message == nil {
		return
	}
	msg := update.Message
	text := strings.TrimSpace(msg.Text)

	switch {
	case strings.HasPrefix(text, "/start"):
		h.bot.sendStart(msg.Chat.ID, msg.From)
	case strings.HasPrefix(text, "/play"):
		h.bot.sendStart(msg.Chat.ID, msg.From)
	case strings.HasPrefix(text, "/help"):
		h.bot.sendHelp(msg.Chat.ID)
	}
}

// ── Bot API calls ─────────────────────────────────────────────────────────────

func (b *botClient) sendStart(chatID int64, from *User) {
	name := "partner"
	if from != nil && from.FirstName != "" {
		name = from.FirstName
	}

	caption := fmt.Sprintf(
		"🤠 Welcome to H\\-GAMES Provider, %s\\!\n\n"+
			"Step into a world where chaos pays, timing is everything, and every escape could turn into fortune\\.\n\n"+
			"🎮 *AVAILABLE GAMES:*\n\n"+
			"🏜️ *OUTLAW ESCAPE:*\n"+
			"Ride hard\\. Leap platform to platform\\. Stay ahead of the sheriff\\. Cash out before you get WASTED\\.\n\n"+
			"👵 *GRANNY RUN:*\n"+
			"Break out\\. Launch skyward on jet\\-powered walkers\\. Escape the doctor\\. Fly fast, cash out, and don't get caught\\.\n\n"+
			"💰 *FEATURES:*\n"+
			"• Provably Fair gameplay\n"+
			"• Real\\-time multipliers\n"+
			"• Instant Cashout\n"+
			"• Auto Cashout\n"+
			"• Premium themed action\n"+
			"• Unique game worlds\n"+
			"• H\\-GAMES provider system\n\n"+
			"🎯 *YOUR MISSION:*\n"+
			"Choose your escape\\.\n"+
			"Survive the chase\\.\n"+
			"Beat the crash\\.\n"+
			"Cash out at the perfect moment\\.\n\n"+
			"🔥 Two worlds\\. Two escapes\\. One goal: WIN BIG\\.",
		name,
	)

	var keyboard interface{}
	if b.appURL != "" {
		keyboard = map[string]interface{}{
			"inline_keyboard": [][]map[string]interface{}{
				{
					{
						"text":    "🎮 Play H-Games",
						"web_app": map[string]string{"url": strings.TrimRight(b.appURL, "/") + "/landing.html"},
					},
				},
			},
		}
	}

	// Send logo image + caption in one message; fall back to text-only if no URL
	if b.appURL != "" {
		photoURL := strings.TrimRight(b.appURL, "/") + "/assets/hgames-logo.jpg"
		if err := b.sendPhoto(chatID, photoURL, caption, keyboard); err != nil {
			// Photo failed — send text-only so the user still gets the message
			b.sendMessage(chatID, caption, keyboard)
		}
	} else {
		b.sendMessage(chatID, caption, keyboard)
	}
}

func (b *botClient) sendHelp(chatID int64) {
	b.sendMessage(chatID,
		"🤠 *OUTLAW ESCAPE* — How to play:\n\n"+
			"1\\. Place your bet before the heist starts\n"+
			"2\\. Watch the multiplier climb as your outlaw rides 🏇\n"+
			"3\\. Cash out before the sheriff catches you\\!\n\n"+
			"The longer you ride, the bigger the payout — but wait too long and you get WASTED\\.\n\n"+
			"/play — Launch the game",
		nil,
	)
}

func (b *botClient) sendPhoto(chatID int64, photoURL, caption string, replyMarkup interface{}) error {
	body := map[string]interface{}{
		"chat_id":    chatID,
		"photo":      photoURL,
		"caption":    caption,
		"parse_mode": "MarkdownV2",
	}
	if replyMarkup != nil {
		body["reply_markup"] = replyMarkup
	}
	return b.apiCall("sendPhoto", body)
}

func (b *botClient) sendMessage(chatID int64, text string, replyMarkup interface{}) {
	body := map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "MarkdownV2",
	}
	if replyMarkup != nil {
		body["reply_markup"] = replyMarkup
	}

	if err := b.apiCall("sendMessage", body); err != nil {
		log.Error().Err(err).Int64("chat_id", chatID).Msg("telegram: sendMessage failed")
	}
}

func (b *botClient) apiCall(method string, body interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}

	url := telegramAPI + b.token + "/" + method
	resp, err := b.httpClient.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var result map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&result)
		return fmt.Errorf("telegram API %s: status %d: %v", method, resp.StatusCode, result)
	}
	return nil
}

// ── Webhook registration ──────────────────────────────────────────────────────

// RegisterWebhook tells Telegram to send updates to webhookURL.
// Called once at startup when TELEGRAM_APP_URL is set.
func (b *botClient) RegisterWebhook(ctx context.Context, webhookURL string) error {
	if webhookURL == "" {
		return nil
	}
	full := strings.TrimRight(webhookURL, "/") + "/tma/webhook"
	log.Info().Str("url", full).Msg("registering telegram webhook")
	return b.apiCall("setWebhook", map[string]string{"url": full})
}

// setMenuButton updates the bot's default menu button for all users to a Web App
// pointing at appURL. This auto-fixes the menu button whenever the tunnel URL changes.
func (b *botClient) setMenuButton(appURL string) error {
	if appURL == "" {
		return nil
	}
	url := strings.TrimRight(appURL, "/") + "/landing.html"
	log.Info().Str("url", url).Msg("setting telegram menu button")
	return b.apiCall("setChatMenuButton", map[string]interface{}{
		"menu_button": map[string]interface{}{
			"type":    "web_app",
			"text":    "Play H-Games",
			"web_app": map[string]string{"url": url},
		},
	})
}

// RegisterWebhook is the exported entry point on BotHandler.
func (h *BotHandler) RegisterWebhook(ctx context.Context, appURL string) error {
	if err := h.bot.RegisterWebhook(ctx, appURL); err != nil {
		return err
	}
	// Also update the menu button so it always points at the current tunnel URL.
	if err := h.bot.setMenuButton(appURL); err != nil {
		log.Warn().Err(err).Msg("telegram: setMenuButton failed")
	}
	return nil
}
