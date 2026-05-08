// botpoll — Telegram bot long-polling runner for local development.
// Use this instead of a webhook when you don't have a public HTTPS URL.
//
// Usage:
//   TELEGRAM_BOT_TOKEN=xxx TELEGRAM_APP_URL=https://... go run ./cmd/botpoll
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const apiBase = "https://api.telegram.org/bot"

func main() {
	_ = godotenv.Load()

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal().Msg("TELEGRAM_BOT_TOKEN is not set")
	}
	appURL := os.Getenv("TELEGRAM_APP_URL")

	// Remove any existing webhook so polling works
	if err := apiCall(token, "deleteWebhook", map[string]bool{"drop_pending_updates": false}); err != nil {
		log.Warn().Err(err).Msg("deleteWebhook failed (non-fatal)")
	} else {
		log.Info().Msg("webhook cleared — using long polling")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	log.Info().Str("app_url", appURL).Msg("bot polling started — send /start in Telegram")

	poll(ctx, token, appURL)

	log.Info().Msg("bot stopped")
}

// ── Polling loop ──────────────────────────────────────────────────────────────

func poll(ctx context.Context, token, appURL string) {
	offset := 0
	client := &http.Client{Timeout: 35 * time.Second}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		updates, err := getUpdates(client, token, offset)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Error().Err(err).Msg("getUpdates failed, retrying in 5s")
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
			continue
		}

		for _, u := range updates {
			offset = u.UpdateID + 1
			handleUpdate(token, appURL, u)
		}
	}
}

func getUpdates(client *http.Client, token string, offset int) ([]Update, error) {
	url := fmt.Sprintf("%s%s/getUpdates?timeout=30&offset=%d", apiBase, token, offset)
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		OK     bool     `json:"ok"`
		Result []Update `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if !result.OK {
		return nil, fmt.Errorf("getUpdates: not ok")
	}
	return result.Result, nil
}

// ── Update handling ───────────────────────────────────────────────────────────

type Update struct {
	UpdateID int      `json:"update_id"`
	Message  *Message `json:"message"`
}

type Message struct {
	Chat Chat    `json:"chat"`
	From *TGUser `json:"from"`
	Text string  `json:"text"`
}

type Chat struct {
	ID int64 `json:"id"`
}

type TGUser struct {
	FirstName string `json:"first_name"`
}

func handleUpdate(token, appURL string, u Update) {
	if u.Message == nil {
		return
	}
	text := strings.TrimSpace(u.Message.Text)
	chatID := u.Message.Chat.ID

	log.Info().Int64("chat", chatID).Str("text", text).Msg("message")

	switch {
	case strings.HasPrefix(text, "/start"), strings.HasPrefix(text, "/play"):
		sendStart(token, appURL, chatID, u.Message.From)
	case strings.HasPrefix(text, "/help"):
		sendHelp(token, chatID)
	default:
		sendMessage(token, chatID, "Tap the button to play 🎮 — or use /start", nil)
	}
}

func sendStart(token, appURL string, chatID int64, from *TGUser) {
	name := "there"
	if from != nil && from.FirstName != "" {
		name = from.FirstName
	}

	text := fmt.Sprintf("Hey %s! 🚀\n\nWelcome to *Lava Crash* — the provably fair crash game.\n\nTap below to play!", name)

	var keyboard interface{}
	if appURL != "" {
		keyboard = map[string]interface{}{
			"inline_keyboard": [][]map[string]interface{}{
				{{"text": "🎮 Play Now", "web_app": map[string]string{"url": appURL}}},
			},
		}
	} else {
		log.Warn().Msg("TELEGRAM_APP_URL not set — Play button will not appear")
		text += "\n\n_(Game URL not configured — set TELEGRAM\\_APP\\_URL)_"
	}

	sendMessage(token, chatID, text, keyboard)
}

func sendHelp(token string, chatID int64) {
	sendMessage(token, chatID,
		"*Lava Crash* — How to play:\n\n"+
			"1. Place a bet before the round starts\n"+
			"2. Watch the multiplier climb 🚀\n"+
			"3. Cash out before it crashes!\n\n/play — Launch the game",
		nil,
	)
}

func sendMessage(token string, chatID int64, text string, replyMarkup interface{}) {
	body := map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "Markdown",
	}
	if replyMarkup != nil {
		body["reply_markup"] = replyMarkup
	}
	if err := apiCall(token, "sendMessage", body); err != nil {
		log.Error().Err(err).Int64("chat_id", chatID).Msg("sendMessage failed")
	}
}

func apiCall(token, method string, body interface{}) error {
	data, _ := json.Marshal(body)
	resp, err := http.Post(apiBase+token+"/"+method, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var r map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&r)
		return fmt.Errorf("%s: %v", method, r)
	}
	return nil
}
