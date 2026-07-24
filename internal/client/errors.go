package client

import (
	"errors"
	"fmt"
)

var ErrNoSession = errors.New("no active session")

type StatusError struct {
	StatusCode int
	Endpoint   string
	Body       string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("unexpected status %d for %s", e.StatusCode, e.Endpoint)
}

// AuthError intentionally does not carry the response body: 401/403 bodies
// from wts-api / wts-cert-api can echo CSRF diagnostics or session-identifying
// fragments, and no caller reads it — only status + endpoint are surfaced to
// users. Dropping the field means debug `%+v` or accidental error-value
// serialization can't leak those fragments.
type AuthError struct {
	StatusCode int
	Endpoint   string
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("authenticated request rejected with status %d for %s", e.StatusCode, e.Endpoint)
}

func IsAuthError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, ErrNoSession) {
		return true
	}

	var authErr *AuthError
	return errors.As(err, &authErr)
}

func newStatusError(statusCode int, endpoint string, body []byte) error {
	if statusCode == 401 || statusCode == 403 {
		return &AuthError{
			StatusCode: statusCode,
			Endpoint:   endpoint,
		}
	}

	return &StatusError{
		StatusCode: statusCode,
		Endpoint:   endpoint,
		Body:       string(body),
	}
}
