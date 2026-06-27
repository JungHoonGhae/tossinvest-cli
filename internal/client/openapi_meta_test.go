package client

import (
	"context"
	"net/http"
	"net/http/httptest"
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

func TestOpenAPIClientInfo_ParsesAllFields(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/openapi/client", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"result": {
				"status": "ACTIVE",
				"issuedAt": "2025-01-15T09:00:00Z",
				"expiresAt": "2026-01-15T09:00:00Z",
				"active": true
			}
		}`))
	})

	c := newOpenAPITestClient(t, mux)
	info, err := c.OpenAPIClientInfo(context.Background())
	if err != nil {
		t.Fatalf("OpenAPIClientInfo error: %v", err)
	}
	if info.Status != "ACTIVE" {
		t.Errorf("Status: got %q, want %q", info.Status, "ACTIVE")
	}
	if !info.Active {
		t.Error("expected Active=true")
	}
	wantIssued := time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC)
	if !info.IssuedAt.Equal(wantIssued) {
		t.Errorf("IssuedAt: got %v, want %v", info.IssuedAt, wantIssued)
	}
	wantExpires := time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)
	if !info.ExpiresAt.Equal(wantExpires) {
		t.Errorf("ExpiresAt: got %v, want %v", info.ExpiresAt, wantExpires)
	}
}

func TestOpenAPIClientInfo_DerivesActiveFromStatus(t *testing.T) {
	t.Parallel()

	// No "active" boolean field — derive from status == "ACTIVE"
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/openapi/client", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"result": {
				"status": "ACTIVE",
				"createdAt": "2025-03-01T00:00:00Z",
				"expiredAt": "2026-03-01T00:00:00Z"
			}
		}`))
	})

	c := newOpenAPITestClient(t, mux)
	info, err := c.OpenAPIClientInfo(context.Background())
	if err != nil {
		t.Fatalf("OpenAPIClientInfo error: %v", err)
	}
	if !info.Active {
		t.Error("expected Active=true derived from status ACTIVE")
	}
	wantIssued := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	if !info.IssuedAt.Equal(wantIssued) {
		t.Errorf("IssuedAt via createdAt fallback: got %v, want %v", info.IssuedAt, wantIssued)
	}
	wantExpires := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	if !info.ExpiresAt.Equal(wantExpires) {
		t.Errorf("ExpiresAt via expiredAt fallback: got %v, want %v", info.ExpiresAt, wantExpires)
	}
}

func TestOpenAPIClientInfo_BadDateDoesNotError(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/openapi/client", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result": {"status": "INACTIVE", "issuedAt": "not-a-date"}}`))
	})

	c := newOpenAPITestClient(t, mux)
	info, err := c.OpenAPIClientInfo(context.Background())
	if err != nil {
		t.Fatalf("OpenAPIClientInfo should not error on unparseable date: %v", err)
	}
	if !info.IssuedAt.IsZero() {
		t.Errorf("expected zero IssuedAt for bad date, got %v", info.IssuedAt)
	}
	if info.Active {
		t.Error("expected Active=false for status INACTIVE")
	}
}

func TestOpenAPIAllowedIPs_StringSlice(t *testing.T) {
	t.Parallel()

	// Primary shape: result is []string
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/openapi/client/allowed-ips", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result": ["203.0.113.7", "203.0.113.8"]}`))
	})

	c := newOpenAPITestClient(t, mux)
	ips, err := c.OpenAPIAllowedIPs(context.Background())
	if err != nil {
		t.Fatalf("OpenAPIAllowedIPs error: %v", err)
	}
	if len(ips) != 2 {
		t.Fatalf("expected 2 IPs, got %d", len(ips))
	}
	if ips[0] != "203.0.113.7" {
		t.Errorf("ips[0]: got %q, want %q", ips[0], "203.0.113.7")
	}
	if ips[1] != "203.0.113.8" {
		t.Errorf("ips[1]: got %q, want %q", ips[1], "203.0.113.8")
	}
}

func TestOpenAPIAllowedIPs_ObjectSlice(t *testing.T) {
	t.Parallel()

	// Alternate shape: result is [{"ip": "..."}]
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/openapi/client/allowed-ips", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result": [{"ip": "203.0.113.9"}, {"ip": "203.0.113.10"}]}`))
	})

	c := newOpenAPITestClient(t, mux)
	ips, err := c.OpenAPIAllowedIPs(context.Background())
	if err != nil {
		t.Fatalf("OpenAPIAllowedIPs error: %v", err)
	}
	if len(ips) != 2 {
		t.Fatalf("expected 2 IPs, got %d", len(ips))
	}
	if ips[0] != "203.0.113.9" {
		t.Errorf("ips[0]: got %q, want %q", ips[0], "203.0.113.9")
	}
}

func TestOpenAPIClientInfo_RequiresSession(t *testing.T) {
	t.Parallel()

	c := New(Config{})
	_, err := c.OpenAPIClientInfo(context.Background())
	if !IsAuthError(err) {
		t.Fatalf("expected auth error for no-session, got %v", err)
	}
}

func TestOpenAPIAllowedIPs_RequiresSession(t *testing.T) {
	t.Parallel()

	c := New(Config{})
	_, err := c.OpenAPIAllowedIPs(context.Background())
	if !IsAuthError(err) {
		t.Fatalf("expected auth error for no-session, got %v", err)
	}
}
