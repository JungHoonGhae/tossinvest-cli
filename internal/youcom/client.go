// Package youcom provides a client for the You.com Research API
// (https://you.com), used as an external (non-Toss) source of deep market
// context with cited sources for the `market research` command. It is a
// third-party paid API, unrelated to Toss Securities' own WTS/Open API.
package youcom

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultBaseURL = "https://api.you.com/v1/research"
	defaultTimeout = 120 * time.Second

	// EnvAPIKey is the environment variable holding the You.com API key.
	EnvAPIKey = "YDC_API_KEY"

	// Research effort levels, cheapest to most expensive.
	EffortLite       = "lite"
	EffortStandard   = "standard"
	EffortDeep       = "deep"
	EffortExhaustive = "exhaustive"

	// DefaultEffort is used when no effort is specified.
	DefaultEffort = EffortLite

	// maxErrorBodyRunes caps how much of an error response body is echoed
	// back in error messages (rune-safe, never splits a multi-byte rune).
	maxErrorBodyRunes = 500
)

// validEfforts is the accepted set of research_effort values.
var validEfforts = map[string]bool{
	EffortLite:       true,
	EffortStandard:   true,
	EffortDeep:       true,
	EffortExhaustive: true,
}

// Sentinel errors returned by the client. Use errors.Is to check for them.
var (
	// ErrMissingAPIKey is returned by NewClient when YDC_API_KEY is unset.
	ErrMissingAPIKey = errors.New("youcom: YDC_API_KEY not set")
	// ErrInvalidEffort is returned when an unknown --effort value is passed.
	ErrInvalidEffort = errors.New("youcom: invalid research effort")
	// ErrAuth covers 401/403 responses (bad or unauthorized API key).
	ErrAuth = errors.New("youcom: authentication failed")
	// ErrInvalidRequest covers 422 responses (bad request parameters).
	ErrInvalidRequest = errors.New("youcom: invalid request")
	// ErrRateLimited covers 429 responses.
	ErrRateLimited = errors.New("youcom: rate limited")
	// ErrServer covers 5xx responses.
	ErrServer = errors.New("youcom: server error")
	// ErrTransport covers network-level failures (DNS, connection refused, etc).
	ErrTransport = errors.New("youcom: transport error")
	// ErrTimeout is returned when the request exceeds its deadline.
	ErrTimeout = errors.New("youcom: request timed out")
	// ErrMalformed is returned when the response body cannot be decoded.
	ErrMalformed = errors.New("youcom: malformed response")
)

// APIError is a passthrough for any non-2xx status not otherwise classified.
type APIError struct {
	Code int
	Body string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("youcom: API error %d: %s", e.Code, truncateBody(e.Body, maxErrorBodyRunes))
}

// Source is one cited source in a Research response.
type Source struct {
	URL      string   `json:"url"`
	Title    string   `json:"title"`
	Snippets []string `json:"snippets,omitempty"`
}

// Output is the research payload: the synthesized answer plus its citations.
type Output struct {
	Content string   `json:"content"`
	Sources []Source `json:"sources,omitempty"`
}

// ResearchResult is the top-level You.com Research API response.
type ResearchResult struct {
	Output Output `json:"output"`
}

// Client is a You.com Research API client. It has no retry logic — a failed
// call is surfaced to the caller as a typed error, never retried silently.
type Client struct {
	apiKey  string
	baseURL string
	hc      *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides the default API base URL (used in tests with httptest).
func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(u, "/") }
}

// WithHTTPClient overrides the HTTP client (used in tests to control timeouts
// and share an httptest transport).
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.hc = hc }
}

// NewClient constructs a Client, reading the API key from the YDC_API_KEY
// environment variable. It returns a wrapped ErrMissingAPIKey with actionable
// guidance if the variable is unset or empty, so callers can surface a clear
// error before attempting any network call.
func NewClient(opts ...Option) (*Client, error) {
	key := os.Getenv(EnvAPIKey)
	if key == "" {
		return nil, fmt.Errorf("%w: export YDC_API_KEY to use `market research` (get a key at https://you.com/platform/api-keys)", ErrMissingAPIKey)
	}
	return newClient(key, opts...), nil
}

// newClient builds a Client for the given key, applying opts over the
// defaults. NewClient uses it after resolving the key from the environment;
// tests in this package call it directly with a fixture key and
// WithBaseURL(srv.URL) to target an httptest server without touching the
// real environment.
func newClient(key string, opts ...Option) *Client {
	c := &Client{
		apiKey:  key,
		baseURL: defaultBaseURL,
		hc:      &http.Client{Timeout: defaultTimeout},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// researchRequest is the POST body sent to the Research API.
type researchRequest struct {
	Input          string `json:"input"`
	ResearchEffort string `json:"research_effort"`
}

// Research calls the You.com Research API for input, at the given effort
// level (one of lite|standard|deep|exhaustive). An empty effort defaults to
// "lite", the cheapest tier. The API key is never included in any returned
// error message. There is no retry logic: a single failed attempt is
// returned as-is.
func (c *Client) Research(ctx context.Context, input string, effort string) (*ResearchResult, error) {
	if effort == "" {
		effort = DefaultEffort
	}
	if !validEfforts[effort] {
		return nil, fmt.Errorf("%w: %q (want one of lite, standard, deep, exhaustive)", ErrInvalidEffort, effort)
	}

	body, err := json.Marshal(researchRequest{Input: input, ResearchEffort: effort})
	if err != nil {
		return nil, fmt.Errorf("%w: encoding request: %s", ErrTransport, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrTransport, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.hc.Do(req)
	if err != nil {
		if isTimeout(err) {
			return nil, fmt.Errorf("%w: You.com Research did not respond in time", ErrTimeout)
		}
		return nil, fmt.Errorf("%w: %s", ErrTransport, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: reading response body: %s", ErrTransport, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, classifyStatus(resp.StatusCode, respBody)
	}

	var result ResearchResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("%w: decoding response: %s", ErrMalformed, err)
	}
	return &result, nil
}

// isTimeout reports whether err is a network-level timeout (context deadline
// exceeded while the http.Client was waiting on the connection/response).
func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}
	return false
}

// classifyStatus maps a non-2xx HTTP status code (and response body) to a
// typed, actionable error. 401/403 -> ErrAuth (checks the key). 422 ->
// ErrInvalidRequest. 429 -> ErrRateLimited. >=500 -> ErrServer. Everything
// else -> *APIError passthrough.
func classifyStatus(code int, body []byte) error {
	switch {
	case code == http.StatusUnauthorized || code == http.StatusForbidden:
		return fmt.Errorf("%w: You.com rejected the request (status %d) — check that YDC_API_KEY is a valid, active key", ErrAuth, code)
	case code == http.StatusUnprocessableEntity:
		return fmt.Errorf("%w: You.com rejected the request parameters (status 422): %s", ErrInvalidRequest, truncateBody(string(body), maxErrorBodyRunes))
	case code == http.StatusTooManyRequests:
		return fmt.Errorf("%w: You.com rate limit hit (status 429) — wait and retry later", ErrRateLimited)
	case code >= 500:
		return fmt.Errorf("%w: You.com server error (status %d)", ErrServer, code)
	default:
		return &APIError{Code: code, Body: string(body)}
	}
}

// truncateBody truncates s to at most maxRunes runes, rune-safe (never
// splits a multi-byte rune), appending an ellipsis if truncated.
func truncateBody(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "…"
}
