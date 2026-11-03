package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type ExtensionStatus struct {
	Status string
}

// Captured live on 2026-05-05 against wts-api.tossinvest.com:
//
//	REQUESTED → COMPLETED → EXPIRED
//	pending     phone OK    finalized (POST /state) or doc TTL elapsed
//
// Service.Extend polls up to COMPLETED, then calls FinalizeExtension and
// returns — so it never observes EXPIRED for its own successful doc. If
// EXPIRED shows up DURING polling it means the doc was invalidated before
// approval (idle TTL or superseded by a newer request); we surface that as
// rejection so the user gets a clear retry hint instead of waiting for the
// outer timeout.
//
// Phone-rejection / user-cancel enums were not observed live; if Toss adds a
// distinct rejection string we'll polling-loop until --timeout (still safe,
// just less crisp UX). Add it here when captured.
var (
	extensionApprovedStates = map[string]struct{}{
		"COMPLETED": {},
	}
	extensionRejectedStates = map[string]struct{}{
		"EXPIRED": {},
	}
)

func (s ExtensionStatus) Approved() bool {
	_, ok := extensionApprovedStates[strings.ToUpper(strings.TrimSpace(s.Status))]
	return ok
}

func (s ExtensionStatus) Rejected() bool {
	_, ok := extensionRejectedStates[strings.ToUpper(strings.TrimSpace(s.Status))]
	return ok
}

type extensionRequestEnvelope struct {
	Result struct {
		TxID string `json:"txId"`
	} `json:"result"`
}

// /doc/{uuid}/status returns the status string directly under "result" (not
// nested under a struct). Captured live on 2026-05-05 — pending state is
// "REQUESTED"; approved/rejected enums are still being mapped.
type extensionStatusEnvelope struct {
	Result string `json:"result"`
}

type serverExpiredAtEnvelope struct {
	Result string `json:"result"`
}

// RequestExtension creates a session-extension document and returns the UUID
// the user must approve from the Toss phone app. The server fans out a push
// notification as a side effect.
func (c *Client) RequestExtension(ctx context.Context) (string, error) {
	if err := c.requireSession(); err != nil {
		return "", err
	}
	endpoint := c.apiBaseURL + "/api/v1/wts-login-extend/doc/request"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader([]byte("{}")))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	c.applySession(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := readBodyCapped(resp.Body, maxJSONBodyBytes)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", newStatusError(resp.StatusCode, endpoint, body)
	}

	var env extensionRequestEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return "", fmt.Errorf("decode extension request: %w", err)
	}
	id := strings.TrimSpace(env.Result.TxID)
	if id == "" {
		return "", fmt.Errorf("extension request returned empty txId")
	}
	return id, nil
}

// GetExtensionStatus polls a previously-issued extension document.
func (c *Client) GetExtensionStatus(ctx context.Context, uuid string) (ExtensionStatus, error) {
	if err := c.requireSession(); err != nil {
		return ExtensionStatus{}, err
	}
	if strings.TrimSpace(uuid) == "" {
		return ExtensionStatus{}, fmt.Errorf("extension uuid must not be empty")
	}
	endpoint := c.apiBaseURL + "/api/v1/wts-login-extend/doc/" + url.PathEscape(uuid) + "/status"
	var env extensionStatusEnvelope
	if err := c.getJSON(ctx, endpoint, &env); err != nil {
		return ExtensionStatus{}, err
	}
	return ExtensionStatus{Status: env.Result}, nil
}

// FinalizeExtension consumes an approved extension document. Without this
// step the server-side expiry clock does NOT advance even though
// /doc/{uuid}/status reports "COMPLETED" — the web UI calls this immediately
// after the status transition and only then does /session/expired-at return a
// new value.
func (c *Client) FinalizeExtension(ctx context.Context, uuid string) error {
	if err := c.requireSession(); err != nil {
		return err
	}
	if strings.TrimSpace(uuid) == "" {
		return fmt.Errorf("extension uuid must not be empty")
	}
	endpoint := c.apiBaseURL + "/api/v1/wts-login-extend/" + url.PathEscape(uuid) + "/state"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader([]byte("{}")))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	c.applySession(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := readBodyCapped(resp.Body, maxJSONBodyBytes)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return newStatusError(resp.StatusCode, endpoint, body)
	}
	return nil
}

// GetServerExpiredAt returns the current server-side session expiry time as a
// time.Time preserving its original offset (KST in observed responses).
func (c *Client) GetServerExpiredAt(ctx context.Context) (time.Time, error) {
	if err := c.requireSession(); err != nil {
		return time.Time{}, err
	}
	endpoint := c.apiBaseURL + "/api/v1/session/expired-at"
	var env serverExpiredAtEnvelope
	if err := c.getJSON(ctx, endpoint, &env); err != nil {
		return time.Time{}, err
	}
	parsed, err := time.Parse(time.RFC3339Nano, env.Result)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse server expired-at %q: %w", env.Result, err)
	}
	return parsed, nil
}
