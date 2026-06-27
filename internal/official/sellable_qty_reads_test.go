package official

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

// TestAdaptSellableQuantityUnit verifies the pure adapter for SellableQuantity.
func TestAdaptSellableQuantityUnit(t *testing.T) {
	raw := apiSellableQuantity{SellableQuantity: "100"}
	got := adaptSellableQuantity("005930", raw)
	if got.Symbol != "005930" {
		t.Fatalf("Symbol: want 005930, got %q", got.Symbol)
	}
	if got.Quantity != 100 {
		t.Fatalf("Quantity: want 100, got %v", got.Quantity)
	}
	if got.ProductCode != "" {
		t.Fatalf("ProductCode: expected empty, got %q", got.ProductCode)
	}
}

// TestAdaptSellableQuantityDecimal verifies fractional share handling.
func TestAdaptSellableQuantityDecimal(t *testing.T) {
	raw := apiSellableQuantity{SellableQuantity: "0.5"}
	got := adaptSellableQuantity("AAPL", raw)
	if got.Quantity != 0.5 {
		t.Fatalf("Quantity: want 0.5, got %v", got.Quantity)
	}
}

// TestSellableQuantityIntegration tests SellableQuantity() against an httptest server.
func TestSellableQuantityIntegration(t *testing.T) {
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/sellable-quantity":
			gotHeader = r.Header.Get("X-Tossinvest-Account")
			if r.URL.Query().Get("symbol") != "005930" {
				t.Errorf("symbol: want 005930, got %q", r.URL.Query().Get("symbol"))
			}
			_, _ = w.Write([]byte(`{"result":{"sellableQuantity":"100"}}`))
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

	got, err := c.SellableQuantity(context.Background(), "005930")
	if err != nil {
		t.Fatal(err)
	}
	if got.Quantity != 100 {
		t.Fatalf("Quantity: want 100, got %v", got.Quantity)
	}
	if gotHeader != "7" {
		t.Fatalf("X-Tossinvest-Account: want 7, got %q", gotHeader)
	}
}
