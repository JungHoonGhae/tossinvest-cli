// Package confirmation owns the shared accidental-mutation confirmation
// primitive used by trading and non-trading settings workflows. Callers own
// their domain-specific canonical payload; this package only hashes and
// compares the resulting string consistently.
package confirmation

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const tokenLength = 12

// Token returns the short deterministic token displayed with a preview.
func Token(canonical string) string {
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:])[:tokenLength]
}

// Matches compares a supplied token with the preview token in constant time.
func Matches(supplied, expected string) bool {
	return subtle.ConstantTimeCompare([]byte(supplied), []byte(expected)) == 1
}

const timeBoundTokenVersion = "v1"

// IssueTimeBound returns a session-keyed confirmation token that carries its
// expiry. The canonical state stays private to the service; only callers with
// the same authenticated-session key can reproduce the signature.
func IssueTimeBound(key []byte, canonical string, now time.Time, ttl time.Duration) (string, error) {
	if len(key) == 0 {
		return "", fmt.Errorf("confirmation key is unavailable")
	}
	if ttl <= 0 {
		return "", fmt.Errorf("confirmation token lifetime must be positive")
	}
	expires := now.Add(ttl).Unix()
	expiry := strconv.FormatInt(expires, 10)
	signature := timeBoundSignature(key, expiry, canonical)
	return timeBoundTokenVersion + "." + expiry + "." + signature, nil
}

// VerifyTimeBound checks the signature and rejects expired tokens. Successful
// state changes invalidate replay because canonical includes the pre-write
// state; no-op execution is harmlessly repeatable until expiry.
func VerifyTimeBound(supplied string, key []byte, canonical string, now time.Time) bool {
	parts := strings.Split(supplied, ".")
	if len(parts) != 3 || parts[0] != timeBoundTokenVersion || len(key) == 0 {
		return false
	}
	expires, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || now.Unix() > expires {
		return false
	}
	expected := timeBoundSignature(key, parts[1], canonical)
	return subtle.ConstantTimeCompare([]byte(parts[2]), []byte(expected)) == 1
}

func timeBoundSignature(key []byte, expiry, canonical string) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(timeBoundTokenVersion))
	_, _ = mac.Write([]byte{'\x00'})
	_, _ = mac.Write([]byte(expiry))
	_, _ = mac.Write([]byte{'\x00'})
	_, _ = mac.Write([]byte(canonical))
	return hex.EncodeToString(mac.Sum(nil))[:24]
}
