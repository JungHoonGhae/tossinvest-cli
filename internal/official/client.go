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
	accountSeq int // used for X-Tossinvest-Account header (0 = unset)
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

// getAcct is like get but also sets the X-Tossinvest-Account header when the
// client's accountSeq is non-zero. Used by account-scoped endpoints such as
// BuyingPower and Holdings that require the header per the official API spec.
func (c *Client) getAcct(ctx context.Context, path string, q url.Values, out any) error {
	var extra map[string]string
	if c.accountSeq != 0 {
		extra = map[string]string{"X-Tossinvest-Account": strconv.Itoa(c.accountSeq)}
	}
	return c.getWithHeaders(ctx, path, q, extra, out)
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

// post performs an authenticated POST request to path (relative to BaseURL).
// body is JSON-encoded and sent with Content-Type: application/json. On 401
// the token is refreshed and the request is retried once. On non-2xx
// classifyStatus is returned. On 2xx the `result` envelope is unwrapped into
// out (out may be nil).
func (c *Client) post(ctx context.Context, path string, body any, out any) error {
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
		return req, nil
	}

	tok, err := c.tm.token(ctx)
	if err != nil {
		return err
	}

	req, err := makeReq(tok)
	if err != nil {
		return err
	}

	code, respBody, doErr := c.doRequest(req)
	if doErr != nil {
		return doErr
	}

	if code == http.StatusUnauthorized {
		tok, err = c.tm.refresh(ctx)
		if err != nil {
			return err
		}
		req, err = makeReq(tok)
		if err != nil {
			return err
		}
		code, respBody, doErr = c.doRequest(req)
		if doErr != nil {
			return doErr
		}
	}

	if code < 200 || code >= 300 {
		return classifyStatus(code, respBody)
	}

	return unwrapAndDecode(respBody, out)
}
