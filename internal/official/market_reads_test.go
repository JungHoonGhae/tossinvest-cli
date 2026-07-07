package official

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// ExchangeRate
// ---------------------------------------------------------------------------

// TestAdaptExchangeRateUnit verifies pure adapter for ExchangeRate.
func TestAdaptExchangeRateUnit(t *testing.T) {
	raw := apiExchangeRate{
		BaseCurrency:  "USD",
		QuoteCurrency: "KRW",
		Rate:          "1380.5",
		MidRate:       "1375",
	}
	got := adaptExchangeRate(raw)
	if got.Code != "USD/KRW" {
		t.Fatalf("Code: want USD/KRW, got %q", got.Code)
	}
	if got.Close != 1380.5 {
		t.Fatalf("Close: want 1380.5, got %v", got.Close)
	}
	if got.Base != 1375 {
		t.Fatalf("Base: want 1375, got %v", got.Base)
	}
	if got.Name != "" {
		t.Fatalf("Name: expected empty (not in official endpoint), got %q", got.Name)
	}
}

// TestExchangeRateIntegration tests ExchangeRate() against an httptest server.
func TestExchangeRateIntegration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/exchange-rate":
			if r.URL.Query().Get("baseCurrency") != "USD" {
				t.Errorf("baseCurrency: want USD, got %q", r.URL.Query().Get("baseCurrency"))
			}
			if r.URL.Query().Get("quoteCurrency") != "KRW" {
				t.Errorf("quoteCurrency: want KRW, got %q", r.URL.Query().Get("quoteCurrency"))
			}
			_, _ = w.Write([]byte(`{"result":{"baseCurrency":"USD","quoteCurrency":"KRW","rate":"1380.5","midRate":"1375","basisPoint":"40","rateChangeType":"UP","validFrom":"2026-03-25T09:30:00+09:00","validUntil":"2026-03-25T09:31:00+09:00"}}`))
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

	got, err := c.ExchangeRate(context.Background(), "USD", "KRW")
	if err != nil {
		t.Fatal(err)
	}
	if got.Code != "USD/KRW" {
		t.Fatalf("Code: want USD/KRW, got %q", got.Code)
	}
	if got.Close != 1380.5 {
		t.Fatalf("Close: want 1380.5, got %v", got.Close)
	}
	if got.Base != 1375 {
		t.Fatalf("Base: want 1375, got %v", got.Base)
	}
}

// ---------------------------------------------------------------------------
// Prices
// ---------------------------------------------------------------------------

// TestAdaptPricesUnit verifies the pure adapter for Prices.
func TestAdaptPricesUnit(t *testing.T) {
	raw := []apiPrice{
		{Symbol: "005930", LastPrice: "72000", Currency: "KRW"},
		{Symbol: "AAPL", LastPrice: "185.70", Currency: "USD"},
	}
	got := adaptPrices(raw)
	if len(got) != 2 {
		t.Fatalf("want 2, got %d", len(got))
	}
	if got[0].Symbol != "005930" || got[0].Last != 72000 || got[0].Currency != "KRW" {
		t.Fatalf("first: %+v", got[0])
	}
	if got[1].Symbol != "AAPL" || got[1].Last != 185.70 || got[1].Currency != "USD" {
		t.Fatalf("second: %+v", got[1])
	}
	// Non-price fields not available from /prices endpoint
	if got[0].Name != "" {
		t.Fatalf("Name: expected empty, got %q", got[0].Name)
	}
}

// TestAdaptPricesEmpty verifies empty slice handling.
func TestAdaptPricesEmpty(t *testing.T) {
	got := adaptPrices(nil)
	if got == nil {
		t.Fatal("expected non-nil empty slice")
	}
	if len(got) != 0 {
		t.Fatalf("expected 0, got %d", len(got))
	}
}

// TestPricesIntegration tests Prices() against an httptest server.
func TestPricesIntegration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/prices":
			if r.URL.Query().Get("symbols") != "005930,AAPL" {
				t.Errorf("symbols: want 005930,AAPL, got %q", r.URL.Query().Get("symbols"))
			}
			_, _ = w.Write([]byte(`{"result":[{"symbol":"005930","lastPrice":"72000","currency":"KRW"},{"symbol":"AAPL","lastPrice":"185.70","currency":"USD"}]}`))
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

	got, err := c.Prices(context.Background(), []string{"005930", "AAPL"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2, got %d", len(got))
	}
	if got[0].Symbol != "005930" || got[0].Last != 72000 {
		t.Fatalf("first: %+v", got[0])
	}
}

// ---------------------------------------------------------------------------
// Orderbook
// ---------------------------------------------------------------------------

// TestAdaptOrderbookUnit verifies the pure adapter for Orderbook.
func TestAdaptOrderbookUnit(t *testing.T) {
	raw := apiOrderbook{
		Asks:     []apiOrderbookEntry{{Price: "72100", Volume: "8500"}, {Price: "72200", Volume: "3400"}},
		Bids:     []apiOrderbookEntry{{Price: "72000", Volume: "5200"}, {Price: "71900", Volume: "4100"}},
		Currency: "KRW",
	}
	got := adaptOrderbook("005930", raw)
	if got.Symbol != "005930" {
		t.Fatalf("Symbol: want 005930, got %q", got.Symbol)
	}
	if len(got.Offers) != 2 {
		t.Fatalf("Offers: want 2, got %d", len(got.Offers))
	}
	if got.Offers[0].Price != 72100 || got.Offers[0].Volume != 8500 {
		t.Fatalf("Offers[0]: %+v", got.Offers[0])
	}
	if len(got.Bids) != 2 {
		t.Fatalf("Bids: want 2, got %d", len(got.Bids))
	}
	if got.Bids[0].Price != 72000 || got.Bids[0].Volume != 5200 {
		t.Fatalf("Bids[0]: %+v", got.Bids[0])
	}
	// TotalOffer = 8500+3400, TotalBid = 5200+4100
	if got.TotalOffer != 11900 {
		t.Fatalf("TotalOffer: want 11900, got %v", got.TotalOffer)
	}
	if got.TotalBid != 9300 {
		t.Fatalf("TotalBid: want 9300, got %v", got.TotalBid)
	}
	// Fields not in official orderbook response
	if got.ProductCode != "" {
		t.Fatalf("ProductCode: expected empty, got %q", got.ProductCode)
	}
	if got.Name != "" {
		t.Fatalf("Name: expected empty, got %q", got.Name)
	}
}

// TestOrderbookIntegration tests Orderbook() against an httptest server.
func TestOrderbookIntegration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/orderbook":
			if r.URL.Query().Get("symbol") != "005930" {
				t.Errorf("symbol: want 005930, got %q", r.URL.Query().Get("symbol"))
			}
			_, _ = w.Write([]byte(`{"result":{"asks":[{"price":"72100","volume":"8500"}],"bids":[{"price":"72000","volume":"5200"}],"currency":"KRW","timestamp":"2026-03-25T09:30:00.123+09:00"}}`))
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

	got, err := c.Orderbook(context.Background(), "005930")
	if err != nil {
		t.Fatal(err)
	}
	if got.Symbol != "005930" {
		t.Fatalf("Symbol: want 005930, got %q", got.Symbol)
	}
	if len(got.Offers) != 1 || got.Offers[0].Price != 72100 {
		t.Fatalf("Offers: %+v", got.Offers)
	}
	if len(got.Bids) != 1 || got.Bids[0].Volume != 5200 {
		t.Fatalf("Bids: %+v", got.Bids)
	}
}

// ---------------------------------------------------------------------------
// Trades
// ---------------------------------------------------------------------------

// TestAdaptTradesUnit verifies the pure adapter for Trades.
func TestAdaptTradesUnit(t *testing.T) {
	raw := []apiTrade{
		{Price: "72000", Volume: "120", Timestamp: "2026-03-25T09:30:42.000+09:00", Currency: "KRW"},
		{Price: "71900", Volume: "50", Timestamp: "2026-03-25T09:30:41.500+09:00", Currency: "KRW"},
	}
	got := adaptTrades("005930", raw)
	if got.Symbol != "005930" {
		t.Fatalf("Symbol: want 005930, got %q", got.Symbol)
	}
	if len(got.Trades) != 2 {
		t.Fatalf("Trades: want 2, got %d", len(got.Trades))
	}
	if got.Trades[0].Price != 72000 || got.Trades[0].Volume != 120 {
		t.Fatalf("Trades[0]: %+v", got.Trades[0])
	}
	if got.Trades[0].Time != "2026-03-25T09:30:42.000+09:00" {
		t.Fatalf("Trades[0].Time: got %q", got.Trades[0].Time)
	}
	// Fields not in official Trade schema
	if got.Trades[0].TradeType != "" {
		t.Fatalf("TradeType: expected empty, got %q", got.Trades[0].TradeType)
	}
	if got.ProductCode != "" {
		t.Fatalf("ProductCode: expected empty, got %q", got.ProductCode)
	}
}

// TestAdaptTradesEmpty verifies empty slice handling.
func TestAdaptTradesEmpty(t *testing.T) {
	got := adaptTrades("TSLA", nil)
	if got.Symbol != "TSLA" {
		t.Fatalf("Symbol: want TSLA, got %q", got.Symbol)
	}
	if len(got.Trades) != 0 {
		t.Fatalf("Trades: expected 0, got %d", len(got.Trades))
	}
}

// TestTradesIntegration tests Trades() against an httptest server.
func TestTradesIntegration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/trades":
			if r.URL.Query().Get("symbol") != "005930" {
				t.Errorf("symbol: want 005930, got %q", r.URL.Query().Get("symbol"))
			}
			if r.URL.Query().Get("count") != "3" {
				t.Errorf("count: want 3, got %q", r.URL.Query().Get("count"))
			}
			_, _ = w.Write([]byte(`{"result":[{"price":"72000","volume":"120","timestamp":"2026-03-25T09:30:42.000+09:00","currency":"KRW"}]}`))
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

	got, err := c.Trades(context.Background(), "005930", 3)
	if err != nil {
		t.Fatal(err)
	}
	if got.Symbol != "005930" {
		t.Fatalf("Symbol: want 005930, got %q", got.Symbol)
	}
	if len(got.Trades) != 1 || got.Trades[0].Price != 72000 {
		t.Fatalf("Trades: %+v", got.Trades)
	}
}

// ---------------------------------------------------------------------------
// Rankings
// ---------------------------------------------------------------------------

func TestAdaptRankingUnit(t *testing.T) {
	raw := apiRankingResult{
		RankedAt: "2026-06-10T14:30:00+09:00",
		Rankings: []apiRankingItem{
			{
				Rank: 1, Symbol: "005930", Currency: "KRW",
				Price:         apiRankingPrice{LastPrice: "71900", BasePrice: "71000", ChangeRate: "0.0127"},
				TradingVolume: "12345678", TradingAmount: "888000000000",
			},
			{
				Rank: 2, Symbol: "000660", Currency: "KRW",
				Price:         apiRankingPrice{LastPrice: "175000", BasePrice: "176000", ChangeRate: ""},
				TradingVolume: "2222", TradingAmount: "3333",
			},
		},
	}
	got := adaptRanking("MARKET_TRADING_AMOUNT", "KR", "1d", raw)
	if got.Type != "MARKET_TRADING_AMOUNT" || got.MarketCountry != "KR" || got.Duration != "1d" {
		t.Fatalf("meta not carried: %+v", got)
	}
	if got.RankedAt != "2026-06-10T14:30:00+09:00" {
		t.Fatalf("RankedAt: got %q", got.RankedAt)
	}
	if len(got.Items) != 2 {
		t.Fatalf("Items len: want 2, got %d", len(got.Items))
	}
	if got.Items[0].LastPrice != 71900 || got.Items[0].ChangeRate != 0.0127 || got.Items[0].TradingAmount != 888000000000 {
		t.Fatalf("item0 decimals: %+v", got.Items[0])
	}
	if got.Items[1].ChangeRate != 0 { // "" → 0 (nullable)
		t.Fatalf("item1 ChangeRate: want 0 for empty, got %v", got.Items[1].ChangeRate)
	}
}

func TestRankingsIntegration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/rankings":
			if r.URL.Query().Get("type") != "MARKET_TRADING_AMOUNT" {
				t.Errorf("type: got %q", r.URL.Query().Get("type"))
			}
			if r.URL.Query().Get("marketCountry") != "KR" {
				t.Errorf("marketCountry: got %q", r.URL.Query().Get("marketCountry"))
			}
			if r.URL.Query().Get("duration") != "1d" {
				t.Errorf("duration: got %q", r.URL.Query().Get("duration"))
			}
			_, _ = w.Write([]byte(`{"result":{"rankedAt":"2026-06-10T14:30:00+09:00","rankings":[{"rank":1,"symbol":"005930","currency":"KRW","price":{"lastPrice":"71900","basePrice":"71000","changeRate":"0.0127"},"tradingVolume":"12345678","tradingAmount":"888000000000"}]}}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := New(
		Credentials{APIKey: "k", SecretKey: "s"},
		filepath.Join(t.TempDir(), "t.json"),
		WithBaseURL(srv.URL),
		WithHTTPClient(srv.Client()),
	)
	got, err := c.Rankings(context.Background(), "MARKET_TRADING_AMOUNT", "KR", "1d", false, 0)
	if err != nil {
		t.Fatalf("Rankings: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].Symbol != "005930" || got.Items[0].LastPrice != 71900 {
		t.Fatalf("unexpected result: %+v", got)
	}
}

// ---------------------------------------------------------------------------
// Market indicator prices
// ---------------------------------------------------------------------------

func TestAdaptMarketIndicatorPricesUnit(t *testing.T) {
	raw := []apiMarketIndicatorPrice{
		{Symbol: "KOSPI", Timestamp: "2026-06-11T15:30:00+09:00", LastPrice: "2812.45"},
		{Symbol: "KOSDAQ", Timestamp: "", LastPrice: "845.1"},
	}
	got := adaptMarketIndicatorPrices(raw)
	if len(got.Indicators) != 2 {
		t.Fatalf("len: %d", len(got.Indicators))
	}
	if got.Indicators[0].Symbol != "KOSPI" || got.Indicators[0].LastPrice != 2812.45 {
		t.Fatalf("item0: %+v", got.Indicators[0])
	}
	if got.Indicators[1].Timestamp != "" || got.Indicators[1].LastPrice != 845.1 {
		t.Fatalf("item1 (null timestamp): %+v", got.Indicators[1])
	}
}

func TestMarketIndicatorPricesIntegration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/market-indicators/prices":
			if r.URL.Query().Get("symbols") != "KOSPI,KOSDAQ" {
				t.Errorf("symbols: got %q", r.URL.Query().Get("symbols"))
			}
			_, _ = w.Write([]byte(`{"result":[{"symbol":"KOSPI","timestamp":"2026-06-11T15:30:00+09:00","lastPrice":"2812.45"},{"symbol":"KOSDAQ","timestamp":null,"lastPrice":"845.1"}]}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := New(
		Credentials{APIKey: "k", SecretKey: "s"},
		filepath.Join(t.TempDir(), "t.json"),
		WithBaseURL(srv.URL),
		WithHTTPClient(srv.Client()),
	)
	got, err := c.MarketIndicatorPrices(context.Background(), []string{"KOSPI", "KOSDAQ"})
	if err != nil {
		t.Fatalf("MarketIndicatorPrices: %v", err)
	}
	if len(got.Indicators) != 2 || got.Indicators[1].Symbol != "KOSDAQ" || got.Indicators[1].Timestamp != "" {
		t.Fatalf("unexpected: %+v", got)
	}
}

func TestAdaptMarketIndicatorCandlesUnit(t *testing.T) {
	raw := apiMarketIndicatorCandlePage{
		Candles: []apiMarketIndicatorCandle{
			{Timestamp: "2026-06-11T09:00:00+09:00", OpenPrice: "2798.32", HighPrice: "2820.15", LowPrice: "2790.1", ClosePrice: "2812.45", Volume: "123456"},
		},
		NextBefore: "2026-06-10T09:00:00+09:00",
	}
	got := adaptMarketIndicatorCandles("KOSPI", "1d", raw)
	if got.Symbol != "KOSPI" || got.Interval != "1d" {
		t.Fatalf("meta: %+v", got)
	}
	if got.NextBefore != "2026-06-10T09:00:00+09:00" {
		t.Fatalf("NextBefore: %q", got.NextBefore)
	}
	if len(got.Candles) != 1 {
		t.Fatalf("len: %d", len(got.Candles))
	}
	c0 := got.Candles[0]
	if c0.Open != 2798.32 || c0.High != 2820.15 || c0.Low != 2790.1 || c0.Close != 2812.45 || c0.Volume != 123456 {
		t.Fatalf("candle0: %+v", c0)
	}
}

func TestMarketIndicatorCandlesIntegration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/market-indicators/KOSPI/candles":
			if r.URL.Query().Get("interval") != "1d" {
				t.Errorf("interval: got %q", r.URL.Query().Get("interval"))
			}
			if r.URL.Query().Get("count") != "5" {
				t.Errorf("count: got %q", r.URL.Query().Get("count"))
			}
			_, _ = w.Write([]byte(`{"result":{"candles":[{"timestamp":"2026-06-11T09:00:00+09:00","openPrice":"2798.32","highPrice":"2820.15","lowPrice":"2790.1","closePrice":"2812.45","volume":"123456"}],"nextBefore":null}}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := New(
		Credentials{APIKey: "k", SecretKey: "s"},
		filepath.Join(t.TempDir(), "t.json"),
		WithBaseURL(srv.URL),
		WithHTTPClient(srv.Client()),
	)
	got, err := c.MarketIndicatorCandles(context.Background(), "KOSPI", "1d", 5, "")
	if err != nil {
		t.Fatalf("MarketIndicatorCandles: %v", err)
	}
	if got.Symbol != "KOSPI" || len(got.Candles) != 1 || got.Candles[0].Close != 2812.45 || got.NextBefore != "" {
		t.Fatalf("unexpected: %+v", got)
	}
}

func TestAdaptInvestorTradingUnit(t *testing.T) {
	raw := apiInvestorTradingResult{
		NextUntil: "2026-06-09",
		Records: []apiInvestorTradingRecord{
			{
				Date: "2026-06-11", UpdatedAt: "2026-06-11T18:10:00+09:00",
				Individual:       apiInvestorTradingAmount{BuyAmount: "5200000000000", SellAmount: "5350000000000"},
				Foreigner:        apiInvestorTradingAmount{BuyAmount: "3800000000000", SellAmount: "3600000000000"},
				Institution:      apiInvestorTradingAmount{BuyAmount: "2100000000000", SellAmount: "2180000000000"},
				OtherCorporation: apiInvestorTradingAmount{BuyAmount: "50000000000", SellAmount: "40000000000"},
			},
		},
	}
	got := adaptInvestorTrading("KOSPI", "1d", raw)
	if got.Symbol != "KOSPI" || got.Interval != "1d" || got.NextUntil != "2026-06-09" {
		t.Fatalf("meta: %+v", got)
	}
	if len(got.Records) != 1 {
		t.Fatalf("records len: %d", len(got.Records))
	}
	r := got.Records[0]
	if r.Individual.BuyAmount != 5200000000000 || r.Individual.SellAmount != 5350000000000 {
		t.Fatalf("individual: %+v", r.Individual)
	}
	if r.Individual.NetAmount != -150000000000 { // buy - sell
		t.Fatalf("individual net: want -150000000000, got %v", r.Individual.NetAmount)
	}
	if r.Foreigner.NetAmount != 200000000000 {
		t.Fatalf("foreigner net: %v", r.Foreigner.NetAmount)
	}
}

func TestMarketInvestorTradingIntegration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/market-indicators/KOSPI/investor-trading":
			if r.URL.Query().Get("interval") != "1d" {
				t.Errorf("interval: got %q", r.URL.Query().Get("interval"))
			}
			_, _ = w.Write([]byte(`{"result":{"nextUntil":null,"records":[{"date":"2026-06-11","updatedAt":"2026-06-11T18:10:00+09:00","individual":{"buyAmount":"5200000000000","sellAmount":"5350000000000"},"foreigner":{"buyAmount":"3800000000000","sellAmount":"3600000000000"},"institution":{"buyAmount":"2100000000000","sellAmount":"2180000000000"},"otherCorporation":{"buyAmount":"50000000000","sellAmount":"40000000000"}}]}}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := New(
		Credentials{APIKey: "k", SecretKey: "s"},
		filepath.Join(t.TempDir(), "t.json"),
		WithBaseURL(srv.URL),
		WithHTTPClient(srv.Client()),
	)
	got, err := c.MarketInvestorTrading(context.Background(), "KOSPI", "1d", 0, "")
	if err != nil {
		t.Fatalf("MarketInvestorTrading: %v", err)
	}
	if got.Symbol != "KOSPI" || len(got.Records) != 1 || got.NextUntil != "" {
		t.Fatalf("unexpected: %+v", got)
	}
	if got.Records[0].Foreigner.NetAmount != 200000000000 {
		t.Fatalf("foreigner net: %v", got.Records[0].Foreigner.NetAmount)
	}
}
