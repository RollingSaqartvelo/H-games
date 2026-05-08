package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lava-platform/internal/session/service"
)

const sessionCtxKey = "wallet_session"

// Session carries player identity resolved from a validated OperatorSession.
type Session struct {
	UserID     string // "op{id}:{player_id}" — wallet data isolation key
	PlayerID   string // raw player_id as supplied by operator
	Currency   string
	OperatorID int64
	Token      string
}

// SessionValidate resolves the Bearer token via SessionService (Redis → PostgreSQL),
// validates expiry and active status, then injects *Session into context.
func SessionValidate(svc service.SessionService) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractToken(c)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "missing authorization",
				"code":  "UNAUTHORIZED",
			})
			return
		}

		operatorSess, err := svc.Validate(c.Request.Context(), token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": err.Error(),
				"code":  "INVALID_SESSION",
			})
			return
		}

		c.Set(sessionCtxKey, &Session{
			UserID:     operatorSess.WalletUserID(),
			PlayerID:   operatorSess.PlayerID,
			Currency:   operatorSess.Currency,
			OperatorID: operatorSess.OperatorID,
			Token:      token,
		})
		c.Next()
	}
}

// SessionFromCtx returns the *Session injected by SessionValidate.
func SessionFromCtx(c *gin.Context) *Session {
	v, _ := c.Get(sessionCtxKey)
	sess, _ := v.(*Session)
	return sess
}

func extractToken(c *gin.Context) string {
	if auth := c.GetHeader("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return c.GetHeader("X-Session-Token")
}
