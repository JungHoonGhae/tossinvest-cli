package official

import (
	"bytes"
	"errors"
	"fmt"
)

// Sentinel errors returned by the official API client.
var (
	ErrTransport    = errors.New("official: transport error")
	ErrAuth         = errors.New("official: authentication failed")
	ErrIPNotAllowed = errors.New("official: IP not allowed")
	ErrRateLimited  = errors.New("official: rate limited")
	ErrServer       = errors.New("official: server error")
)

// APIError represents a non-retryable 4xx error from the official API.
// It is a passthrough and does NOT trigger fallback to the unofficial client.
type APIError struct {
	Code int
	Body string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("official: API error %d: %s", e.Code, e.Body)
}

// classifyStatus maps an HTTP status code (and optional response body) to a
// typed error. 401/403 become ErrIPNotAllowed when the body mentions "ip"
// (case-insensitive), otherwise ErrAuth. 429 → ErrRateLimited. ≥500 →
// ErrServer. All other 4xx → *APIError (passthrough, no fallback).
func classifyStatus(code int, body []byte) error {
	switch {
	case code == 401 || code == 403:
		if bytes.Contains(bytes.ToLower(body), []byte("ip")) {
			return ErrIPNotAllowed
		}
		return ErrAuth
	case code == 429:
		return ErrRateLimited
	case code >= 500:
		return ErrServer
	default:
		return &APIError{Code: code, Body: string(body)}
	}
}

// ShouldFallback reports whether err indicates the official API is unavailable
// or the request should be retried via the unofficial (web-session) client.
// Returns true for sentinel errors; false for *APIError passthroughs.
func ShouldFallback(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return false
	}
	return errors.Is(err, ErrTransport) ||
		errors.Is(err, ErrAuth) ||
		errors.Is(err, ErrIPNotAllowed) ||
		errors.Is(err, ErrRateLimited) ||
		errors.Is(err, ErrServer)
}
