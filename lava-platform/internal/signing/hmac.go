// Package signing implements HMAC-SHA256 request authentication used both
// for verifying incoming operator requests and signing outgoing callbacks.
package signing

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"
)

const MaxClockSkew = 5 * time.Minute

// Canonical builds the string that is signed / verified.
// Format: METHOD\nPATH\nTIMESTAMP\nSHA256(body_hex)
func Canonical(method, path string, timestamp int64, body []byte) string {
	h := sha256.Sum256(body)
	return fmt.Sprintf("%s\n%s\n%d\n%s", method, path, timestamp, hex.EncodeToString(h[:]))
}

// Sign returns base64-encoded HMAC-SHA256 of the canonical string.
func Sign(secretKey, method, path string, timestamp int64, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(Canonical(method, path, timestamp, body)))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// Verify checks the signature and enforces timestamp freshness (replay protection).
func Verify(secretKey, method, path string, timestamp int64, body []byte, sig string) bool {
	if !freshTimestamp(timestamp) {
		return false
	}
	expected := Sign(secretKey, method, path, timestamp, body)
	return hmac.Equal([]byte(expected), []byte(sig))
}

func freshTimestamp(ts int64) bool {
	t := time.Unix(ts, 0)
	diff := time.Now().Sub(t)
	if diff < 0 {
		diff = -diff
	}
	return diff <= MaxClockSkew
}
