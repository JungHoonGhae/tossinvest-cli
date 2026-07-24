package client

import (
	"io"
	"net/http"
	"time"

	"context"
)

// ProbeResult captures a single endpoint-family health probe. Meant for
// diagnostics (e.g. `tossctl doctor --report`) — not for production code paths.
type ProbeResult struct {
	Family     string        `json:"family"`      // wts-api | wts-cert-api | wts-info-api
	Endpoint   string        `json:"endpoint"`    // URL path only (no host)
	StatusCode int           `json:"status_code"` // 0 if transport error
	Duration   time.Duration `json:"duration_ns"`
	Error      string        `json:"error,omitempty"`      // transport error only
	AuthError  bool          `json:"auth_error,omitempty"` // 401/403 — session/UA problems
}

// Probe hits one read-only endpoint per API family and reports the response
// status. Used for diagnostics; avoids the higher-level typed call paths so a
// single 4xx doesn't short-circuit the rest. The probe uses the same session +
// default browser UA as every other call, so it reflects what real commands
// would see right now.
func (c *Client) Probe(ctx context.Context) []ProbeResult {
	targets := []struct {
		family   string
		endpoint string
	}{
		{"wts-api", c.apiBaseURL + "/api/v1/account/list"},
		{"wts-cert-api", c.certBaseURL + "/api/v3/my-assets/summaries/markets/all/overview"},
		{"wts-info-api", c.infoBaseURL + "/api/v2/stock-infos/A005930"},
	}

	results := make([]ProbeResult, 0, len(targets))
	for _, t := range targets {
		results = append(results, c.probeOne(ctx, t.family, t.endpoint))
	}
	return results
}

func (c *Client) probeOne(ctx context.Context, family, endpoint string) ProbeResult {
	probeCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ProbeResult{Family: family, Endpoint: endpoint, Error: err.Error()}
	}
	c.applySession(req)

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	dur := time.Since(start)
	if err != nil {
		return ProbeResult{Family: family, Endpoint: endpoint, Duration: dur, Error: err.Error()}
	}
	defer resp.Body.Close()
	// Drain body so the connection can be reused and timings reflect full response.
	_, _ = io.Copy(io.Discard, resp.Body)

	return ProbeResult{
		Family:     family,
		Endpoint:   endpoint,
		StatusCode: resp.StatusCode,
		Duration:   dur,
		AuthError:  resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden,
	}
}
