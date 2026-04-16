package client

import (
	"context"
	"errors"
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

func TestValidateSessionRejectsHalfSession(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/my-assets/summaries/markets/all/overview":
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

	err := client.ValidateSession(context.Background())
	if err == nil {
		t.Fatal("expected half-session validation error")
	}
	if !IsAuthError(err) {
		t.Fatalf("expected auth error, got %v", err)
	}

	var authErr *AuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected underlying auth error, got %T", err)
	}
	if authErr.StatusCode != http.StatusForbidden {
		t.Fatalf("unexpected status code: %d", authErr.StatusCode)
	}
	if authErr.Endpoint != server.URL+"/api/v3/my-assets/summaries/markets/all/overview" {
		t.Fatalf("unexpected endpoint: %s", authErr.Endpoint)
	}
}

func TestValidateSessionDoesNotDependOnAccountList(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/my-assets/summaries/markets/all/overview":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"result":{"totalAssetAmount":1,"evaluatedProfitAmount":0,"profitRate":0,"overviewByMarket":{"us":{"market":"us","pendingBuyOrderAmount":0,"evaluatedAmount":1,"principalAmount":1,"evaluatedProfitAmount":0,"profitRate":0,"totalAssetAmount":1,"orderableAmount":{"krw":0,"usd":1}}}}}`))
		case "/api/v1/dashboard/common/cached-orderable-amount":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"result":{"orderableAmountKr":{"krw":0,"usd":0},"orderableAmountUs":{"krw":0,"usd":1}}}`))
		case "/api/v1/my-assets/summaries/markets/kr/withdrawable-amount":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"result":{"krw":0}}`))
		case "/api/v1/my-assets/summaries/markets/us/withdrawable-amount":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"result":{"usd":1}}`))
		case "/api/v2/dashboard/asset/sections/all":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"result":{"sections":[{"type":"SORTED_OVERVIEW","data":{"products":[{"marketType":"us","items":[{"stockCode":"NVDA","stockName":"NVIDIA","quantity":1,"currentPrice":{"usd":1},"purchasePrice":{"usd":1},"evaluatedAmount":{"usd":1},"profitLossAmount":{"usd":0},"profitLossRate":{"usd":0},"dailyProfitLossAmount":{"usd":0},"dailyProfitLossRate":{"usd":0},"marketCode":"NASD"}]}]}}]}}`))
		case "/api/v1/account/list":
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

	if err := client.ValidateSession(context.Background()); err != nil {
		t.Fatalf("expected validation success, got %v", err)
	}
}

func TestGetAccountSummaryReturnsCertEndpointAuthError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/my-assets/summaries/markets/all/overview":
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

	_, err := client.GetAccountSummary(context.Background())
	if err == nil {
		t.Fatal("expected summary error")
	}

	var authErr *AuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected auth error, got %T", err)
	}
	if authErr.StatusCode != http.StatusForbidden {
		t.Fatalf("unexpected status code: %d", authErr.StatusCode)
	}
	if authErr.Endpoint != server.URL+"/api/v3/my-assets/summaries/markets/all/overview" {
		t.Fatalf("unexpected endpoint: %s", authErr.Endpoint)
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
