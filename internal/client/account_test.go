package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/junghoonkye/tossinvest-cli/internal/session"
)

func TestAuthenticatedAccountMethodsFromFixtures(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got != defaultBrowserUserAgent {
			t.Fatalf("unexpected user agent: %q", got)
		}
		fixturePath := authenticatedFixturePathForRequest(t, r.URL.Path)
		http.ServeFile(w, r, fixturePath)
	}))
	defer server.Close()

	client := New(Config{
		HTTPClient:  server.Client(),
		APIBaseURL:  server.URL,
		InfoBaseURL: server.URL,
		CertBaseURL: server.URL,
		Session: &session.Session{
			Cookies: map[string]string{
				"SESSION": "test-session",
			},
		},
	})

	accounts, primaryKey, err := client.ListAccounts(context.Background())
	if err != nil {
		t.Fatalf("ListAccounts returned error: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("unexpected account count: %d", len(accounts))
	}
	if primaryKey != "1" {
		t.Fatalf("unexpected primary key: %s", primaryKey)
	}
	if !accounts[0].Primary {
		t.Fatal("expected first account to be primary")
	}

	summary, err := client.GetAccountSummary(context.Background())
	if err != nil {
		t.Fatalf("GetAccountSummary returned error: %v", err)
	}
	if summary.TotalAssetAmount == 0 {
		t.Fatal("expected non-zero total asset amount")
	}
	if _, ok := summary.Markets["us"]; !ok {
		t.Fatal("expected us market summary")
	}

	orders, err := client.ListPendingOrders(context.Background())
	if err != nil {
		t.Fatalf("ListPendingOrders returned error: %v", err)
	}
	if len(orders) != 0 {
		t.Fatalf("expected zero pending orders, got %d", len(orders))
	}
}

func TestValidateSessionClassifiesUnauthorized(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
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

	err := client.ValidateSession(context.Background())
	if !IsAuthError(err) {
		t.Fatalf("expected auth error, got %v", err)
	}
}

func TestValidateSessionRequiresStoredSession(t *testing.T) {
	t.Parallel()

	client := New(Config{})
	err := client.ValidateSession(context.Background())
	if !IsAuthError(err) {
		t.Fatalf("expected auth error, got %v", err)
	}
}

func TestApplySessionPreservesExplicitUserAgent(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "https://example.com", nil)
	req.Header.Set("User-Agent", "custom-agent/1.0")

	client := New(Config{})
	client.applySession(req)

	if got := req.Header.Get("User-Agent"); got != "custom-agent/1.0" {
		t.Fatalf("expected explicit user agent to be preserved, got %q", got)
	}
}

func TestGetAccountSummaryAppliesCertHeaders(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/my-assets/summaries/markets/all/overview",
			"/api/v1/dashboard/common/cached-orderable-amount":
			if got := r.Header.Get("Browser-Tab-Id"); got != "browser-tab-test" {
				t.Fatalf("unexpected Browser-Tab-Id: %q", got)
			}
			if got := r.Header.Get("App-Version"); got != "v260311.1636" {
				t.Fatalf("unexpected App-Version: %q", got)
			}
			if got := r.Header.Get("X-Tossinvest-Account"); got != "1" {
				t.Fatalf("unexpected X-Tossinvest-Account: %q", got)
			}
			http.ServeFile(w, r, authenticatedFixturePathForRequest(t, r.URL.Path))
		case "/api/v1/my-assets/summaries/markets/kr/withdrawable-amount",
			"/api/v1/my-assets/summaries/markets/us/withdrawable-amount":
			http.ServeFile(w, r, authenticatedFixturePathForRequest(t, r.URL.Path))
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

	if _, err := client.GetAccountSummary(context.Background()); err != nil {
		t.Fatalf("GetAccountSummary returned error: %v", err)
	}
}

func authenticatedFixturePathForRequest(t *testing.T, path string) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve test path")
	}

	root := filepath.Join(filepath.Dir(filename), "..", "..", "fixtures", "responses", "auth-sanitized")

	switch path {
	case "/api/v1/account/list":
		return mustFixturePath(t, filepath.Join(root, "account-list.json"))
	case "/api/v3/my-assets/summaries/markets/all/overview":
		return mustFixturePath(t, filepath.Join(root, "asset-overview.json"))
	case "/api/v1/dashboard/common/cached-orderable-amount":
		return mustFixturePath(t, filepath.Join(root, "cached-orderable-amount.json"))
	case "/api/v1/my-assets/summaries/markets/kr/withdrawable-amount":
		return mustFixturePath(t, filepath.Join(root, "withdrawable-kr.json"))
	case "/api/v1/my-assets/summaries/markets/us/withdrawable-amount":
		return mustFixturePath(t, filepath.Join(root, "withdrawable-us.json"))
	case "/api/v1/trading/orders/histories/all/pending":
		return mustFixturePath(t, filepath.Join(root, "pending-orders.json"))
	default:
		t.Fatalf("unexpected request path: %s", path)
		return ""
	}
}

func mustFixturePath(t *testing.T, path string) string {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("fixture missing: %s: %v", path, err)
	}
	return path
}
