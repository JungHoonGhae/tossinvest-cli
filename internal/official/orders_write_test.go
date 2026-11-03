package official

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/orderintent"
)

// ---------------------------------------------------------------------------
// buildOrderCreate unit tests
// ---------------------------------------------------------------------------

// TestBuildOrderCreateFractionalBuy verifies the amount-based variant (variant1)
// for fractional BUY. Expects: orderAmount set, no quantity field, orderType=MARKET.
func TestBuildOrderCreateFractionalBuy(t *testing.T) {
	intent := orderintent.PlaceIntent{
		Symbol:       "TSLA",
		Market:       "us",
		Side:         "buy",
		Fractional:   true,
		Amount:       100.5,
		CurrencyMode: "USD",
	}
	body, err := buildOrderCreate(intent)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(body)
	got := string(b)
	for _, want := range []string{
		`"symbol":"TSLA"`,
		`"side":"BUY"`,
		`"orderType":"MARKET"`,
		`"orderAmount":"100.5"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %s in %s", want, got)
		}
	}
	if strings.Contains(got, `"quantity"`) {
		t.Fatalf("fractional buy must not use quantity; got %s", got)
	}
}

// TestBuildOrderCreateFractionalSell verifies the quantity-based variant (variant0)
// for fractional SELL. Expects: quantity decimal string, orderType=MARKET, no orderAmount.
func TestBuildOrderCreateFractionalSell(t *testing.T) {
	intent := orderintent.PlaceIntent{
		Symbol:     "TSLA",
		Market:     "us",
		Side:       "sell",
		Fractional: true,
		Quantity:   0.5,
	}
	body, err := buildOrderCreate(intent)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(body)
	got := string(b)
	for _, want := range []string{
		`"symbol":"TSLA"`,
		`"side":"SELL"`,
		`"orderType":"MARKET"`,
		`"quantity":"0.5"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %s in %s", want, got)
		}
	}
	if strings.Contains(got, `"orderAmount"`) {
		t.Fatalf("fractional sell must not use orderAmount; got %s", got)
	}
}

// TestBuildOrderCreateLimit verifies the LIMIT quantity-based variant.
// Expects: price + quantity + timeInForce=DAY, no orderAmount.
func TestBuildOrderCreateLimit(t *testing.T) {
	intent := orderintent.PlaceIntent{
		Symbol:    "005930",
		Side:      "buy",
		OrderType: "limit",
		Quantity:  10,
		Price:     70000,
	}
	body, err := buildOrderCreate(intent)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(body)
	got := string(b)
	for _, want := range []string{
		`"symbol":"005930"`,
		`"side":"BUY"`,
		`"orderType":"LIMIT"`,
		`"quantity":"10"`,
		`"price":"70000"`,
		`"timeInForce":"DAY"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %s in %s", want, got)
		}
	}
	if strings.Contains(got, `"orderAmount"`) {
		t.Fatalf("limit order must not use orderAmount; got %s", got)
	}
}

// TestBuildOrderCreateMarket verifies the MARKET quantity-based variant.
// Expects: quantity only, no price, no timeInForce, no orderAmount.
func TestBuildOrderCreateMarket(t *testing.T) {
	intent := orderintent.PlaceIntent{
		Symbol:    "TSLA",
		Side:      "sell",
		OrderType: "market",
		Quantity:  5,
	}
	body, err := buildOrderCreate(intent)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(body)
	got := string(b)
	for _, want := range []string{
		`"symbol":"TSLA"`,
		`"side":"SELL"`,
		`"orderType":"MARKET"`,
		`"quantity":"5"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %s in %s", want, got)
		}
	}
	if strings.Contains(got, `"price"`) {
		t.Fatalf("market order must not include price; got %s", got)
	}
	if strings.Contains(got, `"orderAmount"`) {
		t.Fatalf("market order must not include orderAmount; got %s", got)
	}
}

// ---------------------------------------------------------------------------
// PlaceOrder integration test
// ---------------------------------------------------------------------------

// TestPlaceOrder verifies POST body JSON and X-Tossinvest-Account header via httptest.
func TestPlaceOrder(t *testing.T) {
	var gotBody string
	var gotAcctHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/orders":
			gotAcctHeader = r.Header.Get("X-Tossinvest-Account")
			var m map[string]any
			_ = json.NewDecoder(r.Body).Decode(&m)
			b, _ := json.Marshal(m)
			gotBody = string(b)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"result":{"orderId":"O1","clientOrderId":null}}`))
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
		WithAccountSeq(7),
	)

	intent := orderintent.PlaceIntent{
		Symbol:       "TSLA",
		Market:       "us",
		Side:         "buy",
		OrderType:    "limit",
		Quantity:     3,
		Price:        150.25,
		CurrencyMode: "USD",
	}
	res, err := c.PlaceOrder(context.Background(), intent)
	if err != nil {
		t.Fatal(err)
	}
	if res.Kind != "place" {
		t.Fatalf("Kind: want place, got %q", res.Kind)
	}
	if res.Status != "accepted" {
		t.Fatalf("Status: want accepted, got %q", res.Status)
	}
	if res.OrderID != "O1" {
		t.Fatalf("OrderID: want O1, got %q", res.OrderID)
	}
	// intent summary fields echoed back onto the result.
	if res.Symbol != "TSLA" {
		t.Fatalf("Symbol: want TSLA, got %q", res.Symbol)
	}
	if res.Market != "us" {
		t.Fatalf("Market: want us, got %q", res.Market)
	}
	if res.Quantity != 3 {
		t.Fatalf("Quantity: want 3, got %v", res.Quantity)
	}
	if res.Price != 150.25 {
		t.Fatalf("Price: want 150.25, got %v", res.Price)
	}
	if gotAcctHeader != "7" {
		t.Fatalf("X-Tossinvest-Account: want 7, got %q", gotAcctHeader)
	}
	for _, want := range []string{
		`"symbol":"TSLA"`,
		`"side":"BUY"`,
		`"orderType":"LIMIT"`,
		`"quantity":"3"`,
		`"price":"150.25"`,
	} {
		if !strings.Contains(gotBody, want) {
			t.Fatalf("POST body missing %s; got %s", want, gotBody)
		}
	}
}

// ---------------------------------------------------------------------------
// CancelOrder integration test
// ---------------------------------------------------------------------------

// TestCancelOrder verifies POST to correct path and X-Tossinvest-Account header.
// The intent symbol is verified against the live order first (issue #6).
func TestCancelOrder(t *testing.T) {
	var gotPath string
	var gotAcctHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/orders/order-abc-123":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"result":{"orderId":"order-abc-123","symbol":"005930","side":"BUY","orderType":"LIMIT","status":"OPEN","quantity":"10","price":"70000"}}`))
		default:
			if strings.HasSuffix(r.URL.Path, "/cancel") {
				gotPath = r.URL.Path
				gotAcctHeader = r.Header.Get("X-Tossinvest-Account")
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"result":{"orderId":"C1"}}`))
			} else {
				http.NotFound(w, r)
			}
		}
	}))
	defer srv.Close()

	c := New(
		Credentials{APIKey: "k", SecretKey: "s"},
		filepath.Join(t.TempDir(), "t.json"),
		WithBaseURL(srv.URL),
		WithHTTPClient(srv.Client()),
		WithAccountSeq(9),
	)

	res, err := c.CancelOrder(context.Background(), orderintent.CancelIntent{OrderID: "order-abc-123", Symbol: "005930"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Kind != "cancel" {
		t.Fatalf("Kind: want cancel, got %q", res.Kind)
	}
	if res.Status != "accepted" {
		t.Fatalf("Status: want accepted, got %q", res.Status)
	}
	if res.OrderID != "C1" {
		t.Fatalf("OrderID: want C1, got %q", res.OrderID)
	}
	wantPath := "/api/v1/orders/order-abc-123/cancel"
	if gotPath != wantPath {
		t.Fatalf("path: want %q, got %q", wantPath, gotPath)
	}
	if gotAcctHeader != "9" {
		t.Fatalf("X-Tossinvest-Account: want 9, got %q", gotAcctHeader)
	}
}

// TestCancelOrderRejectsSymbolMismatch guards issue #6: cancelling an order
// with another symbol asserted must refuse before the cancel POST goes out.
func TestCancelOrderRejectsSymbolMismatch(t *testing.T) {
	cancelCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/orders/order-abc-123":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"result":{"orderId":"order-abc-123","symbol":"TSLA","side":"BUY","orderType":"MARKET","status":"OPEN","quantity":"5"}}`))
		default:
			if strings.HasSuffix(r.URL.Path, "/cancel") {
				cancelCalls++
			}
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := New(
		Credentials{APIKey: "k", SecretKey: "s"},
		filepath.Join(t.TempDir(), "t.json"),
		WithBaseURL(srv.URL),
		WithHTTPClient(srv.Client()),
		WithAccountSeq(9),
	)

	if _, err := c.CancelOrder(context.Background(), orderintent.CancelIntent{OrderID: "order-abc-123", Symbol: "005930"}); err == nil {
		t.Fatal("expected symbol-mismatch refusal, got nil")
	}
	if cancelCalls != 0 {
		t.Fatalf("refused cancel must not POST, got %d calls", cancelCalls)
	}
	// A005930 vs 005930 is the same KR listing spelled differently: allowed.
	if !cancelSymbolsMatch("A005930", "005930") || !cancelSymbolsMatch("aapl", "AAPL") {
		t.Fatal("expected tolerant symbol match")
	}
	if cancelSymbolsMatch("TSLA", "005930") || cancelSymbolsMatch("", "005930") {
		t.Fatal("expected mismatch for different/empty symbols")
	}
}

// ---------------------------------------------------------------------------
// ModifyOrder integration test
// ---------------------------------------------------------------------------

// TestModifyOrder verifies POST body (inferred LIMIT orderType) and account header.
func TestModifyOrder(t *testing.T) {
	var gotBody string
	var gotPath string
	var gotAcctHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		default:
			if strings.HasSuffix(r.URL.Path, "/modify") {
				gotPath = r.URL.Path
				gotAcctHeader = r.Header.Get("X-Tossinvest-Account")
				var m map[string]any
				_ = json.NewDecoder(r.Body).Decode(&m)
				b, _ := json.Marshal(m)
				gotBody = string(b)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"result":{"orderId":"M1"}}`))
			} else {
				http.NotFound(w, r)
			}
		}
	}))
	defer srv.Close()

	c := New(
		Credentials{APIKey: "k", SecretKey: "s"},
		filepath.Join(t.TempDir(), "t.json"),
		WithBaseURL(srv.URL),
		WithHTTPClient(srv.Client()),
		WithAccountSeq(11),
	)

	newPrice := 71000.0
	intent := orderintent.AmendIntent{
		OrderID: "order-xyz-456",
		Price:   &newPrice,
	}
	res, err := c.ModifyOrder(context.Background(), intent)
	if err != nil {
		t.Fatal(err)
	}
	if res.Kind != "amend" {
		t.Fatalf("Kind: want amend, got %q", res.Kind)
	}
	if res.Status != "accepted" {
		t.Fatalf("Status: want accepted, got %q", res.Status)
	}
	// amend uses OriginalOrderID (the request target) and CurrentOrderID
	// (the new id issued by the server); the OrderID field is left empty.
	if res.OriginalOrderID != "order-xyz-456" {
		t.Fatalf("OriginalOrderID: want order-xyz-456, got %q", res.OriginalOrderID)
	}
	if res.CurrentOrderID != "M1" {
		t.Fatalf("CurrentOrderID: want M1, got %q", res.CurrentOrderID)
	}
	if res.OrderID != "" {
		t.Fatalf("OrderID: want empty, got %q", res.OrderID)
	}
	wantPath := "/api/v1/orders/order-xyz-456/modify"
	if gotPath != wantPath {
		t.Fatalf("path: want %q, got %q", wantPath, gotPath)
	}
	if gotAcctHeader != "11" {
		t.Fatalf("X-Tossinvest-Account: want 11, got %q", gotAcctHeader)
	}
	for _, want := range []string{`"orderType":"LIMIT"`, `"price":"71000"`} {
		if !strings.Contains(gotBody, want) {
			t.Fatalf("body missing %s; got %s", want, gotBody)
		}
	}
}

// TestModifyOrderRejectsMissingPrice guards issue #2: with no price the old
// code sent orderType MARKET, silently converting a limit order into a market
// order. The builder must refuse instead — no /modify call may go out.
func TestModifyOrderRejectsMissingPrice(t *testing.T) {
	modifyCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		default:
			if strings.HasSuffix(r.URL.Path, "/modify") {
				modifyCalls++
			}
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := New(
		Credentials{APIKey: "k", SecretKey: "s"},
		filepath.Join(t.TempDir(), "t.json"),
		WithBaseURL(srv.URL),
		WithHTTPClient(srv.Client()),
		WithAccountSeq(11),
	)

	newQty := 4.0
	intent := orderintent.AmendIntent{
		OrderID:  "order-mkt-789",
		Quantity: &newQty,
	}
	if _, err := c.ModifyOrder(context.Background(), intent); err == nil {
		t.Fatal("expected rejection for price-less amend, got nil")
	}
	if modifyCalls != 0 {
		t.Fatalf("rejected amend must not call /modify, got %d calls", modifyCalls)
	}

	if _, err := buildOrderModify(intent); err == nil {
		t.Fatal("expected buildOrderModify error for price-less intent, got nil")
	}
	newPrice := 71000.0
	limitIntent := orderintent.AmendIntent{OrderID: "order-mkt-789", Quantity: &newQty, Price: &newPrice}
	body, err := buildOrderModify(limitIntent)
	if err != nil {
		t.Fatalf("buildOrderModify: %v", err)
	}
	if body.OrderType != "LIMIT" || body.Price != "71000" || body.Quantity != "4" {
		t.Fatalf("unexpected body: %+v", body)
	}
}

// ---------------------------------------------------------------------------
// buildOrderCreate error cases
// ---------------------------------------------------------------------------

// TestBuildOrderCreateErrors verifies the builder rejects incoherent intents.
func TestBuildOrderCreateErrors(t *testing.T) {
	cases := []struct {
		name   string
		intent orderintent.PlaceIntent
	}{
		{
			name:   "fractional buy without amount",
			intent: orderintent.PlaceIntent{Symbol: "TSLA", Side: "buy", Fractional: true},
		},
		{
			name:   "fractional sell without quantity",
			intent: orderintent.PlaceIntent{Symbol: "TSLA", Side: "sell", Fractional: true},
		},
		{
			name:   "non-fractional without quantity",
			intent: orderintent.PlaceIntent{Symbol: "005930", Side: "buy", OrderType: "market"},
		},
		{
			name:   "limit without price",
			intent: orderintent.PlaceIntent{Symbol: "005930", Side: "buy", OrderType: "limit", Quantity: 10},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := buildOrderCreate(tc.intent); err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
		})
	}
}

// 검증만 통과하고 바디에 안 실리면 사용자가 지정한 조건 없이 주문이 나간다.
// MARKET 은 원래 timeInForce 를 아예 안 보냈으므로(omitempty) 특히 중요하다.
func TestBuildOrderCreateCarriesTimeInForce(t *testing.T) {
	cases := []struct {
		name   string
		intent orderintent.PlaceIntent
		want   string
	}{
		{"지정가 기본은 DAY 유지", orderintent.PlaceIntent{
			Symbol: "AAPL", Market: "us", Side: "buy", OrderType: "limit", Quantity: 1, Price: 10,
			CurrencyMode: "USD",
		}, "DAY"},
		{"CLS 가 실린다", orderintent.PlaceIntent{
			Symbol: "AAPL", Market: "us", Side: "buy", OrderType: "limit", Quantity: 1, Price: 10,
			TimeInForce: "CLS", CurrencyMode: "USD",
		}, "CLS"},
		{"시장가에도 OPG 가 실린다", orderintent.PlaceIntent{
			Symbol: "005930", Market: "kr", Side: "buy", OrderType: "market", Quantity: 1,
			TimeInForce: "OPG",
		}, "OPG"},
		{"시장가 기본은 종전대로 비운다", orderintent.PlaceIntent{
			Symbol: "005930", Market: "kr", Side: "buy", OrderType: "market", Quantity: 1,
		}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := buildOrderCreate(c.intent)
			if err != nil {
				t.Fatalf("buildOrderCreate: %v", err)
			}
			v0, ok := got.(orderCreateV0)
			if !ok {
				t.Fatalf("expected quantity-based variant, got %T", got)
			}
			if v0.TimeInForce != c.want {
				t.Errorf("TimeInForce = %q, want %q", v0.TimeInForce, c.want)
			}
		})
	}
}

// TestBuildOrderCreateRejectsNonUSDCurrency guards issue #1: the official API
// prices US fractional-buy amounts and US limit prices in USD. A KRW value
// sent here would execute ~1400x off intent, so the builder must refuse it —
// the hybrid broker routes these to WTS, and official-only callers (ops/MCP)
// must fail closed instead of mis-sending.
func TestBuildOrderCreateRejectsNonUSDCurrency(t *testing.T) {
	cases := []struct {
		name   string
		intent orderintent.PlaceIntent
	}{
		{
			name: "US fractional buy in KRW",
			intent: orderintent.PlaceIntent{Symbol: "TSLA", Market: "us", Side: "buy",
				OrderType: "market", Fractional: true, Amount: 10000, CurrencyMode: "KRW"},
		},
		{
			name: "US limit in KRW",
			intent: orderintent.PlaceIntent{Symbol: "AAPL", Market: "us", Side: "buy",
				OrderType: "limit", Quantity: 1, Price: 250000, CurrencyMode: "KRW"},
		},
		{
			name: "US fractional buy without currency",
			intent: orderintent.PlaceIntent{Symbol: "TSLA", Market: "us", Side: "buy",
				OrderType: "market", Fractional: true, Amount: 100},
		},
		{
			name: "US limit without currency",
			intent: orderintent.PlaceIntent{Symbol: "AAPL", Market: "us", Side: "buy",
				OrderType: "limit", Quantity: 1, Price: 150},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := buildOrderCreate(tc.intent); err == nil {
				t.Fatal("expected currency guard error, got nil")
			}
		})
	}

	allowed := []struct {
		name   string
		intent orderintent.PlaceIntent
	}{
		{
			name: "US fractional buy in USD",
			intent: orderintent.PlaceIntent{Symbol: "TSLA", Market: "us", Side: "buy",
				OrderType: "market", Fractional: true, Amount: 100, CurrencyMode: "USD"},
		},
		{
			name: "US limit in USD",
			intent: orderintent.PlaceIntent{Symbol: "AAPL", Market: "us", Side: "buy",
				OrderType: "limit", Quantity: 1, Price: 150.25, CurrencyMode: "USD"},
		},
		{
			name: "US market carries no currency value",
			intent: orderintent.PlaceIntent{Symbol: "AAPL", Market: "us", Side: "buy",
				OrderType: "market", Quantity: 1, CurrencyMode: "KRW"},
		},
		{
			name: "US fractional sell is share quantity",
			intent: orderintent.PlaceIntent{Symbol: "TSLA", Market: "us", Side: "sell",
				OrderType: "market", Fractional: true, Quantity: 0.5, CurrencyMode: "KRW"},
		},
		{
			name: "KR limit stays KRW",
			intent: orderintent.PlaceIntent{Symbol: "005930", Market: "kr", Side: "buy",
				OrderType: "limit", Quantity: 10, Price: 70000, CurrencyMode: "KRW"},
		},
	}
	for _, tc := range allowed {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := buildOrderCreate(tc.intent); err != nil {
				t.Fatalf("buildOrderCreate: %v", err)
			}
		})
	}
}
