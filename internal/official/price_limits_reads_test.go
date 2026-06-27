package official

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

// TestAdaptPriceLimitsUnit verifies the pure adapter for PriceLimits.
func TestAdaptPriceLimitsUnit(t *testing.T) {
	raw := apiPriceLimits{
		UpperLimitPrice: "93000",
		LowerLimitPrice: "50400",
		Currency:        "KRW",
		Timestamp:       "2026-03-25T09:30:00.123+09:00",
	}
	got := adaptPriceLimits("005930", raw)
	if got.Symbol != "005930" {
		t.Fatalf("Symbol: want 005930, got %q", got.Symbol)
	}
	if got.UpperLimit != 93000 {
		t.Fatalf("UpperLimit: want 93000, got %v", got.UpperLimit)
	}
	if got.LowerLimit != 50400 {
		t.Fatalf("LowerLimit: want 50400, got %v", got.LowerLimit)
	}
	// Fields not available from /price-limits endpoint.
	if got.ProductCode != "" {
		t.Fatalf("ProductCode: expected empty, got %q", got.ProductCode)
	}
	if got.Date != "" {
		t.Fatalf("Date: expected empty, got %q", got.Date)
	}
}

// TestAdaptPriceLimitsNullable verifies that null (empty string) limits map to 0.
func TestAdaptPriceLimitsNullable(t *testing.T) {
	raw := apiPriceLimits{
		UpperLimitPrice: "",
		LowerLimitPrice: "",
		Currency:        "USD",
	}
	got := adaptPriceLimits("AAPL", raw)
	if got.UpperLimit != 0 {
		t.Fatalf("UpperLimit: want 0, got %v", got.UpperLimit)
	}
	if got.LowerLimit != 0 {
		t.Fatalf("LowerLimit: want 0, got %v", got.LowerLimit)
	}
}

// TestPriceLimitsIntegration tests PriceLimits() against an httptest server.
func TestPriceLimitsIntegration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/price-limits":
			if r.URL.Query().Get("symbol") != "005930" {
				t.Errorf("symbol: want 005930, got %q", r.URL.Query().Get("symbol"))
			}
			_, _ = w.Write([]byte(`{"result":{"upperLimitPrice":"93000","lowerLimitPrice":"50400","currency":"KRW","timestamp":"2026-03-25T09:30:00.123+09:00"}}`))
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
	)

	got, err := c.PriceLimits(context.Background(), "005930")
	if err != nil {
		t.Fatal(err)
	}
	if got.Symbol != "005930" {
		t.Fatalf("Symbol: want 005930, got %q", got.Symbol)
	}
	if got.UpperLimit != 93000 {
		t.Fatalf("UpperLimit: want 93000, got %v", got.UpperLimit)
	}
	if got.LowerLimit != 50400 {
		t.Fatalf("LowerLimit: want 50400, got %v", got.LowerLimit)
	}
}
