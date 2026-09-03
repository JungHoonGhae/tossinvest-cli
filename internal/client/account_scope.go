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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	c.applySession(req)
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
	return json.Unmarshal(data, target)
}

// mutateJSONWithAccountKey sends an account-scoped JSON mutation. Callers own
// all validation and human-confirmation gates before crossing this internal
// seam.
func (c *Client) mutateJSONWithAccountKey(ctx context.Context, method, endpoint string, body []byte, accountKey string) error {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	c.applySession(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("accountKey", accountKey)
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
	var secret []byte
	if c.session != nil {
		secret = []byte(c.session.Cookies["SESSION"])
		if len(secret) == 0 {
			// encoding/json sorts string map keys, producing stable key material
			// for providers that authenticate with cookies other than SESSION.
			secret, _ = json.Marshal(c.session.Cookies)
		}
	}
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
