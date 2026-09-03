package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestGetQuoteFromFixtures(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fixturePath := fixturePathForRequest(t, r.URL.Path)
		http.ServeFile(w, r, fixturePath)
	}))
	defer server.Close()

	client := New(Config{
		HTTPClient:  server.Client(),
		InfoBaseURL: server.URL,
	})

	quote, err := client.GetQuote(context.Background(), "005930")
	if err != nil {
		t.Fatalf("GetQuote returned error: %v", err)
	}

	if quote.ProductCode != "A005930" {
		t.Fatalf("unexpected product code: %s", quote.ProductCode)
	}

	if quote.Symbol != "005930" {
		t.Fatalf("unexpected symbol: %s", quote.Symbol)
	}

	if quote.Name != "삼성전자" {
		t.Fatalf("unexpected name: %s", quote.Name)
	}

	if quote.Last != 193900 {
		t.Fatalf("unexpected last price: %v", quote.Last)
	}

	if quote.ReferencePrice != 187900 {
		t.Fatalf("unexpected reference price: %v", quote.ReferencePrice)
	}

	if quote.Volume != 27306483 {
		t.Fatalf("unexpected volume: %v", quote.Volume)
	}

	// v3 details enrichment
	if quote.High52w != 200000 || quote.Low52w != 56800 {
		t.Fatalf("unexpected 52w high/low: %v / %v", quote.High52w, quote.Low52w)
	}
	if quote.MarketCap != 1150000000000000 {
		t.Fatalf("unexpected market cap: %v", quote.MarketCap)
	}
	if quote.TradingStrength != 102.34 {
		t.Fatalf("unexpected trading strength: %v", quote.TradingStrength)
	}
}

func TestGetQuoteResolvesUSSymbolViaSearch(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v2/search/stocks":
			_, _ = w.Write([]byte(`{"result":{"stocks":[{"stockCode":"US20220809012","stockName":"TSLL","matchType":"EXACT"}]}}`))
		case r.URL.Path == "/api/v2/stock-infos/US20220809012":
			_, _ = w.Write([]byte(`{"result":{"symbol":"TSLL","name":"TSLL","currency":"USD","status":"N","market":{"code":"NSQ","displayName":"NASDAQ"}}}`))
		case r.URL.Path == "/api/v1/product/stock-prices":
			_, _ = w.Write([]byte(`{"result":[{"productCode":"US20220809012","currency":"USD","base":14.38,"close":15.36,"volume":13409779}]}`))
		case r.URL.Path == "/api/v3/stock-prices/details":
			_, _ = w.Write([]byte(`{"result":[{"code":"US20220809012","open":14.5,"high":15.6,"low":14.2,"close":15.36,"high52w":20.1,"low52w":8.3,"marketCap":0,"value":0,"tradingStrength":98.7}]}`))
		case r.URL.Path == "/api/v1/stock-detail/ui/US20220809012/common":
			_, _ = w.Write([]byte(`{"result":{"badges":[],"notices":[]}}`))
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := New(Config{
		HTTPClient:  server.Client(),
		InfoBaseURL: server.URL,
	})

	quote, err := client.GetQuote(context.Background(), "TSLL")
	if err != nil {
		t.Fatalf("GetQuote returned error: %v", err)
	}
	if quote.ProductCode != "US20220809012" {
		t.Fatalf("unexpected product code: %s", quote.ProductCode)
	}
	if quote.Symbol != "TSLL" {
		t.Fatalf("unexpected symbol: %s", quote.Symbol)
	}
	if quote.Last != 15.36 {
		t.Fatalf("unexpected last price: %v", quote.Last)
	}
}

func fixturePathForRequest(t *testing.T, path string) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve test path")
	}

	root := filepath.Join(filepath.Dir(filename), "..", "..", "fixtures", "responses", "public")

	switch path {
	case "/api/v2/stock-infos/A005930":
		return mustPublicFixturePath(t, filepath.Join(root, "stock-info.json"))
	case "/api/v1/stock-detail/ui/A005930/common":
		return mustPublicFixturePath(t, filepath.Join(root, "stock-detail-common.json"))
	case "/api/v1/product/stock-prices":
		return mustPublicFixturePath(t, filepath.Join(root, "stock-price.json"))
	case "/api/v3/stock-prices/details":
		return mustPublicFixturePath(t, filepath.Join(root, "stock-price-details.json"))
	default:
		t.Fatalf("unexpected request path: %s", path)
		return ""
	}
}

func mustPublicFixturePath(t *testing.T, path string) string {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("fixture missing: %s: %v", path, err)
	}
	return path
}

// 옵션 계약 guid 는 product code 인데도 shape 검사에 걸려 종목 검색으로 빠졌다.
// 검색에는 계약 항목이 없으니 quote options 로 얻은 guid 를 조회하면 전부 실패한다.
func TestLooksLikeProductCodeAcceptsOptionGuid(t *testing.T) {
	if !looksLikeProductCode("OPT_AAPL260805C00230000_20260722") {
		t.Error("option contract guid rejected as a product code")
	}
	// 기존 형태는 그대로 통과해야 한다.
	for _, ok := range []string{"A005930", "US19801212001", "US.AAPL"} {
		if !looksLikeProductCode(ok) {
			t.Errorf("%s rejected", ok)
		}
	}
	// 티커는 여전히 검색을 거쳐야 한다.
	for _, no := range []string{"AAPL", "삼성전자", "OPT"} {
		if looksLikeProductCode(no) {
			t.Errorf("%s accepted as a product code", no)
		}
	}
}
