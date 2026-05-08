package realtime

import (
	"context"
	"sync"

	"github.com/rs/zerolog/log"
)

// Hub maintains the set of active WebSocket clients and broadcasts messages.
// It is the single point of contact between the round engine and all players.
type Hub struct {
	mu         sync.RWMutex
	clients    map[*Client]struct{}
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte

	// lastState holds the most recent "state" message for new client reconnects.
	lastState []byte
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]struct{}),
		register:   make(chan *Client, 64),
		unregister: make(chan *Client, 64),
		broadcast:  make(chan []byte, 512),
	}
}

// Run processes registration and broadcast events until ctx is cancelled.
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			h.clients[c] = struct{}{}
			// Send current round state immediately so reconnecting clients
			// don't have to wait for the next tick.
			if h.lastState != nil {
				select {
				case c.send <- h.lastState:
				default:
				}
			}
			h.mu.Unlock()
			log.Debug().Str("remote", c.remoteAddr).Msg("ws client connected")

		case c := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.send)
			}
			h.mu.Unlock()
			log.Debug().Str("remote", c.remoteAddr).Msg("ws client disconnected")

		case msg := <-h.broadcast:
			h.mu.Lock()
			for c := range h.clients {
				select {
				case c.send <- msg:
				default:
					// Slow client: close and remove.
					close(c.send)
					delete(h.clients, c)
				}
			}
			h.mu.Unlock()

		case <-ctx.Done():
			return
		}
	}
}

// Broadcast enqueues a message to all connected clients.
func (h *Hub) Broadcast(msg []byte) {
	h.broadcast <- msg
}

// SetLastState stores the latest round state message for reconnect delivery.
func (h *Hub) SetLastState(msg []byte) {
	h.mu.Lock()
	h.lastState = msg
	h.mu.Unlock()
}

// ConnectedCount returns the number of connected WebSocket clients.
func (h *Hub) ConnectedCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
