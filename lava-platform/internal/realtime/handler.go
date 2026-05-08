package realtime

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow all origins in development; restrict in production via config.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// WSHandler handles WebSocket upgrade requests for real-time round streaming.
type WSHandler struct {
	hub *Hub
}

func NewWSHandler(hub *Hub) *WSHandler {
	return &WSHandler{hub: hub}
}

// ServeWS upgrades an HTTP connection to WebSocket and starts serving the client.
// Token-based auth can be added here via the "token" query param in future.
func (h *WSHandler) ServeWS(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Warn().Err(err).Str("remote", c.Request.RemoteAddr).Msg("ws upgrade failed")
		return
	}

	client := NewClient(h.hub, conn)
	// Player ID can be extracted from a validated session token in the future;
	// for now all connections are anonymous spectators.
	client.Serve()
}
