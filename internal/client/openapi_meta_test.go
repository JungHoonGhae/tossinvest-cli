package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
)

func newOpenAPITestClient(t *testing.T, mux *http.ServeMux) *Client {
	t.Helper()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return New(Config{
		HTTPClient: server.Client(),
		APIBaseURL: server.URL,
		Session: &session.Session{
			Cookies: map[string]string{"SESSION": "test-session"},
		},
	})
}

// clientBody renders a dummy /api/v1/openapi/client response in the REAL shape.
// Dummy values only (RFC 5737 TEST-NET IPs; placeholder key/secret).
func clientBody(issued, expires string, ips ...string) string {
	allowed := ""
	for i, ip := range ips {
		if i > 0 {
			allowed += ","
		}
		allowed += fmt.Sprintf(`{"ip":%q,"osName":null,"agentName":null,"createdAt":"2026-06-27T14:29:00.123456789Z"}`, ip)
	}
	return fmt.Sprintf(`{"result":{
		"id":1,"userId":2,"gaId":3,
		"clientId":"tsck_live_dummy","clientSecret":"tssk_live_SHOULD_NEVER_SURFACE",
		"clientIdIssuedAt":%q,"clientSecretExpiresAt":%q,
		"clientName":"dummy","tier":"BASIC","scopes":["read","trade"],
		"allowedIps":[%s]
	}}`, issued, expires, allowed)
}

func TestOpenAPIClientInfo_ParsesRealShape(t *testing.T) {
	t.Parallel()

	issued := "2026-06-27T14:29:00Z"
	expires := time.Now().Add(365 * 24 * time.Hour).UTC().Format(time.RFC3339) // future → active

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/openapi/client", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(clientBody(issued, expires, "203.0.113.7", "203.0.113.8")))
	})

	info, err := newOpenAPITestClient(t, mux).OpenAPIClientInfo(context.Background())
	if err != nil {
		t.Fatalf("OpenAPIClientInfo error: %v", err)
	}
	if !info.Active || info.Status != "활성" {
		t.Errorf("expected active/활성, got Active=%v Status=%q", info.Active, info.Status)
	}
	if info.Tier != "BASIC" {
		t.Errorf("Tier: got %q, want BASIC", info.Tier)
	}
	wantIssued := time.Date(2026, 6, 27, 14, 29, 0, 0, time.UTC)
	if !info.IssuedAt.Equal(wantIssued) {
		t.Errorf("IssuedAt: got %v, want %v", info.IssuedAt, wantIssued)
	}
	if len(info.AllowedIPs) != 2 || info.AllowedIPs[0] != "203.0.113.7" || info.AllowedIPs[1] != "203.0.113.8" {
		t.Errorf("AllowedIPs: got %v", info.AllowedIPs)
	}
}

func TestOpenAPIClientInfo_ExpiredDerivesInactive(t *testing.T) {
	t.Parallel()

	expires := time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339) // past → expired

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/openapi/client", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(clientBody("2025-01-01T00:00:00Z", expires)))
	})

	info, err := newOpenAPITestClient(t, mux).OpenAPIClientInfo(context.Background())
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if info.Active || info.Status != "만료" {
		t.Errorf("expected inactive/만료, got Active=%v Status=%q", info.Active, info.Status)
	}
}

func TestOpenAPIClientInfo_BadExpiryDoesNotError(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/openapi/client", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"clientIdIssuedAt":"nope","clientSecretExpiresAt":"also-bad","tier":"BASIC","allowedIps":[]}}`))
	})

	info, err := newOpenAPITestClient(t, mux).OpenAPIClientInfo(context.Background())
	if err != nil {
		t.Fatalf("bad dates must not error, got %v", err)
	}
	if !info.ExpiresAt.IsZero() || !info.IssuedAt.IsZero() {
		t.Errorf("bad dates should be zero time, got issued=%v expires=%v", info.IssuedAt, info.ExpiresAt)
	}
	if info.Active { // zero expiry is not after now
		t.Error("zero expiry must derive inactive")
	}
}

func TestOpenAPIClientInfo_NeverSurfacesSecret(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/openapi/client", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(clientBody("2026-06-27T14:29:00Z", "2027-06-27T14:29:00Z", "203.0.113.7")))
	})

	info, err := newOpenAPITestClient(t, mux).OpenAPIClientInfo(context.Background())
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	blob, _ := json.Marshal(info)
	if strings.Contains(string(blob), "SHOULD_NEVER_SURFACE") {
		t.Fatal("client secret leaked into OpenAPIClientInfo")
	}
}

func TestOpenAPIAllowedIPs_ReadsFromClientEndpoint(t *testing.T) {
	t.Parallel()

	called := false
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/openapi/client", func(w http.ResponseWriter, _ *http.Request) {
		called = true
		_, _ = w.Write([]byte(clientBody("2026-06-27T14:29:00Z", "2027-06-27T14:29:00Z", "203.0.113.7")))
	})
	// No /allowed-ips handler: if the code called that dead endpoint it would 404 → error.

	ips, err := newOpenAPITestClient(t, mux).OpenAPIAllowedIPs(context.Background())
	if err != nil {
		t.Fatalf("OpenAPIAllowedIPs error: %v", err)
	}
	if !called {
		t.Fatal("expected allowlist to come from /openapi/client")
	}
	if len(ips) != 1 || ips[0] != "203.0.113.7" {
		t.Errorf("AllowedIPs: got %v", ips)
	}
}

func TestOpenAPIClientInfo_AuthError(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/openapi/client", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	_, err := newOpenAPITestClient(t, mux).OpenAPIClientInfo(context.Background())
	if !IsAuthError(err) {
		t.Fatalf("expected auth error, got %v", err)
	}
}
