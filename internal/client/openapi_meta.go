package client

// openapi_meta.go — WTS internal endpoint for Open API key metadata.
//
// Served by wts-api.tossinvest.com; requires a live web session (same as all
// other WTS reads). NOT part of the official 21-endpoint OAuth surface; powers
// the Toss settings UI and `tossctl openapi status` / `doctor` diagnostics.
//
// Real response shape (verified live 2026-06-27 against /api/v1/openapi/client):
//
//   {"result": {
//     "id": 0, "userId": 0, "gaId": 0,
//     "clientId":               "tsck_live_…",      // the API key
//     "clientSecret":           "tssk_live_…",      // ⚠ plaintext secret — NEVER mapped/logged
//     "clientIdIssuedAt":       "2026-06-27T14:29:00Z",   // RFC3339 — issued
//     "clientSecretExpiresAt":  "2027-06-27T14:29:00Z",   // RFC3339 — expiry
//     "clientName": "…", "tier": "BASIC", "scopes": ["…"],
//     "allowedIps": [{"ip":"203.0.113.7","osName":null,"agentName":null,"createdAt":"…"}]
//   }}
//
// There is NO `status`/`active` field — activity is derived from the expiry.
// The standalone GET /api/v1/openapi/client/allowed-ips returns 400 (does not
// exist as a GET); the allowlist is embedded here as `allowedIps`.

import (
	"context"
	"encoding/json"
	"time"
)

// OpenAPIClientInfo holds key metadata for the user's WTS-side Open API key.
type OpenAPIClientInfo struct {
	Status     string // derived: "활성" / "만료"
	IssuedAt   time.Time
	ExpiresAt  time.Time
	Active     bool
	Tier       string
	AllowedIPs []string
}

// raw envelope for GET /api/v1/openapi/client.
// Intentionally does NOT include clientSecret — the endpoint returns it in
// plaintext and we must never capture, store, or log it.
type openapiClientEnvelope struct {
	Result struct {
		ClientIDIssuedAt      json.RawMessage `json:"clientIdIssuedAt"`
		ClientSecretExpiresAt json.RawMessage `json:"clientSecretExpiresAt"`
		Tier                  string          `json:"tier"`
		AllowedIPs            []struct {
			IP string `json:"ip"`
		} `json:"allowedIps"`
	} `json:"result"`
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

	issuedAt := parseRawDate(r.ClientIDIssuedAt)
	expiresAt := parseRawDate(r.ClientSecretExpiresAt)

	// No status/active field in the response — derive activity from the expiry.
	active := !expiresAt.IsZero() && expiresAt.After(time.Now())
	status := "만료"
	if active {
		status = "활성"
	}

	ips := make([]string, 0, len(r.AllowedIPs))
	for _, a := range r.AllowedIPs {
		if a.IP != "" {
			ips = append(ips, a.IP)
		}
	}

	return OpenAPIClientInfo{
		Status:     status,
		IssuedAt:   issuedAt,
		ExpiresAt:  expiresAt,
		Active:     active,
		Tier:       r.Tier,
		AllowedIPs: ips,
	}, nil
}

// OpenAPIAllowedIPs returns the IP allowlist for the user's WTS Open API key.
// The allowlist is embedded in the /api/v1/openapi/client response (the
// standalone /allowed-ips GET returns 400), so this delegates to
// OpenAPIClientInfo rather than calling a separate endpoint.
func (c *Client) OpenAPIAllowedIPs(ctx context.Context) ([]string, error) {
	info, err := c.OpenAPIClientInfo(ctx)
	if err != nil {
		return nil, err
	}
	return info.AllowedIPs, nil
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

	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02T15:04:05", s); err == nil {
		return t.UTC()
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.UTC()
	}

	return time.Time{}
}
