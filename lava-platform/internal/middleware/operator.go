package middleware

import (
	"bytes"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lava-platform/internal/domain"
	"github.com/lava-platform/internal/operator/service"
)

const operatorCtxKey = "operator"

// OperatorAuth validates X-API-KEY + X-TIMESTAMP + X-SIGNATURE on every request.
//
// Security flow:
//  1. Extract X-API-KEY header → identify operator
//  2. Lookup operator in Redis cache (miss → PostgreSQL)
//  3. Verify HMAC-SHA256: Sign(secretKey, METHOD+PATH+TIMESTAMP+body_sha256)
//  4. Reject if |now - timestamp| > 5 minutes (replay attack prevention)
//  5. Inject *domain.Operator into request context
func OperatorAuth(svc service.OperatorService) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-KEY")
		sig := c.GetHeader("X-SIGNATURE")
		tsStr := c.GetHeader("X-TIMESTAMP")

		if apiKey == "" {
			abort(c, http.StatusUnauthorized, domain.ErrAPIKeyRequired)
			return
		}
		if sig == "" {
			abort(c, http.StatusUnauthorized, domain.ErrSignatureRequired)
			return
		}
		if tsStr == "" {
			abort(c, http.StatusUnauthorized, domain.ErrTimestampRequired)
			return
		}

		ts, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil {
			abort(c, http.StatusBadRequest, domain.ErrExpiredTimestamp)
			return
		}

		// Read body and restore it so downstream handlers can read it again.
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "cannot read body", "code": "BAD_REQUEST"})
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

		op, err := svc.Authenticate(
			c.Request.Context(),
			apiKey, sig, ts,
			c.Request.Method, c.Request.URL.Path,
			body,
		)
		if err != nil {
			status := errorStatus(err)
			abort(c, status, err)
			return
		}

		c.Set(operatorCtxKey, op)
		c.Next()
	}
}

// OperatorFromCtx returns the *domain.Operator injected by OperatorAuth.
func OperatorFromCtx(c *gin.Context) *domain.Operator {
	v, _ := c.Get(operatorCtxKey)
	op, _ := v.(*domain.Operator)
	return op
}

// AdminBasicAuth protects the admin dashboard with HTTP Basic Auth.
// username: "admin", password: systemKey.
func AdminBasicAuth(systemKey string) gin.HandlerFunc {
	return gin.BasicAuth(gin.Accounts{"admin": systemKey})
}

// SystemAuth protects admin endpoints with a static system API key.
func SystemAuth(systemKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if systemKey == "" {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error": "admin API not configured",
				"code":  "NOT_CONFIGURED",
			})
			return
		}
		if c.GetHeader("X-SYSTEM-KEY") != systemKey {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid system key",
				"code":  "UNAUTHORIZED",
			})
			return
		}
		c.Next()
	}
}

func abort(c *gin.Context, status int, err error) {
	c.AbortWithStatusJSON(status, gin.H{"error": err.Error(), "code": errorCode(err)})
}

func errorCode(err error) string {
	switch err {
	case domain.ErrOperatorNotFound:
		return "OPERATOR_NOT_FOUND"
	case domain.ErrOperatorSuspended:
		return "OPERATOR_SUSPENDED"
	case domain.ErrOperatorInactive:
		return "OPERATOR_INACTIVE"
	case domain.ErrInvalidSignature:
		return "INVALID_SIGNATURE"
	case domain.ErrExpiredTimestamp:
		return "EXPIRED_TIMESTAMP"
	default:
		return "UNAUTHORIZED"
	}
}

func errorStatus(err error) int {
	switch err {
	case domain.ErrOperatorSuspended, domain.ErrOperatorInactive:
		return http.StatusForbidden
	case domain.ErrInvalidSignature, domain.ErrExpiredTimestamp:
		return http.StatusUnauthorized
	case domain.ErrOperatorNotFound:
		return http.StatusUnauthorized // don't reveal existence
	default:
		return http.StatusInternalServerError
	}
}
