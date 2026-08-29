package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
)

// Locks the reverse-engineered new-watchlists mutation contract (method, path,
// body) so a future refactor can't silently drift.
func TestWatchlistMutationContract(t *testing.T) {
	var gotMethod, gotPath, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Respond to resolveProductCode lookup (POST /api/v2/search/stocks).
		if r.Method == "POST" && r.URL.Path == "/api/v2/search/stocks" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"result": map[string]any{
					"stocks": []map[string]string{{"stockCode": "US.AAPL", "stockName": "Apple Inc", "matchType": "SYMBOL"}},
				},
			})
			return
		}
		gotMethod, gotPath = r.Method, r.URL.Path
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Write([]byte(`{"result":{"id":123,"name":"x","type":"USER_MADE","itemCount":0}}`))
	}))
	defer srv.Close()

	c := New(Config{
		HTTPClient:  srv.Client(),
		CertBaseURL: srv.URL,
		InfoBaseURL: srv.URL,
		Session:     &session.Session{Cookies: map[string]string{"SESSION": "x"}, Headers: map[string]string{"X-XSRF-TOKEN": "t"}},
	})

	// create group
	if _, err := c.CreateWatchlistGroup(context.Background(), "내폴더"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if gotMethod != "POST" || gotPath != "/api/v1/new-watchlists/groups" {
		t.Errorf("create routing: %s %s", gotMethod, gotPath)
	}
	var cbody map[string]string
	_ = json.Unmarshal([]byte(gotBody), &cbody)
	if cbody["name"] != "내폴더" {
		t.Errorf("create body: %s", gotBody)
	}

	// rename → PATCH /groups/{id}
	if err := c.RenameWatchlistGroup(context.Background(), 123, "새이름"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if gotMethod != "PATCH" || gotPath != "/api/v1/new-watchlists/groups/123" {
		t.Errorf("rename routing: %s %s", gotMethod, gotPath)
	}

	// delete → DELETE /groups/{id}
	if err := c.DeleteWatchlistGroup(context.Background(), 123); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if gotMethod != "DELETE" || gotPath != "/api/v1/new-watchlists/groups/123" {
		t.Errorf("delete routing: %s %s", gotMethod, gotPath)
	}

	// add item → POST /items with watchlistIds (plural, array)
	if err := c.AddWatchlistItem(context.Background(), 456, "AAPL"); err != nil {
		t.Fatalf("add item: %v", err)
	}
	if gotMethod != "POST" || gotPath != "/api/v1/new-watchlists/items" {
		t.Errorf("add routing: %s %s", gotMethod, gotPath)
	}
	var addBody map[string]any
	_ = json.Unmarshal([]byte(gotBody), &addBody)
	// watchlistIds must be present as an array (not watchlistId singular).
	ids, ok := addBody["watchlistIds"].([]any)
	if !ok || len(ids) != 1 {
		t.Fatalf("add body: expected watchlistIds array with 1 element, got: %s", gotBody)
	}
	if int64(ids[0].(float64)) != 456 {
		t.Errorf("add body: expected watchlistIds=[456], got: %s", gotBody)
	}
	if _, hasSingular := addBody["watchlistId"]; hasSingular {
		t.Errorf("add body: must NOT contain singular watchlistId field, got: %s", gotBody)
	}

	// remove item → POST /items/remove with watchlistId (singular, number)
	if err := c.RemoveWatchlistItem(context.Background(), 456, "AAPL"); err != nil {
		t.Fatalf("remove item: %v", err)
	}
	if gotMethod != "POST" || gotPath != "/api/v1/new-watchlists/items/remove" {
		t.Errorf("remove routing: %s %s", gotMethod, gotPath)
	}
	var rmBody map[string]any
	_ = json.Unmarshal([]byte(gotBody), &rmBody)
	// watchlistId must be present as a number (not watchlistIds plural).
	wid, ok := rmBody["watchlistId"].(float64)
	if !ok || int64(wid) != 456 {
		t.Fatalf("remove body: expected watchlistId=456, got: %s", gotBody)
	}
	if _, hasPlural := rmBody["watchlistIds"]; hasPlural {
		t.Errorf("remove body: must NOT contain plural watchlistIds field, got: %s", gotBody)
	}
}

// XSRF header must ride along on mutations (via applySession session.Headers).
func TestWatchlistMutationSendsXSRF(t *testing.T) {
	var xsrf string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		xsrf = r.Header.Get("X-XSRF-TOKEN")
		w.Write([]byte(`{"result":{"id":1}}`))
	}))
	defer srv.Close()
	c := New(Config{
		HTTPClient:  srv.Client(),
		CertBaseURL: srv.URL,
		Session:     &session.Session{Cookies: map[string]string{"SESSION": "x"}, Headers: map[string]string{"X-XSRF-TOKEN": "tok-123"}},
	})
	if _, err := c.CreateWatchlistGroup(context.Background(), "f"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if xsrf != "tok-123" {
		t.Errorf("expected XSRF header, got %q", xsrf)
	}
}
