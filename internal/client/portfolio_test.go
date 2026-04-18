package client

import (
	"context"
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

func TestListPositionsAppliesCertHeaders(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/dashboard/asset/sections/all":
			if got := r.Header.Get("Browser-Tab-Id"); got != "browser-tab-test" {
				t.Fatalf("unexpected Browser-Tab-Id: %q", got)
			}
			if got := r.Header.Get("App-Version"); got != "v260311.1636" {
				t.Fatalf("unexpected App-Version: %q", got)
			}
			if got := r.Header.Get("X-Tossinvest-Account"); got != "1" {
				t.Fatalf("unexpected X-Tossinvest-Account: %q", got)
			}
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
			Headers: map[string]string{
				"Browser-Tab-Id": "browser-tab-test",
				"App-Version":    "v260311.1636",
			},
		},
	})

	if _, err := client.ListPositions(context.Background()); err != nil {
		t.Fatalf("ListPositions returned error: %v", err)
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
