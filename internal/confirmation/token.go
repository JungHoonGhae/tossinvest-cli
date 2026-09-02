// Package confirmation owns the shared accidental-mutation confirmation
// primitive used by trading and non-trading settings workflows. Callers own
// their domain-specific canonical payload; this package only hashes and
// compares the resulting string consistently.
package confirmation

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
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
