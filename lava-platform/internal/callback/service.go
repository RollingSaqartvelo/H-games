package callback

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/lava-platform/internal/signing"
	"github.com/rs/zerolog/log"
	"github.com/shopspring/decimal"
)

type Event struct {
	OperatorCallbackURL string
	OperatorSecretKey   string
	EventType           string          // "bet" | "win" | "rollback"
	TransactionID       string
	PlayerID            string
	RoundID             string
	Amount              decimal.Decimal
	Currency            string
	Balance             decimal.Decimal
}

type Service interface {
	// Send dispatches a signed callback asynchronously (fire-and-forget).
	Send(event *Event)
}

type httpService struct {
	client  *http.Client
	retries int
}

func NewService(timeout time.Duration, retries int) Service {
	return &httpService{
		client:  &http.Client{Timeout: timeout},
		retries: retries,
	}
}

func (s *httpService) Send(event *Event) {
	if event.OperatorCallbackURL == "" {
		return
	}
	go s.dispatch(event)
}

func (s *httpService) dispatch(event *Event) {
	payload := buildPayload(event)
	backoff := time.Second

	for attempt := 1; attempt <= s.retries; attempt++ {
		err := s.post(event, payload)
		if err == nil {
			log.Debug().
				Str("event", event.EventType).
				Str("tx_id", event.TransactionID).
				Str("url", event.OperatorCallbackURL).
				Int("attempt", attempt).
				Msg("callback delivered")
			return
		}

		log.Warn().
			Err(err).
			Str("event", event.EventType).
			Str("tx_id", event.TransactionID).
			Int("attempt", attempt).
			Dur("backoff", backoff).
			Msg("callback failed, retrying")

		if attempt < s.retries {
			time.Sleep(backoff)
			backoff *= 2
		}
	}

	log.Error().
		Str("event", event.EventType).
		Str("tx_id", event.TransactionID).
		Str("url", event.OperatorCallbackURL).
		Msg("callback exhausted all retries")
}

func (s *httpService) post(event *Event, body []byte) error {
	ts := time.Now().Unix()
	sig := signing.Sign(event.OperatorSecretKey, http.MethodPost, "/callback", ts, body)

	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodPost, event.OperatorCallbackURL, bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-TIMESTAMP", strconv.FormatInt(ts, 10))
	req.Header.Set("X-SIGNATURE", sig)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("operator returned %d", resp.StatusCode)
	}
	return nil
}

func buildPayload(e *Event) []byte {
	return []byte(fmt.Sprintf(
		`{"event_type":%q,"transaction_id":%q,"player_id":%q,"round_id":%q,"amount":%q,"currency":%q,"balance":%q,"timestamp":%d}`,
		e.EventType, e.TransactionID, e.PlayerID, e.RoundID,
		e.Amount.StringFixed(2), e.Currency, e.Balance.StringFixed(2),
		time.Now().Unix(),
	))
}
