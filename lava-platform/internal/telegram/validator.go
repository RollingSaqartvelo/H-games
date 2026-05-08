// Package telegram validates Telegram Mini App initData and provides the
// /tma/auth HTTP endpoint that exchanges it for a platform session token.
//
// Validation algorithm (per Telegram docs):
//   data_check_string = sorted "key=value" pairs (excl. "hash"), joined by \n
//   secret_key        = HMAC-SHA256(key="WebAppData", data=bot_token)
//   expected_hash     = hex(HMAC-SHA256(key=secret_key, data=data_check_string))
//   valid             = expected_hash == hash && Now()-auth_date <= maxAge
package telegram

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ErrInvalidHash is returned when the HMAC check fails.
var ErrInvalidHash = errors.New("telegram: invalid initData hash")

// ErrAuthExpired is returned when auth_date is older than the configured max age.
var ErrAuthExpired = errors.New("telegram: initData auth_date expired")

// User is the Telegram user object embedded in initData.
type User struct {
	ID           int64  `json:"id"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Username     string `json:"username"`
	LanguageCode string `json:"language_code"`
	IsPremium    bool   `json:"is_premium"`
}

// Validator validates Telegram Mini App initData strings.
type Validator struct {
	secretKey []byte        // derived once from the bot token
	maxAge    time.Duration // max allowed age of auth_date
}

// NewValidator creates a Validator for the given bot token.
func NewValidator(botToken string, maxAge time.Duration) *Validator {
	// secret_key = HMAC-SHA256("WebAppData", botToken)
	mac := hmac.New(sha256.New, []byte("WebAppData"))
	mac.Write([]byte(botToken))
	return &Validator{
		secretKey: mac.Sum(nil),
		maxAge:    maxAge,
	}
}

// Validate parses and validates a raw initData query string.
// Returns the authenticated Telegram User on success.
func (v *Validator) Validate(initData string) (*User, error) {
	params, err := url.ParseQuery(initData)
	if err != nil {
		return nil, fmt.Errorf("telegram: parse initData: %w", err)
	}

	providedHash := params.Get("hash")
	if providedHash == "" {
		return nil, ErrInvalidHash
	}

	// Build data_check_string: sorted key=value pairs, excluding "hash"
	pairs := make([]string, 0, len(params))
	for k, vals := range params {
		if k == "hash" {
			continue
		}
		pairs = append(pairs, k+"="+vals[0])
	}
	sort.Strings(pairs)
	dataCheckString := strings.Join(pairs, "\n")

	// Compute expected hash
	mac := hmac.New(sha256.New, v.secretKey)
	mac.Write([]byte(dataCheckString))
	expectedHash := hex.EncodeToString(mac.Sum(nil))

	// Constant-time compare (hmac.Equal operates on []byte)
	if !hmac.Equal([]byte(providedHash), []byte(expectedHash)) {
		return nil, ErrInvalidHash
	}

	// Check auth_date freshness
	authDateStr := params.Get("auth_date")
	authDate, err := strconv.ParseInt(authDateStr, 10, 64)
	if err != nil || authDate == 0 {
		return nil, fmt.Errorf("telegram: missing auth_date")
	}
	age := time.Since(time.Unix(authDate, 0))
	if age > v.maxAge {
		return nil, ErrAuthExpired
	}

	// Parse user JSON
	userJSON := params.Get("user")
	if userJSON == "" {
		return nil, fmt.Errorf("telegram: missing user field")
	}
	var user User
	if err := json.Unmarshal([]byte(userJSON), &user); err != nil {
		return nil, fmt.Errorf("telegram: parse user: %w", err)
	}
	if user.ID == 0 {
		return nil, fmt.Errorf("telegram: user.id is zero")
	}

	return &user, nil
}
