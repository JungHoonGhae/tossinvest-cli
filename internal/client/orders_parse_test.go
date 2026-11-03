package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
)

func TestListPendingOrdersParsesStructuredFields(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/trading/orders/histories/all/pending" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
		  "result": [
		    {
		      "orderNo": 5,
		      "orderId": "opaque-order-id",
		      "stockCode": "US20220809012",
		      "tradeType": "buy",
		      "status": "체결대기",
		      "quantity": 1,
		      "pendingQuantity": 1,
		      "orderPrice": 500,
		      "orderedAt": "2026-03-11T19:10:26.582935Z"
		    }
		  ]
		}`))
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

	orders, err := client.ListPendingOrders(context.Background())
	if err != nil {
		t.Fatalf("ListPendingOrders returned error: %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("expected 1 pending order, got %d", len(orders))
	}
	if orders[0].ID != "5" {
		t.Fatalf("expected parsed order id 5, got %q", orders[0].ID)
	}
	if orders[0].Symbol != "US20220809012" {
		t.Fatalf("expected parsed symbol, got %q", orders[0].Symbol)
	}
	if orders[0].Price != 500 {
		t.Fatalf("expected parsed price 500, got %v", orders[0].Price)
	}
	if orders[0].Status != "체결대기" {
		t.Fatalf("expected parsed status, got %q", orders[0].Status)
	}
}

// TestListPendingOrdersKeepsPartialFieldsOnTypeMismatch guards issue #14: one
// corrupt field must not blank the whole entry — Go fills every decodable
// field before reporting UnmarshalTypeError, and duplicate-checking needs the
// surviving ID/symbol.
func TestListPendingOrdersKeepsPartialFieldsOnTypeMismatch(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/trading/orders/histories/all/pending" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
		  "result": [
		    {
		      "orderNo": 7,
		      "orderId": "opaque-7",
		      "stockCode": "US20220809012",
		      "tradeType": "buy",
		      "status": "체결대기",
		      "quantity": "one",
		      "pendingQuantity": 1,
		      "orderPrice": 700,
		      "orderedAt": "2026-03-11T19:10:26.582935Z"
		    }
		  ]
		}`))
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

	orders, err := client.ListPendingOrders(context.Background())
	if err != nil {
		t.Fatalf("ListPendingOrders returned error: %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("expected 1 pending order, got %d", len(orders))
	}
	got := orders[0]
	if got.ID != "7" || got.Symbol != "US20220809012" || got.Status != "체결대기" {
		t.Fatalf("partial decode must keep id/symbol/status, got %+v", got)
	}
	if len(got.Raw) == 0 {
		t.Fatal("raw payload must be kept for id-based lookup")
	}
}

// TestParseOrderTimeUsesKoreaLocation guards issue #14: zone-less ledger
// timestamps are KST, not the machine-local zone.
func TestParseOrderTimeUsesKoreaLocation(t *testing.T) {
	t.Parallel()
	got := parseOrderTime("2026-03-11T09:30:00")
	if got == nil {
		t.Fatal("expected parsed time, got nil")
	}
	if _, offset := got.Zone(); offset != 9*3600 {
		t.Fatalf("naive ledger time must be KST (+09:00), got %v", got)
	}
	if got.Hour() != 9 {
		t.Fatalf("wall clock must be preserved, got %v", got)
	}
	zoned := parseOrderTime("2026-03-11T09:30:00Z")
	if zoned == nil {
		t.Fatal("expected parsed zoned time, got nil")
	}
	if _, offset := zoned.Zone(); offset != 0 {
		t.Fatalf("explicit zone must win, got %v", zoned)
	}
}
