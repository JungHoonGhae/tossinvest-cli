package official

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

// TestAdaptOrderUnit verifies the pure adapter for a single Order.
func TestAdaptOrderUnit(t *testing.T) {
	raw := apiOrder{
		OrderID:   "abc123",
		Symbol:    "005930",
		Side:      "BUY",
		OrderType: "LIMIT",
		Status:    "OPEN",
		Quantity:  "10",
		Price:     "70000",
		Currency:  "KRW",
		OrderedAt: "2026-03-29T09:30:00+09:00",
		Execution: apiOrderExecution{
			FilledQuantity:     "0",
			AverageFilledPrice: "",
		},
	}

	got := adaptOrder(raw)

	if got.ID != "abc123" {
		t.Fatalf("ID: want abc123, got %q", got.ID)
	}
	if got.Symbol != "005930" {
		t.Fatalf("Symbol: want 005930, got %q", got.Symbol)
	}
	if got.Side != "BUY" {
		t.Fatalf("Side: want BUY, got %q", got.Side)
	}
	if got.Status != "OPEN" {
		t.Fatalf("Status: want OPEN, got %q", got.Status)
	}
	if got.Quantity != 10 {
		t.Fatalf("Quantity: want 10, got %v", got.Quantity)
	}
	if got.Price != 70000 {
		t.Fatalf("Price: want 70000, got %v", got.Price)
	}
	if got.FilledQuantity != 0 {
		t.Fatalf("FilledQuantity: want 0, got %v", got.FilledQuantity)
	}
	if got.AverageExecutionPrice != 0 {
		t.Fatalf("AverageExecutionPrice: want 0 (null), got %v", got.AverageExecutionPrice)
	}
	if got.OrderDate != "2026-03-29" {
		t.Fatalf("OrderDate: want 2026-03-29, got %q", got.OrderDate)
	}
	if got.SubmittedAt == nil {
		t.Fatal("SubmittedAt: expected non-nil for valid orderedAt")
	}
	// 09:30 KST = 00:30 UTC on same calendar day.
	wantUTC := time.Date(2026, 3, 29, 0, 30, 0, 0, time.UTC)
	if !got.SubmittedAt.UTC().Equal(wantUTC) {
		t.Fatalf("SubmittedAt UTC: want %v, got %v", wantUTC, got.SubmittedAt.UTC())
	}
	// Fields not in /orders response.
	if got.Name != "" {
		t.Fatalf("Name: expected empty, got %q", got.Name)
	}
	if got.Market != "" {
		t.Fatalf("Market: expected empty, got %q", got.Market)
	}
}

// TestAdaptOrderFilledExecution verifies a fully-filled order adapter.
func TestAdaptOrderFilledExecution(t *testing.T) {
	raw := apiOrder{
		OrderID:   "xyz789",
		Symbol:    "AAPL",
		Side:      "SELL",
		Status:    "CLOSED",
		Quantity:  "5",
		Price:     "200",
		OrderedAt: "2026-03-28T22:00:00+00:00",
		Execution: apiOrderExecution{
			FilledQuantity:     "5",
			AverageFilledPrice: "201.5",
		},
	}

	got := adaptOrder(raw)

	if got.FilledQuantity != 5 {
		t.Fatalf("FilledQuantity: want 5, got %v", got.FilledQuantity)
	}
	if got.AverageExecutionPrice != 201.5 {
		t.Fatalf("AverageExecutionPrice: want 201.5, got %v", got.AverageExecutionPrice)
	}
}

// TestAdaptOrderNoOrderedAt verifies nil SubmittedAt when orderedAt is empty.
func TestAdaptOrderNoOrderedAt(t *testing.T) {
	raw := apiOrder{OrderID: "x", Symbol: "X", OrderedAt: ""}
	got := adaptOrder(raw)
	if got.SubmittedAt != nil {
		t.Fatalf("SubmittedAt: expected nil for empty orderedAt, got %v", got.SubmittedAt)
	}
	if got.OrderDate != "" {
		t.Fatalf("OrderDate: expected empty for empty orderedAt, got %q", got.OrderDate)
	}
}

// TestAdaptOrderMillisecondOrderedAt guards the 2026-08-19 spec change: orderedAt·
// filledAt examples gained a `.SSS` fraction. time.RFC3339 as a *parse* layout accepts
// it, so this is the check that fails if someone swaps in a stricter layout.
func TestAdaptOrderMillisecondOrderedAt(t *testing.T) {
	raw := apiOrder{OrderID: "x", Symbol: "005930", OrderedAt: "2026-03-29T09:30:00.123+09:00"}
	got := adaptOrder(raw)
	if got.SubmittedAt == nil {
		t.Fatal("SubmittedAt: expected non-nil for millisecond orderedAt")
	}
	wantUTC := time.Date(2026, 3, 29, 0, 30, 0, 123000000, time.UTC)
	if !got.SubmittedAt.UTC().Equal(wantUTC) {
		t.Fatalf("SubmittedAt UTC: want %v, got %v", wantUTC, got.SubmittedAt.UTC())
	}
	if got.OrderDate != "2026-03-29" {
		t.Fatalf("OrderDate: want 2026-03-29, got %q", got.OrderDate)
	}
}

// TestAdaptOrdersBatch verifies the slice adapter.
func TestAdaptOrdersBatch(t *testing.T) {
	raw := []apiOrder{
		{OrderID: "a", Symbol: "A", Quantity: "1"},
		{OrderID: "b", Symbol: "B", Quantity: "2"},
	}
	got := adaptOrders(raw)
	if len(got) != 2 {
		t.Fatalf("len: want 2, got %d", len(got))
	}
	if got[0].ID != "a" || got[1].ID != "b" {
		t.Fatalf("IDs: want a,b got %q,%q", got[0].ID, got[1].ID)
	}
}

// TestAdaptOrdersEmpty verifies that nil input returns a non-nil empty slice.
func TestAdaptOrdersEmpty(t *testing.T) {
	got := adaptOrders(nil)
	if got == nil {
		t.Fatal("expected non-nil slice")
	}
	if len(got) != 0 {
		t.Fatalf("want 0, got %d", len(got))
	}
}

// TestOrdersIntegration tests Orders() against an httptest server.
func TestOrdersIntegration(t *testing.T) {
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/orders":
			gotHeader = r.Header.Get("X-Tossinvest-Account")
			q := r.URL.Query()
			if q.Get("status") != "OPEN" {
				t.Errorf("status: want OPEN, got %q", q.Get("status"))
			}
			_, _ = w.Write([]byte(`{"result":{"orders":[{"orderId":"abc123","symbol":"005930","side":"BUY","orderType":"LIMIT","timeInForce":"DAY","status":"OPEN","quantity":"10","price":"70000","currency":"KRW","orderedAt":"2026-03-29T09:30:00+09:00","canceledAt":null,"orderAmount":null,"execution":{"filledQuantity":"0","averageFilledPrice":null,"filledAmount":null,"commission":null,"tax":null,"filledAt":null,"settlementDate":null}}],"nextCursor":null,"hasNext":false}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := New(
		Credentials{APIKey: "k", SecretKey: "s"},
		filepath.Join(t.TempDir(), "t.json"),
		WithBaseURL(srv.URL),
		WithHTTPClient(srv.Client()),
		WithAccountSeq(3),
	)

	got, err := c.Orders(context.Background(), OrdersFilter{Status: "OPEN"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Orders) != 1 {
		t.Fatalf("want 1 order, got %d", len(got.Orders))
	}
	if got.Orders[0].ID != "abc123" {
		t.Fatalf("ID: want abc123, got %q", got.Orders[0].ID)
	}
	if got.Orders[0].Quantity != 10 {
		t.Fatalf("Quantity: want 10, got %v", got.Orders[0].Quantity)
	}
	if gotHeader != "3" {
		t.Fatalf("X-Tossinvest-Account: want 3, got %q", gotHeader)
	}
}

// 서버가 다음 페이지가 있다고 말하면 그 사실이 호출자에게 도달해야 한다.
// 예전에는 Orders() 가 nextCursor/hasNext 를 파싱해놓고 버려서, 다건 조회가
// 첫 페이지만 조용히 반환됐다 — 에이전트는 그게 전부인 줄 안다.
// spec v1.2.5 에서 status=CLOSED 가 400 대신 페이징 응답을 주기 시작하며
// 이 경로가 처음 실사용된다.
func TestOrdersSurfacesPaginationMetadata(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/orders":
			q := r.URL.Query()
			if q.Get("status") != "CLOSED" {
				t.Errorf("status: want CLOSED, got %q", q.Get("status"))
			}
			if q.Get("limit") != "1" {
				t.Errorf("limit: want 1, got %q", q.Get("limit"))
			}
			_, _ = w.Write([]byte(`{"result":{"orders":[{"orderId":"abc123","symbol":"005930","side":"BUY","orderType":"LIMIT","timeInForce":"DAY","status":"CLOSED","quantity":"10","price":"70000","currency":"KRW","orderedAt":"2026-03-29T09:30:00+09:00","canceledAt":null,"orderAmount":null,"execution":{"filledQuantity":"10","averageFilledPrice":"70000","filledAmount":"700000","commission":null,"tax":null,"filledAt":null,"settlementDate":null}}],"nextCursor":"CURSOR-2","hasNext":true}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := New(
		Credentials{APIKey: "k", SecretKey: "s"},
		filepath.Join(t.TempDir(), "t.json"),
		WithBaseURL(srv.URL),
		WithHTTPClient(srv.Client()),
		WithAccountSeq(1),
	)

	got, err := c.Orders(context.Background(), OrdersFilter{Status: "CLOSED", Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Orders) != 1 {
		t.Fatalf("want 1 order, got %d", len(got.Orders))
	}
	if !got.HasNext {
		t.Error("HasNext 가 false — 다음 페이지가 있다는 사실이 사라졌다")
	}
	if got.NextCursor != "CURSOR-2" {
		t.Errorf("NextCursor: want CURSOR-2, got %q", got.NextCursor)
	}
}

// 마지막 페이지는 커서가 비어야 한다. 커서가 남으면 호출자가 무한 루프에 빠진다.
func TestOrdersLastPageHasNoCursor(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/orders":
			_, _ = w.Write([]byte(`{"result":{"orders":[],"nextCursor":null,"hasNext":false}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := New(
		Credentials{APIKey: "k", SecretKey: "s"},
		filepath.Join(t.TempDir(), "t.json"),
		WithBaseURL(srv.URL),
		WithHTTPClient(srv.Client()),
		WithAccountSeq(1),
	)

	got, err := c.Orders(context.Background(), OrdersFilter{Status: "CLOSED"})
	if err != nil {
		t.Fatal(err)
	}
	if got.HasNext || got.NextCursor != "" {
		t.Errorf("마지막 페이지인데 다음 페이지가 있다고 한다: HasNext=%v NextCursor=%q", got.HasNext, got.NextCursor)
	}
}

// TestOrdersFilterEmpty verifies that Orders() with zero filter omits all params.
func TestOrdersFilterEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/orders":
			q := r.URL.Query()
			for _, key := range []string{"status", "symbol", "from", "to", "cursor", "limit"} {
				if q.Get(key) != "" {
					t.Errorf("param %q: expected empty/absent, got %q", key, q.Get(key))
				}
			}
			_, _ = w.Write([]byte(`{"result":{"orders":[],"nextCursor":null,"hasNext":false}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := New(
		Credentials{APIKey: "k", SecretKey: "s"},
		filepath.Join(t.TempDir(), "t.json"),
		WithBaseURL(srv.URL),
		WithHTTPClient(srv.Client()),
		WithAccountSeq(1), // skip lazy fetch; test focuses on query param absence
	)

	got, err := c.Orders(context.Background(), OrdersFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Orders) != 0 {
		t.Fatalf("want 0 orders, got %d", len(got.Orders))
	}
}

// ---------------------------------------------------------------------------
// OrderByID integration tests
// ---------------------------------------------------------------------------

// TestOrderByIDIntegration tests OrderByID() against an httptest server.
func TestOrderByIDIntegration(t *testing.T) {
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/orders/abc123":
			gotHeader = r.Header.Get("X-Tossinvest-Account")
			_, _ = w.Write([]byte(`{"result":{"orderId":"abc123","symbol":"005930","side":"BUY","orderType":"LIMIT","timeInForce":"DAY","status":"OPEN","quantity":"10","price":"70000","currency":"KRW","orderedAt":"2026-03-29T09:30:00+09:00","canceledAt":null,"orderAmount":null,"execution":{"filledQuantity":"0","averageFilledPrice":null,"filledAmount":null,"commission":null,"tax":null,"filledAt":null,"settlementDate":null}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := New(
		Credentials{APIKey: "k", SecretKey: "s"},
		filepath.Join(t.TempDir(), "t.json"),
		WithBaseURL(srv.URL),
		WithHTTPClient(srv.Client()),
		WithAccountSeq(5),
	)

	got, err := c.OrderByID(context.Background(), "abc123")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "abc123" {
		t.Fatalf("ID: want abc123, got %q", got.ID)
	}
	if got.Symbol != "005930" {
		t.Fatalf("Symbol: want 005930, got %q", got.Symbol)
	}
	if got.Price != 70000 {
		t.Fatalf("Price: want 70000, got %v", got.Price)
	}
	if gotHeader != "5" {
		t.Fatalf("X-Tossinvest-Account: want 5, got %q", gotHeader)
	}
}
