package official

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

// TestAdaptHoldingsUnit verifies the pure adapter for Holdings.
func TestAdaptHoldingsUnit(t *testing.T) {
	items := []apiHoldingsItem{
		{
			Symbol:               "005930",
			Name:                 "삼성전자",
			MarketCountry:        "KR",
			Currency:             "KRW",
			Quantity:             "100",
			LastPrice:            "72000",
			AveragePurchasePrice: "65000",
			MarketValue:          struct {
				Amount          string `json:"amount"`
				AmountAfterCost string `json:"amountAfterCost"`
				PurchaseAmount  string `json:"purchaseAmount"`
			}{Amount: "7200000", AmountAfterCost: "7050000", PurchaseAmount: "6500000"},
			ProfitLoss: struct {
				Amount          string `json:"amount"`
				AmountAfterCost string `json:"amountAfterCost"`
				Rate            string `json:"rate"`
				RateAfterCost   string `json:"rateAfterCost"`
			}{Amount: "700000", AmountAfterCost: "550000", Rate: "0.1077", RateAfterCost: "0.0846"},
			DailyProfitLoss: struct {
				Amount string `json:"amount"`
				Rate   string `json:"rate"`
			}{Amount: "100000", Rate: "0.0141"},
		},
	}
	got := adaptHoldings(items)
	if len(got) != 1 {
		t.Fatalf("want 1, got %d", len(got))
	}
	p := got[0]
	if p.Symbol != "005930" {
		t.Fatalf("Symbol: want 005930, got %q", p.Symbol)
	}
	if p.Name != "삼성전자" {
		t.Fatalf("Name: want 삼성전자, got %q", p.Name)
	}
	if p.MarketType != "KR" {
		t.Fatalf("MarketType: want KR, got %q", p.MarketType)
	}
	if p.Quantity != 100 {
		t.Fatalf("Quantity: want 100, got %v", p.Quantity)
	}
	if p.AveragePrice != 65000 {
		t.Fatalf("AveragePrice: want 65000, got %v", p.AveragePrice)
	}
	if p.CurrentPrice != 72000 {
		t.Fatalf("CurrentPrice: want 72000, got %v", p.CurrentPrice)
	}
	if p.MarketValue != 7200000 {
		t.Fatalf("MarketValue: want 7200000, got %v", p.MarketValue)
	}
	if p.UnrealizedPnL != 700000 {
		t.Fatalf("UnrealizedPnL: want 700000, got %v", p.UnrealizedPnL)
	}
	if p.ProfitRate != 0.1077 {
		t.Fatalf("ProfitRate: want 0.1077, got %v", p.ProfitRate)
	}
	if p.DailyProfitLoss != 100000 {
		t.Fatalf("DailyProfitLoss: want 100000, got %v", p.DailyProfitLoss)
	}
	if p.DailyProfitRate != 0.0141 {
		t.Fatalf("DailyProfitRate: want 0.0141, got %v", p.DailyProfitRate)
	}
	// Fields not in official holdings response
	if p.ProductCode != "" {
		t.Fatalf("ProductCode: expected empty, got %q", p.ProductCode)
	}
	if p.MarketCode != "" {
		t.Fatalf("MarketCode: expected empty, got %q", p.MarketCode)
	}
}

// TestAdaptHoldingsEmpty verifies empty slice handling.
func TestAdaptHoldingsEmpty(t *testing.T) {
	got := adaptHoldings(nil)
	if got == nil {
		t.Fatal("expected non-nil empty slice")
	}
	if len(got) != 0 {
		t.Fatalf("expected 0, got %d", len(got))
	}
}

// TestHoldingsIntegration tests Holdings() against an httptest server.
func TestHoldingsIntegration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/holdings":
			if sym := r.URL.Query().Get("symbol"); sym != "005930" {
				t.Errorf("symbol: want 005930, got %q", sym)
			}
			_, _ = w.Write([]byte(`{"result":{"items":[{"symbol":"005930","name":"삼성전자","marketCountry":"KR","currency":"KRW","quantity":"100","lastPrice":"72000","averagePurchasePrice":"65000","marketValue":{"amount":"7200000","amountAfterCost":"7050000","purchaseAmount":"6500000"},"profitLoss":{"amount":"700000","amountAfterCost":"550000","rate":"0.1077","rateAfterCost":"0.0846"},"dailyProfitLoss":{"amount":"100000","rate":"0.0141"},"cost":{"commission":"14400","tax":"135600"}}],"totalPurchaseAmount":{"krw":"6500000","usd":null},"marketValue":{"amount":{"krw":"7200000","usd":null},"amountAfterCost":{"krw":"7050000","usd":null}},"profitLoss":{"amount":{"krw":"700000","usd":null},"amountAfterCost":{"krw":"550000","usd":null},"rate":"0.1077","rateAfterCost":"0.0846"},"dailyProfitLoss":{"amount":{"krw":"100000","usd":null},"rate":"0.0141"}}}`))
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

	got, err := c.Holdings(context.Background(), "005930")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 position, got %d", len(got))
	}
	if got[0].Symbol != "005930" {
		t.Fatalf("Symbol: want 005930, got %q", got[0].Symbol)
	}
	if got[0].Quantity != 100 {
		t.Fatalf("Quantity: want 100, got %v", got[0].Quantity)
	}
}

// TestHoldingsIntegrationAllSymbols tests Holdings() with no symbol filter.
func TestHoldingsIntegrationAllSymbols(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/holdings":
			if sym := r.URL.Query().Get("symbol"); sym != "" {
				t.Errorf("symbol: expected empty, got %q", sym)
			}
			_, _ = w.Write([]byte(`{"result":{"items":[],"totalPurchaseAmount":{"krw":"0","usd":null},"marketValue":{"amount":{"krw":"0","usd":null},"amountAfterCost":{"krw":"0","usd":null}},"profitLoss":{"amount":{"krw":"0","usd":null},"amountAfterCost":{"krw":"0","usd":null},"rate":"0","rateAfterCost":"0"},"dailyProfitLoss":{"amount":{"krw":"0","usd":null},"rate":"0"}}}`))
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

	got, err := c.Holdings(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("want 0 positions, got %d", len(got))
	}
}
