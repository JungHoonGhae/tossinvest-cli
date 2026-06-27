package official

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

// TestAdaptStocksUnit verifies the pure adapter for Stocks.
func TestAdaptStocksUnit(t *testing.T) {
	raw := []apiStockInfo{
		{
			Symbol:      "005930",
			Name:        "삼성전자",
			EnglishName: "SamsungElec",
			Market:      "KOSPI",
			Currency:    "KRW",
			Status:      "ACTIVE",
		},
		{
			Symbol:      "AAPL",
			Name:        "애플",
			EnglishName: "APPLE INC",
			Market:      "NASDAQ",
			Currency:    "USD",
			Status:      "ACTIVE",
		},
	}
	got := adaptStocks(raw)
	if len(got) != 2 {
		t.Fatalf("want 2, got %d", len(got))
	}

	kr := got[0]
	if kr.Symbol != "005930" {
		t.Fatalf("Symbol: want 005930, got %q", kr.Symbol)
	}
	if kr.Name != "삼성전자" {
		t.Fatalf("Name: want 삼성전자, got %q", kr.Name)
	}
	if kr.MarketCode != "KOSPI" {
		t.Fatalf("MarketCode: want KOSPI, got %q", kr.MarketCode)
	}
	if kr.Currency != "KRW" {
		t.Fatalf("Currency: want KRW, got %q", kr.Currency)
	}
	if kr.Status != "ACTIVE" {
		t.Fatalf("Status: want ACTIVE, got %q", kr.Status)
	}
	// Price fields not in /stocks response
	if kr.Last != 0 {
		t.Fatalf("Last: expected 0, got %v", kr.Last)
	}
	if kr.Volume != 0 {
		t.Fatalf("Volume: expected 0, got %v", kr.Volume)
	}

	us := got[1]
	if us.Symbol != "AAPL" {
		t.Fatalf("Symbol: want AAPL, got %q", us.Symbol)
	}
	// market maps to MarketCode; the display-name Market field stays empty (/stocks
	// does not provide it).
	if us.Market != "" {
		t.Fatalf("Market: want empty, got %q", us.Market)
	}
	if us.MarketCode != "NASDAQ" {
		t.Fatalf("MarketCode: want NASDAQ, got %q", us.MarketCode)
	}
}

// TestAdaptStocksEmpty verifies empty slice handling.
func TestAdaptStocksEmpty(t *testing.T) {
	got := adaptStocks(nil)
	if got == nil {
		t.Fatal("expected non-nil empty slice")
	}
	if len(got) != 0 {
		t.Fatalf("expected 0, got %d", len(got))
	}
}

// TestStocksIntegration tests Stocks() against an httptest server.
func TestStocksIntegration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/stocks":
			if r.URL.Query().Get("symbols") != "005930,AAPL" {
				t.Errorf("symbols: want 005930,AAPL, got %q", r.URL.Query().Get("symbols"))
			}
			_, _ = w.Write([]byte(`{"result":[{"symbol":"005930","name":"삼성전자","englishName":"SamsungElec","isinCode":"KR7005930003","market":"KOSPI","securityType":"STOCK","isCommonShare":true,"status":"ACTIVE","currency":"KRW","sharesOutstanding":"5919637922"},{"symbol":"AAPL","name":"애플","englishName":"APPLE INC","isinCode":"US0378331005","market":"NASDAQ","securityType":"STOCK","isCommonShare":true,"status":"ACTIVE","currency":"USD","sharesOutstanding":"14702703000"}]}`))
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

	got, err := c.Stocks(context.Background(), []string{"005930", "AAPL"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2, got %d", len(got))
	}
	if got[0].Symbol != "005930" || got[0].MarketCode != "KOSPI" {
		t.Fatalf("first: %+v", got[0])
	}
	if got[1].Symbol != "AAPL" || got[1].MarketCode != "NASDAQ" {
		t.Fatalf("second: %+v", got[1])
	}
}
