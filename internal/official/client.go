package official

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultBaseURL = "https://openapi.tossinvest.com"
const defaultTimeout = 15 * time.Second

// Client is the official Toss Open API client.
// It manages OAuth2 token acquisition/refresh and provides authed HTTP helpers.
type Client struct {
	base       string
	hc         *http.Client
	tm         *tokenManager
	mu         sync.Mutex // guards accountSeq lazy resolution
	accountSeq int        // used for X-Tossinvest-Account header (0 = unset, resolved lazily)
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides the default API base URL (used in tests with httptest).
func WithBaseURL(u string) Option {
	return func(c *Client) {
		c.base = strings.TrimRight(u, "/")
	}
}

// WithHTTPClient overrides the HTTP client (used in tests to share httptest transport).
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		c.hc = hc
	}
}

// WithAccountSeq sets the default account sequence number sent as the
// X-Tossinvest-Account header on account-scoped endpoints (BuyingPower, Holdings).
func WithAccountSeq(seq int) Option {
	return func(c *Client) {
		c.accountSeq = seq
	}
}

// New constructs a Client. cacheFile is the path for the on-disk token cache.
// Options are applied after defaults, so WithBaseURL/WithHTTPClient override them.
func New(creds Credentials, cacheFile string, opts ...Option) *Client {
	c := &Client{
		base: defaultBaseURL,
		hc:   &http.Client{Timeout: defaultTimeout},
	}
	for _, o := range opts {
		o(c)
	}
	// tokenManager shares the same http.Client so httptest servers handle both
	// /oauth2/token and data paths without TLS cert mismatches.
	c.tm = newTokenManager(creds, c.base, cacheFile, c.hc)
	return c
}

// BaseURL returns the base URL this client targets.
func (c *Client) BaseURL() string {
	return c.base
}

// apiEnvelope is the common response wrapper: {"result": <payload>}.
type apiEnvelope struct {
	Result json.RawMessage `json:"result"`
}

// doRequest executes req, returning (statusCode, body, error).
func (c *Client) doRequest(req *http.Request) (int, []byte, error) {
	resp, err := c.hc.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("%w: %s", ErrTransport, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("%w: reading body: %s", ErrTransport, err)
	}
	return resp.StatusCode, body, nil
}

// unwrapAndDecode extracts `result` from the response envelope and unmarshals
// it into out. If out is nil the body is discarded. Responses are expected to
// have the shape {"result": <payload>}; if the `result` key is absent and out
// is non-nil an error is returned.
func unwrapAndDecode(body []byte, out any) error {
	if out == nil {
		return nil
	}
	var env apiEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("%w: decoding envelope: %s", ErrServer, err)
	}
	if env.Result == nil {
		return fmt.Errorf("%w: response has no 'result' key", ErrServer)
	}
	if err := json.Unmarshal(env.Result, out); err != nil {
		return fmt.Errorf("%w: decoding result payload: %s", ErrServer, err)
	}
	return nil
}

// get performs an authenticated GET request to path (relative to BaseURL).
// Query parameters q may be nil. On 401 the token is refreshed and the request
// is retried once. On non-2xx classifyStatus is returned. On 2xx the `result`
// envelope is unwrapped into out (out may be nil).
func (c *Client) get(ctx context.Context, path string, q url.Values, out any) error {
	return c.getWithHeaders(ctx, path, q, nil, out)
}

// ensureAccountSeq returns the account sequence number, resolving it lazily
// when no seq was provided via WithAccountSeq. The first call (accountSeq==0)
// fetches /api/v1/accounts, parses the first entry's ID, and caches the result.
// Subsequent calls return the cached value with no network round-trip. The mu
// field serialises concurrent first-call resolution so /accounts is called at
// most once. Callers that set WithAccountSeq at construction time skip the
// fetch entirely.
func (c *Client) ensureAccountSeq(ctx context.Context) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.accountSeq != 0 {
		return c.accountSeq, nil
	}
	accts, err := c.Accounts(ctx)
	if err != nil {
		return 0, fmt.Errorf("lazy account-seq resolution: %w", err)
	}
	if len(accts) == 0 {
		return 0, fmt.Errorf("lazy account-seq resolution: no accounts found")
	}
	seq, err := strconv.Atoi(accts[0].ID)
	if err != nil {
		return 0, fmt.Errorf("lazy account-seq resolution: parsing account ID %q: %w", accts[0].ID, err)
	}
	c.accountSeq = seq
	return seq, nil
}

// getAcct is like get but also resolves the account sequence (lazily if needed)
// and sets the X-Tossinvest-Account header. Used by account-scoped endpoints
// such as BuyingPower, Holdings, and Orders that require the header per the
// official API spec.
func (c *Client) getAcct(ctx context.Context, path string, q url.Values, out any) error {
	seq, err := c.ensureAccountSeq(ctx)
	if err != nil {
		return err
	}
	extra := map[string]string{"X-Tossinvest-Account": strconv.Itoa(seq)}
	return c.getWithHeaders(ctx, path, q, extra, out)
}

// postAcct is like post but injects the X-Tossinvest-Account header (lazy seq).
func (c *Client) postAcct(ctx context.Context, path string, body any, out any) error {
	seq, err := c.ensureAccountSeq(ctx)
	if err != nil {
		return err
	}
	extra := map[string]string{"X-Tossinvest-Account": strconv.Itoa(seq)}
	return c.postWithHeaders(ctx, path, body, extra, out)
}

// deleteAcct performs an authenticated DELETE with the account header, mirroring
// send runs the authenticated-request flow every verb shares: acquire a token,
// build the request, retry ONCE with a refreshed token on 401, classify non-2xx,
// and unwrap the `result` envelope into out (out may be nil).
//
// Callers supply makeReq because that is the only part that differs between
// verbs — method, body, query, and per-request headers. Keeping the retry here
// means the auth policy is decided in one place; it used to be hand-repeated in
// getWithHeaders, postWithHeaders and deleteAcct, where it could silently drift.
//
// makeReq must be callable twice: the retry rebuilds the request so the new
// token is applied (and so a consumed body reader is not reused).
func (c *Client) send(ctx context.Context, makeReq func(tok string) (*http.Request, error), out any) error {
	tok, err := c.tm.token(ctx)
	if err != nil {
		return err
	}
	req, err := makeReq(tok)
	if err != nil {
		return err
	}
	code, body, err := c.doRequest(req)
	if err != nil {
		return err
	}
	if code == http.StatusUnauthorized {
		// Force-refresh the token and retry once.
		tok, err = c.tm.refresh(ctx)
		if err != nil {
			return err
		}
		req, err = makeReq(tok)
		if err != nil {
			return err
		}
		code, body, err = c.doRequest(req)
		if err != nil {
			return err
		}
	}
	if code < 200 || code >= 300 {
		return classifyStatus(code, body)
	}
	return unwrapAndDecode(body, out)
}

// getWithHeaders' token/401-retry/unwrap flow.
func (c *Client) deleteAcct(ctx context.Context, path string, out any) error {
	seq, err := c.ensureAccountSeq(ctx)
	if err != nil {
		return err
	}
	rawURL := c.base + path
	makeReq := func(tok string) (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, rawURL, nil)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", ErrTransport, err)
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("X-Tossinvest-Account", strconv.Itoa(seq))
		return req, nil
	}
	return c.send(ctx, makeReq, out)
}

// getWithHeaders performs an authenticated GET request to path (relative to
// BaseURL), injecting any extraHeaders on top of the Authorization header.
// Query parameters q may be nil. On 401 the token is refreshed and the request
// is retried once. On non-2xx classifyStatus is returned. On 2xx the `result`
// envelope is unwrapped into out (out may be nil).
func (c *Client) getWithHeaders(ctx context.Context, path string, q url.Values, extraHeaders map[string]string, out any) error {
	rawURL := c.base + path
	if len(q) > 0 {
		rawURL += "?" + q.Encode()
	}

	makeReq := func(tok string) (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", ErrTransport, err)
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		for k, v := range extraHeaders {
			req.Header.Set(k, v)
		}
		return req, nil
	}

	return c.send(ctx, makeReq, out)
}

// post performs an authenticated POST request to path (relative to BaseURL).
// body is JSON-encoded and sent with Content-Type: application/json. On 401
// the token is refreshed and the request is retried once. On non-2xx
// classifyStatus is returned. On 2xx the `result` envelope is unwrapped into
// out (out may be nil).
func (c *Client) post(ctx context.Context, path string, body any, out any) error {
	return c.postWithHeaders(ctx, path, body, nil, out)
}

// postWithHeaders is like post but also injects extraHeaders (e.g.
// X-Tossinvest-Account) on top of the Authorization header. extraHeaders may
// be nil. On 401 the token is refreshed and the request is retried once. On
// non-2xx classifyStatus is returned. On 2xx the `result` envelope is
// unwrapped into out (out may be nil).
func (c *Client) postWithHeaders(ctx context.Context, path string, body any, extraHeaders map[string]string, out any) error {
	rawURL := c.base + path

	encoded, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("%w: marshalling request body: %s", ErrTransport, err)
	}

	makeReq := func(tok string) (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(encoded))
		if err != nil {
			return nil, fmt.Errorf("%w: %s", ErrTransport, err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tok)
		for k, v := range extraHeaders {
			req.Header.Set(k, v)
		}
		return req, nil
	}

	return c.send(ctx, makeReq, out)
}
