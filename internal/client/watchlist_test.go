package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
)

func newWatchlistFixturePath(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve test path")
	}
	return filepath.Join(filepath.Dir(filename), "..", "..", "fixtures", "responses", "auth-sanitized", "new-watchlists.json")
}

func newTestClientWithWatchlistFixture(t *testing.T) (*Client, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/new-watchlists":
			http.ServeFile(w, r, newWatchlistFixturePath(t))
		case "/api/v1/new-watchlists/groups":
			http.ServeFile(w, r, newWatchlistFixturePath(t))
		case "/api/v1/new-watchlists/groups/simple":
			http.ServeFile(w, r, newWatchlistFixturePath(t))
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	client := New(Config{
		HTTPClient:  server.Client(),
		APIBaseURL:  server.URL,
		InfoBaseURL: server.URL,
		CertBaseURL: server.URL,
		Session:     &session.Session{Cookies: map[string]string{"SESSION": "test-session"}},
	})
	return client, server
}

func TestListWatchlistFromFixtures(t *testing.T) {
	t.Parallel()
	client, server := newTestClientWithWatchlistFixture(t)
	defer server.Close()

	items, err := client.ListWatchlist(context.Background())
	if err != nil {
		t.Fatalf("ListWatchlist returned error: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected at least one watchlist item")
	}
	if items[0].Symbol == "" {
		t.Fatal("expected first watchlist item to have a symbol")
	}
}

func TestListWatchlistGroupsSymbolMapping(t *testing.T) {
	t.Parallel()
	client, server := newTestClientWithWatchlistFixture(t)
	defer server.Close()

	items, err := client.GetWatchlistGroupItems(context.Background(), 100)
	if err != nil {
		t.Fatalf("GetWatchlistGroupItems returned error: %v", err)
	}
	// First group "미국 빅테크" should have symbol "NVDA" (from symbol field, not code).
	if items[0].Symbol != "NVDA" {
		t.Fatalf("expected symbol 'NVDA', got %q", items[0].Symbol)
	}
	if items[1].Symbol != "GOOG" {
		t.Fatalf("expected symbol 'GOOG', got %q", items[1].Symbol)
	}
}

func TestListWatchlistGroupsSymbolFallback(t *testing.T) {
	t.Parallel()
	client, server := newTestClientWithWatchlistFixture(t)
	defer server.Close()

	items, err := client.GetWatchlistGroupItems(context.Background(), 300)
	if err != nil {
		t.Fatalf("GetWatchlistGroupItems returned error: %v", err)
	}
	// Third group "코드만 있는 폴더" has empty symbol → should fall back to code.
	if items[0].Symbol != "FALLBACK_CODE" {
		t.Fatalf("expected symbol fallback to code 'FALLBACK_CODE', got %q", items[0].Symbol)
	}
}

func TestListWatchlistGroupsCurrencyMapping(t *testing.T) {
	t.Parallel()
	client, server := newTestClientWithWatchlistFixture(t)
	defer server.Close()

	items, err := client.ListAllWatchlistItems(context.Background())
	if err != nil {
		t.Fatalf("ListAllWatchlistItems returned error: %v", err)
	}
	if items[0].Currency != "USD" {
		t.Fatalf("expected currency 'USD', got %q", items[0].Currency)
	}
	if items[2].Currency != "KRW" {
		t.Fatalf("expected currency 'KRW', got %q", items[2].Currency)
	}
}

func TestListWatchlistGroupsOrdering(t *testing.T) {
	t.Parallel()
	client, server := newTestClientWithWatchlistFixture(t)
	defer server.Close()

	groups, err := client.ListWatchlistGroups(context.Background())
	if err != nil {
		t.Fatalf("ListWatchlistGroups returned error: %v", err)
	}
	if len(groups) < 3 {
		t.Fatalf("expected at least 3 groups, got %d", len(groups))
	}
	// Groups should be sorted by ordering: -100, 0, 100.
	for i := 1; i < len(groups); i++ {
		if groups[i].Ordering < groups[i-1].Ordering {
			t.Fatalf("groups not sorted by ordering: groups[%d].Ordering=%d < groups[%d].Ordering=%d",
				i, groups[i].Ordering, i-1, groups[i-1].Ordering)
		}
	}
}

func TestGetWatchlistGroupItems(t *testing.T) {
	t.Parallel()
	client, server := newTestClientWithWatchlistFixture(t)
	defer server.Close()

	items100, err := client.GetWatchlistGroupItems(context.Background(), 100)
	if err != nil {
		t.Fatalf("GetWatchlistGroupItems(100) returned error: %v", err)
	}
	if len(items100) != 2 {
		t.Fatalf("expected 2 items for group 100, got %d", len(items100))
	}
	if items100[0].Symbol != "NVDA" {
		t.Fatalf("expected first item symbol 'NVDA', got %q", items100[0].Symbol)
	}

	// Test non-first group lookup to ensure ID-matching works regardless of slice order.
	items200, err := client.GetWatchlistGroupItems(context.Background(), 200)
	if err != nil {
		t.Fatalf("GetWatchlistGroupItems(200) returned error: %v", err)
	}
	if len(items200) != 1 {
		t.Fatalf("expected 1 item for group 200, got %d", len(items200))
	}
	if items200[0].Symbol != "005930" {
		t.Fatalf("expected item symbol '005930', got %q", items200[0].Symbol)
	}
}

func TestGetWatchlistGroupItemsNotFound(t *testing.T) {
	t.Parallel()
	// Server returns fixture with groups 100, 200, 300 — requesting 999 should fail.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return an empty watchlists array.
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"result":{"maxWatchlistCount":30,"watchlists":[]}}`)
	}))
	defer server.Close()

	client := New(Config{
		HTTPClient:  server.Client(),
		APIBaseURL:  server.URL,
		InfoBaseURL: server.URL,
		CertBaseURL: server.URL,
		Session:     &session.Session{Cookies: map[string]string{"SESSION": "test-session"}},
	})

	_, err := client.GetWatchlistGroupItems(context.Background(), 999)
	if err == nil {
		t.Fatal("expected error for non-existent group, got nil")
	}
}

func TestListAllWatchlistItems(t *testing.T) {
	t.Parallel()
	client, server := newTestClientWithWatchlistFixture(t)
	defer server.Close()

	items, err := client.ListAllWatchlistItems(context.Background())
	if err != nil {
		t.Fatalf("ListAllWatchlistItems returned error: %v", err)
	}
	// Fixture has 2 + 1 + 1 = 4 items across 3 groups.
	if len(items) != 4 {
		t.Fatalf("expected 4 items, got %d", len(items))
	}
	// Items should be ordered by group ordering, then item ordering.
	// Group 1 (ordering -100): NVDA, GOOG
	// Group 2 (ordering 0): 삼성전자
	// Group 3 (ordering 100): 심볼없는종목
	if items[0].Group != "미국 빅테크" {
		t.Fatalf("expected first item from group '미국 빅테크', got %q", items[0].Group)
	}
	if items[2].Group != "국내 대형주" {
		t.Fatalf("expected third item from group '국내 대형주', got %q", items[2].Group)
	}
}

func TestListAllWatchlistItemsEmpty(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"result":{"maxWatchlistCount":30,"watchlists":[]}}`)
	}))
	defer server.Close()

	client := New(Config{
		HTTPClient:  server.Client(),
		APIBaseURL:  server.URL,
		InfoBaseURL: server.URL,
		CertBaseURL: server.URL,
		Session:     &session.Session{Cookies: map[string]string{"SESSION": "test-session"}},
	})

	items, err := client.ListAllWatchlistItems(context.Background())
	if err != nil {
		t.Fatalf("ListAllWatchlistItems returned error: %v", err)
	}
	if items == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}

// Legacy helper — keep for backward compat with any other test that imports it.
func watchlistFixturePath(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve test path")
	}
	return filepath.Join(filepath.Dir(filename), "..", "..", "fixtures", "responses", "auth-sanitized", "asset-sections-v2.json")
}
