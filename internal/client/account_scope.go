package client

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const accountScopeTokenLength = 12

// getJSONWithAccountKey is getJSON with the account-scoping header used by
// per-account Securities endpoints.
func (c *Client) getJSONWithAccountKey(ctx context.Context, endpoint, accountKey string, target any) error {
	return c.doJSONWithAccountKey(ctx, http.MethodGet, endpoint, nil, accountKey, target)
}

// postJSONWithAccountKey sends an account-scoped read whose HTTP transport is
// POST and decodes its JSON response. The method name deliberately says POST,
// not mutate: WTS uses POST for several read-only dashboard queries.
func (c *Client) postJSONWithAccountKey(ctx context.Context, endpoint string, body []byte, accountKey string, target any) error {
	return c.doJSONWithAccountKey(ctx, http.MethodPost, endpoint, body, accountKey, target)
}

// mutateJSONWithAccountKey sends an account-scoped JSON mutation. Callers own
// all validation and human-confirmation gates before crossing this internal
// seam.
func (c *Client) mutateJSONWithAccountKey(ctx context.Context, method, endpoint string, body []byte, accountKey string) error {
	return c.doJSONWithAccountKey(ctx, method, endpoint, body, accountKey, nil)
}

// doJSONWithAccountKey centralizes the transport contract shared by account-
// scoped reads and writes. HTTP method remains separate from semantic intent:
// callers choose postJSONWithAccountKey for read-only POSTs and
// mutateJSONWithAccountKey only after their confirmation boundary.
func (c *Client) doJSONWithAccountKey(ctx context.Context, method, endpoint string, body []byte, accountKey string, target any) error {
	var requestBody io.Reader
	if method != http.MethodGet || body != nil {
		requestBody = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, requestBody)
	if err != nil {
		return err
	}
	c.applySession(req)
	if method != http.MethodGet {
		req.Header.Set("Content-Type", "application/json")
	}
	if accountKey != "" {
		req.Header.Set("accountKey", accountKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return newStatusError(resp.StatusCode, endpoint, data)
	}
	if target != nil {
		return json.Unmarshal(data, target)
	}
	return nil
}

// primaryAccountKey returns the primary Securities account key, falling back
// to the first account when the upstream response has no explicit primary.
func (c *Client) primaryAccountKey(ctx context.Context) (string, error) {
	accounts, primary, err := c.ListAccounts(ctx)
	if err != nil {
		return "", err
	}
	if primary != "" {
		return primary, nil
	}
	if len(accounts) > 0 {
		return accounts[0].ID, nil
	}
	return "", fmt.Errorf("no account found")
}

// resolveAccountKey preserves an explicit account selection and falls back to
// the primary Securities account only when the caller omits it.
func (c *Client) resolveAccountKey(ctx context.Context, accountKey string) (string, error) {
	if key := strings.TrimSpace(accountKey); key != "" {
		return key, nil
	}
	return c.primaryAccountKey(ctx)
}

// accountScope returns a session-bound opaque identifier for an account. A
// plain hash is insufficient because WTS account keys can have very little
// entropy; keying the digest with the authenticated session prevents an
// observer from recovering the account key by enumerating likely values.
func (c *Client) accountScope(accountKey string) string {
	secret := c.sessionKeyMaterial()
	if len(secret) == 0 {
		// Account-scoped public methods require a session before reaching here.
		// Keep this private helper fail-closed if that invariant is broken.
		return "unavailable"
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte("tossctl/account-scope/v1\x00"))
	_, _ = mac.Write([]byte(accountKey))
	return hex.EncodeToString(mac.Sum(nil))[:accountScopeTokenLength]
}

// ConfirmationKey derives purpose-separated key material from the current WTS
// session without exposing the session cookie itself. Domain services use it
// to make preview tokens invalid after an account/session switch.
func (c *Client) ConfirmationKey(purpose string) []byte {
	secret := c.sessionKeyMaterial()
	if len(secret) == 0 {
		return nil
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte("tossctl/confirmation-key/v1\x00"))
	_, _ = mac.Write([]byte(purpose))
	return mac.Sum(nil)
}

func (c *Client) sessionKeyMaterial() []byte {
	if c.session == nil {
		return nil
	}
	secret := []byte(c.session.Cookies["SESSION"])
	if len(secret) != 0 {
		return secret
	}
	// encoding/json sorts string map keys, producing stable key material for
	// providers that authenticate with cookies other than SESSION.
	secret, _ = json.Marshal(c.session.Cookies)
	return secret
}
