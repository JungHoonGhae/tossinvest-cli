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
		Symbol:     "TSLA",
		Side:       "buy",
		Fractional: true,
		Amount:     100.5,
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
		Symbol:    "TSLA",
		Side:      "buy",
		OrderType: "limit",
		Quantity:  3,
		Price:     150.25,
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
func TestCancelOrder(t *testing.T) {
	var gotPath string
	var gotAcctHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
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

	res, err := c.CancelOrder(context.Background(), "order-abc-123")
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
	if res.OrderID != "M1" {
		t.Fatalf("OrderID: want M1, got %q", res.OrderID)
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
