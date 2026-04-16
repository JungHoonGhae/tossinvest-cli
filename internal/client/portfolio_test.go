package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/junghoonkye/tossinvest-cli/internal/session"
)

func TestListPositionsFromFixtures(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/dashboard/asset/sections/all":
			http.ServeFile(w, r, portfolioFixturePath(t))
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := New(Config{
		HTTPClient:  server.Client(),
		APIBaseURL:  server.URL,
		InfoBaseURL: server.URL,
		CertBaseURL: server.URL,
		Session: &session.Session{
			Cookies: map[string]string{"SESSION": "test-session"},
		},
	})

	positions, err := client.ListPositions(context.Background())
	if err != nil {
		t.Fatalf("ListPositions returned error: %v", err)
	}
	if len(positions) == 0 {
		t.Fatal("expected at least one position")
	}
	if positions[0].Name == "" {
		t.Fatal("expected first position to have a name")
	}
}

func TestListPositionsReturnsCertEndpointAuthError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/dashboard/asset/sections/all":
			http.Error(w, "forbidden", http.StatusForbidden)
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := New(Config{
		HTTPClient:  server.Client(),
		APIBaseURL:  server.URL,
		InfoBaseURL: server.URL,
		CertBaseURL: server.URL,
		Session: &session.Session{
			Cookies: map[string]string{"SESSION": "test-session"},
		},
	})

	_, err := client.ListPositions(context.Background())
	if err == nil {
		t.Fatal("expected positions error")
	}

	var authErr *AuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected auth error, got %T", err)
	}
	if authErr.StatusCode != http.StatusForbidden {
		t.Fatalf("unexpected status code: %d", authErr.StatusCode)
	}
	if authErr.Endpoint != server.URL+"/api/v2/dashboard/asset/sections/all" {
		t.Fatalf("unexpected endpoint: %s", authErr.Endpoint)
	}
}

func portfolioFixturePath(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve test path")
	}
	return filepath.Join(filepath.Dir(filename), "..", "..", "fixtures", "responses", "auth-sanitized", "asset-sections-v2.json")
}
