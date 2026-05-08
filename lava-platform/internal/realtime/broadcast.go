// Package realtime handles WebSocket broadcasting via Redis pub/sub.
// Using Redis as the message bus allows horizontal scaling: any server
// instance publishes to Redis, all instances receive and forward to clients.
package realtime

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

const broadcastChannel = "round:broadcast"

// MsgType identifies WebSocket message categories.
type MsgType string

const (
	MsgTypeState      MsgType = "state"       // full round state (sent on connect + transitions)
	MsgTypeTick       MsgType = "tick"        // multiplier update (every 100ms)
	MsgTypeCrashed    MsgType = "crashed"     // round ended — reveals server_seed
	MsgTypeBetPlaced  MsgType = "bet_placed"  // someone placed a bet
	MsgTypeCashout    MsgType = "cashout"     // someone cashed out
	MsgTypeError      MsgType = "error"
)

// Msg is the envelope for all WebSocket messages.
type Msg struct {
	Type MsgType `json:"type"`
	Data any     `json:"data"`
}

// Publisher publishes messages to Redis and provides a subscriber for the Hub.
type Publisher struct {
	client *redis.Client
}

func NewPublisher(client *redis.Client) *Publisher {
	return &Publisher{client: client}
}

// Publish serializes msg and pushes it to the Redis broadcast channel.
func (p *Publisher) Publish(ctx context.Context, msgType MsgType, data any) error {
	b, err := json.Marshal(Msg{Type: msgType, Data: data})
	if err != nil {
		return fmt.Errorf("marshal msg: %w", err)
	}
	return p.client.Publish(ctx, broadcastChannel, b).Err()
}

// Subscribe reads from Redis pub/sub and forwards raw bytes to the Hub.
// Runs until ctx is cancelled.
func (p *Publisher) Subscribe(ctx context.Context, hub *Hub) {
	sub := p.client.Subscribe(ctx, broadcastChannel)
	defer sub.Close()

	ch := sub.Channel()
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			hub.Broadcast([]byte(msg.Payload))
		case <-ctx.Done():
			return
		}
	}
}

// ─── State snapshots (sent to new clients on connect) ─────────────────────────

type TickData struct {
	RoundID    string  `json:"round_id"`
	Multiplier float64 `json:"multiplier"`
	ElapsedMs  int64   `json:"elapsed_ms"`
}

type StateData struct {
	ID             string  `json:"id"`
	State          string  `json:"state"`
	ServerSeedHash string  `json:"server_seed_hash"`
	ClientSeed     string  `json:"client_seed"`
	RTPProfile     int     `json:"rtp_profile"`
	StartedAt      *int64  `json:"started_at,omitempty"` // unix timestamp
}

type CrashedData struct {
	RoundID    string  `json:"round_id"`
	CrashPoint float64 `json:"crash_point"`
	ServerSeed string  `json:"server_seed"`
	ClientSeed string  `json:"client_seed"`
	Nonce      int64   `json:"nonce"`
}

type BetPlacedData struct {
	RoundID  string `json:"round_id"`
	PlayerID string `json:"player_id"`
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

type CashoutData struct {
	RoundID    string  `json:"round_id"`
	PlayerID   string  `json:"player_id"`
	Multiplier float64 `json:"multiplier"`
	Payout     string  `json:"payout"`
	Currency   string  `json:"currency"`
}

// LogPublishErr logs if publish fails (non-fatal — WebSocket is best-effort).
func LogPublishErr(err error, msgType MsgType) {
	if err != nil {
		log.Warn().Err(err).Str("type", string(msgType)).Msg("broadcast publish failed")
	}
}
