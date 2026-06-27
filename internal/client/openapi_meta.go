package client

// openapi_meta.go — WTS internal endpoints for Open API key metadata.
//
// These paths are served by wts-api.tossinvest.com and require a live web
// session (same session as all other WTS reads). They are NOT part of the
// official 21-endpoint OAuth surface; they power the Toss settings UI.
//
// Assumed response shapes (no live capture available — reconcile on first run):
//
//   GET /api/v1/openapi/client
//   {"result": {
//     "status":    "ACTIVE",          // string; "ACTIVE"|"INACTIVE" or Korean equiv
//     "issuedAt":  "2025-01-15T09:00:00Z",   // RFC3339; fallback: "createdAt"
//     "expiresAt": "2026-01-15T09:00:00Z",   // RFC3339; fallback: "expiredAt"
//     "active":    true               // bool; optional — derived from status if absent
//   }}
//
//   GET /api/v1/openapi/client/allowed-ips
//   {"result": ["1.2.3.4", "5.6.7.8"]}           // primary: []string
//   {"result": [{"ip":"1.2.3.4"},{"ip":"5.6.7.8"}]} // alternate: []{ip string}

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

// OpenAPIClientInfo holds key metadata for the user's WTS-side Open API key.
type OpenAPIClientInfo struct {
	Status    string
	IssuedAt  time.Time
	ExpiresAt time.Time
	Active    bool
}

// raw envelope for GET /api/v1/openapi/client
type openapiClientEnvelope struct {
	Result struct {
		Status    string          `json:"status"`
		IssuedAt  json.RawMessage `json:"issuedAt"`
		CreatedAt json.RawMessage `json:"createdAt"` // fallback field name
		ExpiresAt json.RawMessage `json:"expiresAt"`
		ExpiredAt json.RawMessage `json:"expiredAt"` // fallback field name
		Active    *bool           `json:"active"`    // pointer so we can detect absence
	} `json:"result"`
}

// raw envelope for GET /api/v1/openapi/client/allowed-ips
type openapiAllowedIPsEnvelope struct {
	Result json.RawMessage `json:"result"`
}

// OpenAPIClientInfo fetches the user's WTS Open API key metadata.
// Maps to GET /api/v1/openapi/client.
func (c *Client) OpenAPIClientInfo(ctx context.Context) (OpenAPIClientInfo, error) {
	if err := c.requireSession(); err != nil {
		return OpenAPIClientInfo{}, err
	}

	var envelope openapiClientEnvelope
	if err := c.getJSON(ctx, c.apiBaseURL+"/api/v1/openapi/client", &envelope); err != nil {
		return OpenAPIClientInfo{}, err
	}

	r := envelope.Result

	// Parse IssuedAt: try issuedAt first, then createdAt
	issuedAt := parseRawDate(r.IssuedAt)
	if issuedAt.IsZero() {
		issuedAt = parseRawDate(r.CreatedAt)
	}

	// Parse ExpiresAt: try expiresAt first, then expiredAt
	expiresAt := parseRawDate(r.ExpiresAt)
	if expiresAt.IsZero() {
		expiresAt = parseRawDate(r.ExpiredAt)
	}

	// Determine active: use explicit boolean if present, else derive from status
	active := false
	if r.Active != nil {
		active = *r.Active
	} else {
		upper := strings.ToUpper(strings.TrimSpace(r.Status))
		active = upper == "ACTIVE" || upper == "활성"
	}

	return OpenAPIClientInfo{
		Status:    r.Status,
		IssuedAt:  issuedAt,
		ExpiresAt: expiresAt,
		Active:    active,
	}, nil
}

// OpenAPIAllowedIPs fetches the IP allowlist for the user's WTS Open API key.
// Maps to GET /api/v1/openapi/client/allowed-ips.
// Handles both []string and [{"ip":"..."}] response shapes.
func (c *Client) OpenAPIAllowedIPs(ctx context.Context) ([]string, error) {
	if err := c.requireSession(); err != nil {
		return nil, err
	}

	var envelope openapiAllowedIPsEnvelope
	if err := c.getJSON(ctx, c.apiBaseURL+"/api/v1/openapi/client/allowed-ips", &envelope); err != nil {
		return nil, err
	}

	return parseAllowedIPs(envelope.Result), nil
}

// parseAllowedIPs handles both primary ([]string) and alternate ([{"ip":""}]) shapes.
func parseAllowedIPs(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}

	// Try primary shape: []string
	var strSlice []string
	if err := json.Unmarshal(raw, &strSlice); err == nil {
		return strSlice
	}

	// Try alternate shape: [{"ip": "..."}]
	var objSlice []struct {
		IP string `json:"ip"`
	}
	if err := json.Unmarshal(raw, &objSlice); err == nil {
		ips := make([]string, 0, len(objSlice))
		for _, obj := range objSlice {
			if obj.IP != "" {
				ips = append(ips, obj.IP)
			}
		}
		return ips
	}

	return nil
}

// parseRawDate unquotes a JSON string and tries RFC3339 then common fallbacks.
// Returns zero time.Time on any parse failure — callers must not error on zero.
func parseRawDate(raw json.RawMessage) time.Time {
	if len(raw) == 0 {
		return time.Time{}
	}

	var s string
	if err := json.Unmarshal(raw, &s); err != nil || s == "" {
		return time.Time{}
	}

	// RFC3339 with nanoseconds
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t
	}
	// Plain RFC3339
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	// Common local format without TZ (treat as UTC)
	if t, err := time.Parse("2006-01-02T15:04:05", s); err == nil {
		return t.UTC()
	}
	// Date-only
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.UTC()
	}

	return time.Time{}
}
