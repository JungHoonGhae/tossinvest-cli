package hybrid

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/client"
	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/official"
	"github.com/JungHoonGhae/tossinvest-cli/internal/orderintent"
	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
)

// route is the keystone: it decides official-vs-WTS and emits a fallback notice
// only when it actually falls back. These tests exercise route directly with
// string closures so no live client is required.

func TestRoutePrefersOfficialHappyPathIsSilent(t *testing.T) {
	var buf bytes.Buffer
	c := &Client{off: &official.Client{}, pol: Policy{Prefer: "auto", Fallback: true}, stderr: &buf}

	got, err := route(c,
		func() (string, error) { return "official", nil },
		func() (string, error) { t.Fatal("wts must not be called on official success"); return "", nil })

	if err != nil || got != "official" {
		t.Fatalf("want official,nil; got %q,%v", got, err)
	}
	if buf.Len() != 0 {
		t.Fatalf("happy path must be silent on stderr; got %q", buf.String())
	}
}

func TestRouteFallsBackOnServerError(t *testing.T) {
	var buf bytes.Buffer
	c := &Client{off: &official.Client{}, pol: Policy{Prefer: "auto", Fallback: true}, stderr: &buf}

	got, err := route(c,
		func() (string, error) { return "", official.ErrServer },
		func() (string, error) { return "wts", nil })

	if err != nil || got != "wts" {
		t.Fatalf("want wts fallback; got %q,%v", got, err)
	}
	if !strings.Contains(buf.String(), "falling back") {
		t.Fatalf("missing fallback notice on stderr; got %q", buf.String())
	}
}

func TestRouteFallbackWithNilWriter(t *testing.T) {
	c := &Client{off: &official.Client{}, pol: Policy{Fallback: true}}
	wtsCalls := 0
	got, err := route(c,
		func() (string, error) { return "", official.ErrServer },
		func() (string, error) { wtsCalls++; return "wts", nil })
	if err != nil || got != "wts" || wtsCalls != 1 {
		t.Fatalf("fallback = %q, %v; WTS calls = %d", got, err, wtsCalls)
	}
}

func TestRouteDoesNotFallbackOnDomainError(t *testing.T) {
	var buf bytes.Buffer
	c := &Client{off: &official.Client{}, pol: Policy{Prefer: "auto", Fallback: true}, stderr: &buf}
	domainErr := &official.APIError{Code: 404, Body: "not found"}

	got, err := route(c,
		func() (string, error) { return "", domainErr },
		func() (string, error) { t.Fatal("wts must not be called on domain error"); return "", nil })

	if got != "" {
		t.Fatalf("want empty value; got %q", got)
	}
	if !errors.Is(err, domainErr) {
		t.Fatalf("want domain error returned as-is; got %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("domain error path must be silent on stderr; got %q", buf.String())
	}
}

func TestRoutePreferWTSNeverCallsOfficial(t *testing.T) {
	c := &Client{off: &official.Client{}, pol: Policy{Prefer: "wts", Fallback: true}, stderr: io.Discard}

	got, err := route(c,
		func() (string, error) { t.Fatal("official must not be called when prefer=wts"); return "", nil },
		func() (string, error) { return "wts", nil })

	if err != nil || got != "wts" {
		t.Fatalf("want wts; got %q,%v", got, err)
	}
}

func TestRouteOffNilIsWTSPassthrough(t *testing.T) {
	c := &Client{off: nil, pol: Policy{Prefer: "auto", Fallback: true}, stderr: io.Discard}

	got, err := route(c,
		func() (string, error) { t.Fatal("official must not be called when off==nil"); return "", nil },
		func() (string, error) { return "wts", nil })

	if err != nil || got != "wts" {
		t.Fatalf("want wts; got %q,%v", got, err)
	}
}

func TestRouteFallbackDisabledReturnsServerError(t *testing.T) {
	var buf bytes.Buffer
	c := &Client{off: &official.Client{}, pol: Policy{Prefer: "auto", Fallback: false}, stderr: &buf}

	_, err := route(c,
		func() (string, error) { return "", official.ErrServer },
		func() (string, error) { t.Fatal("wts must not be called when fallback disabled"); return "", nil })

	if !errors.Is(err, official.ErrServer) {
		t.Fatalf("want ErrServer returned when fallback disabled; got %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("no fallback means no notice; got %q", buf.String())
	}
}

// New defaults a nil stderr to io.Discard so overrides never panic.
func TestNewDefaultsStderr(t *testing.T) {
	c := New(nil, nil, Policy{Prefer: "auto"}, nil)
	if c.stderr == nil {
		t.Fatal("New must default nil stderr to a non-nil writer")
	}
}

func TestPinnedWTSHidesOfficialClient(t *testing.T) {
	c := New(nil, &official.Client{}, Policy{Prefer: "wts"}, nil)
	if c.Official() != nil {
		t.Fatal("Official must be nil when policy pins the client to WTS")
	}
}

// Integration-style: with off==nil, an overridden method must pass through to
// the embedded WTS client. We back the WTS client with httptest and assert the
// orderbook value comes from the web-session path.
func TestGetOrderBookPassthroughWhenOfficialAbsent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/quotes") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"result":{"close":71000,"offerPrices":[71100],"offerVolumes":[10],"bidPrices":[71000],"bidVolumes":[20],"offerVolume":10,"bidVolume":20}}`)
			return
		}
		// stock-infos endpoint: error is ignored by GetOrderBook, respond empty.
		_, _ = io.WriteString(w, `{"result":{}}`)
	}))
	defer server.Close()

	wts := client.New(client.Config{InfoBaseURL: server.URL})
	c := New(wts, nil, Policy{Prefer: "auto", Fallback: true}, io.Discard)

	ob, err := c.GetOrderBook(context.Background(), "A005930")
	if err != nil {
		t.Fatalf("GetOrderBook passthrough failed: %v", err)
	}
	if ob.Close != 71000 {
		t.Fatalf("want close from WTS path 71000; got %v", ob.Close)
	}
	if ob.ProductCode != "A005930" {
		t.Fatalf("want product code A005930; got %q", ob.ProductCode)
	}
}

func TestOfficialOnlyReadsRequireKey(t *testing.T) {
	// off == nil simulates "no official credentials connected".
	assertOfficialOnlyUnavailable(t, New(nil, nil, Policy{}, nil))
}

func TestOfficialOnlyReadsRespectPinnedWTS(t *testing.T) {
	// A caller can construct the router with an official adapter and still pin
	// this run to WTS. Official-only reads must respect the policy rather than
	// escaping through the raw adapter.
	assertOfficialOnlyUnavailable(t, New(nil, &official.Client{}, Policy{Prefer: "wts"}, nil))
}

func assertOfficialOnlyUnavailable(t *testing.T, c *Client) {
	t.Helper()
	ctx := context.Background()
	calls := []struct {
		name string
		call func() error
	}{
		{"BuyingPower", func() error { _, err := c.BuyingPower(ctx, "KRW"); return err }},
		{"MarketCalendar", func() error { _, err := c.MarketCalendar(ctx, "KR", ""); return err }},
		{"Stocks", func() error { _, err := c.Stocks(ctx, []string{"AAPL"}); return err }},
		{"Rankings", func() error { _, err := c.Rankings(ctx, "MARKET_TRADING_AMOUNT", "KR", "1d", false, 0); return err }},
		{"MarketIndicatorPrices", func() error { _, err := c.MarketIndicatorPrices(ctx, []string{"KOSPI"}); return err }},
		{"MarketIndicatorCandles", func() error { _, err := c.MarketIndicatorCandles(ctx, "KOSPI", "1d", 5, ""); return err }},
		{"MarketInvestorTrading", func() error { _, err := c.MarketInvestorTrading(ctx, "KOSPI", "1d", 0, ""); return err }},
		{"ConditionalOrders", func() error { _, err := c.ConditionalOrders(ctx, "", "", "", 0); return err }},
		{"ConditionalOrder", func() error { _, err := c.ConditionalOrder(ctx, "co-1"); return err }},
		{"CancelConditionalOrder", func() error { return c.CancelConditionalOrder(ctx, orderintent.ConditionalCancelIntent{ID: "co-1"}) }},
		{"CreateConditionalOrder", func() error {
			_, err := c.CreateConditionalOrder(ctx, orderintent.ConditionalPlaceIntent{Symbol: "005930", ConditionalShape: orderintent.ConditionalShape{Type: "SINGLE"}})
			return err
		}},
		{"ModifyConditionalOrder", func() error {
			_, err := c.ModifyConditionalOrder(ctx, orderintent.ConditionalModifyIntent{ID: "co-1", ConditionalShape: orderintent.ConditionalShape{Type: "SINGLE"}})
			return err
		}},
		{"Supply", func() error { _, err := c.Supply(ctx, "005930", domain.SupplyInvestor, 0, ""); return err }},
		{"InvestorTradingRankings", func() error { _, err := c.InvestorTradingRankings(ctx, 0); return err }},
		{"ListStocks", func() error { _, err := c.ListStocks(ctx, "KOSPI", "", "", false); return err }},
	}
	for _, tc := range calls {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); !errors.Is(err, ErrOfficialAccessRequired) {
				t.Errorf("want ErrOfficialAccessRequired, got %v", err)
			}
		})
	}
}

func TestNewOfficialOnlyReadsDelegateToOfficial(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = io.WriteString(w, `{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`)
		case "/api/v1/buying-power":
			if got := r.Header.Get("X-Tossinvest-Account"); got != "7" {
				t.Errorf("account header = %q, want 7", got)
			}
			_, _ = io.WriteString(w, `{"result":{"cashBuyingPower":"12345.5","currency":"KRW"}}`)
		case "/api/v1/market-calendar/KR":
			_, _ = io.WriteString(w, `{"result":{"today":{"date":"2026-09-01","integrated":{"regularMarket":{"startTime":"2026-09-01T09:00:00+09:00","endTime":"2026-09-01T15:30:00+09:00"}}}}}`)
		case "/api/v1/stocks":
			if got := r.URL.Query().Get("symbols"); got != "AAPL,005930" {
				t.Errorf("symbols query = %q, want AAPL,005930", got)
			}
			_, _ = io.WriteString(w, `{"result":[{"symbol":"AAPL","name":"애플","englishName":"Apple Inc.","isinCode":"US0378331005","market":"NASDAQ","securityType":"STOCK","isCommonShare":true,"status":"ACTIVE","currency":"USD","sharesOutstanding":"14702703000"}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	off := official.New(
		official.Credentials{APIKey: "k", SecretKey: "s"},
		filepath.Join(t.TempDir(), "token.json"),
		official.WithBaseURL(srv.URL),
		official.WithHTTPClient(srv.Client()),
		official.WithAccountSeq(7),
	)
	c := New(nil, off, Policy{Prefer: "auto"}, nil)

	power, err := c.BuyingPower(context.Background(), "KRW")
	if err != nil || power.Currency != "KRW" || power.CashBuyingPower != 12345.5 {
		t.Fatalf("BuyingPower delegation = %+v, %v", power, err)
	}
	calendar, err := c.MarketCalendar(context.Background(), "KR", "")
	if err != nil || calendar.Today.Date != "2026-09-01" || calendar.Today.Holiday {
		t.Fatalf("MarketCalendar delegation = %+v, %v", calendar, err)
	}
	stocks, err := c.Stocks(context.Background(), []string{"AAPL", "005930"})
	if err != nil || len(stocks) != 1 || stocks[0].ISINCode != "US0378331005" {
		t.Fatalf("Stocks delegation = %+v, %v", stocks, err)
	}
}

func TestExchangeRatesPinnedToWTSNeverFallsBackToOfficial(t *testing.T) {
	officialRequests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		officialRequests++
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = io.WriteString(w, `{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`)
		case "/api/v1/exchange-rates":
			_, _ = io.WriteString(w, `{"result":{"baseCurrency":"USD","quoteCurrency":"KRW","rate":"1300"}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	wtsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forced WTS failure", http.StatusInternalServerError)
	}))
	defer wtsSrv.Close()
	wts := client.New(client.Config{InfoBaseURL: wtsSrv.URL, HTTPClient: wtsSrv.Client()})
	off := official.New(
		official.Credentials{APIKey: "k", SecretKey: "s"},
		filepath.Join(t.TempDir(), "token.json"),
		official.WithBaseURL(srv.URL),
		official.WithHTTPClient(srv.Client()),
	)
	c := New(wts, off, Policy{Prefer: "wts"}, nil)

	if _, err := c.GetExchangeRates(context.Background()); err == nil {
		t.Fatal("pinned WTS failure must be returned instead of using official")
	}
	if officialRequests != 0 {
		t.Fatalf("pinned WTS must make zero official requests, got %d", officialRequests)
	}
}

// 공식 API 는 심볼 정규식이 `^[A-Za-z0-9.,\-]+$` 라 언더스코어를 거부한다. 옵션 계약
// guid 가 정확히 거기 걸리는데, 그 400 은 도메인 에러라 폴백이 안 걸린다 — WTS 는
// 멀쩡히 서빙하는데도 요청이 거기서 죽는다. 아예 보내지 않는지 본다.
func TestRouteSymbolSkipsOfficialForOptionGuid(t *testing.T) {
	officialCalled := false
	wtsCalled := false
	c := &Client{off: &official.Client{}, pol: Policy{Prefer: "auto", Fallback: true}, stderr: io.Discard}

	got, err := routeSymbol(c, "OPT_AAPL260805C00230000_20260722",
		func() (string, error) { officialCalled = true; return "official", nil },
		func() (string, error) { wtsCalled = true; return "wts", nil })

	if err != nil {
		t.Fatalf("routeSymbol: %v", err)
	}
	if officialCalled {
		t.Error("official was called with a symbol it cannot express")
	}
	if !wtsCalled || got != "wts" {
		t.Errorf("want WTS result, got %q (wtsCalled=%v)", got, wtsCalled)
	}
}

func TestRouteSymbolUsesOfficialForPlainTicker(t *testing.T) {
	for _, sym := range []string{"AAPL", "005930", "BRK.B", "TSLA-X"} {
		officialCalled := false
		c := &Client{off: &official.Client{}, pol: Policy{Prefer: "auto", Fallback: true}, stderr: io.Discard}
		got, err := routeSymbol(c, sym,
			func() (string, error) { officialCalled = true; return "official", nil },
			func() (string, error) { return "wts", nil })
		if err != nil {
			t.Fatalf("%s: %v", sym, err)
		}
		if !officialCalled || got != "official" {
			t.Errorf("%s: routed away from official (got %q)", sym, got)
		}
	}
}

// 공식 investor-trading 은 WTS 보다 풍부하지만 quote flows 가 찍는 3개 필드로 좁혀
// 들어간다. 좁히는 과정에서 값이 뒤바뀌지 않는지 본다.
func TestSupplyToFlowsNarrowsCorrectly(t *testing.T) {
	s := domain.SupplySeries{FetchedAt: time.Now(), Records: []domain.SupplyRecord{
		{Date: "2026-01-05",
			Individual:  &domain.TradingVolume{NetBuy: -100},
			Foreigner:   &domain.TradingVolume{NetBuy: 60},
			Institution: &domain.TradingVolume{NetBuy: 40}},
		// 잠정 기록: 개인이 아직 없다. domain.TradingFlow 는 평범한 float 라
		// 0 이 되는데, 그 손실은 quote supply 로 가면 복구된다.
		{Date: "2026-01-06", Foreigner: &domain.TradingVolume{NetBuy: 5}},
	}}
	got := supplyToFlows("005930", s)
	if got.Symbol != "005930" || len(got.Flows) != 2 {
		t.Fatalf("shape wrong: %+v", got)
	}
	f := got.Flows[0]
	if f.NetIndividuals != -100 || f.NetForeigner != 60 || f.NetInstitution != 40 {
		t.Errorf("values crossed: %+v", f)
	}
	if got.Flows[1].NetIndividuals != 0 {
		t.Errorf("missing individual should be 0 here, got %v", got.Flows[1].NetIndividuals)
	}
}

// 공식 키가 없으면 수급은 안내 메시지를 줘야 한다 — WTS 에 대응 표면이 없다.
func TestSupplyWithoutOfficialKey(t *testing.T) {
	c := &Client{pol: Policy{Prefer: "auto"}, stderr: io.Discard}
	if _, err := c.Supply(context.Background(), "005930", domain.SupplyShort, 0, ""); err == nil {
		t.Fatal("want error without official credentials")
	}
}

func TestModifyConditionalOrderExplainsModifyRestricted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = io.WriteString(w, `{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`)
		case "/api/v1/conditional-orders/co-1/modify":
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = io.WriteString(w, `{"error":{"code":"modify-restricted","message":"해당 주문은 정정할 수 없습니다.","requestId":"req-1"}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	off := official.New(
		official.Credentials{APIKey: "k", SecretKey: "s"},
		filepath.Join(t.TempDir(), "token.json"),
		official.WithBaseURL(srv.URL),
		official.WithHTTPClient(srv.Client()),
		official.WithAccountSeq(7),
	)
	c := New(nil, off, Policy{Prefer: "auto"}, nil)

	_, err := c.ModifyConditionalOrder(context.Background(), orderintent.ConditionalModifyIntent{ID: "co-1", ConditionalShape: orderintent.ConditionalShape{Type: "SINGLE"}})
	var apiErr *official.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != 422 {
		t.Fatalf("want wrapped 422 APIError, got %v", err)
	}
	for _, want := range []string{"WATCHING or PAUSED", "order conditional get co-1", "does NOT cancel"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error lacks %q: %v", want, err)
		}
	}
}

// TestOfficialSupportedChart guards issue #9: the official candles endpoint
// serves only 1m/1d up to 200 bars. Anything else must go straight to WTS —
// the official 400 is a domain error with no fallback.
func TestOfficialSupportedChart(t *testing.T) {
	cases := []struct {
		interval string
		count    int
		want     bool
	}{
		{"1m", 30, true},
		{"1d", 200, true},
		{"1m", 0, true}, // 0 means server default: within cap
		{"3m", 30, false},
		{"5m", 30, false},
		{"60m", 30, false},
		{"1m", 201, false},
		{"1d", 1000, false},
		{"1h", 30, false},
		{"", 30, false},
	}
	for _, tc := range cases {
		if got := officialSupportedChart(tc.interval, tc.count); got != tc.want {
			t.Errorf("officialSupportedChart(%q, %d) = %v, want %v", tc.interval, tc.count, got, tc.want)
		}
	}
}

// TestGetChartSkipsOfficialForUnsupportedInterval proves a 3m request never
// touches the official endpoint: the official mock fails the test on any hit
// while the WTS mock serves search, stock-info, and chart data.
func TestGetChartSkipsOfficialForUnsupportedInterval(t *testing.T) {
	offSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("official must not be called for 3m, got %s", r.URL.Path)
	}))
	t.Cleanup(offSrv.Close)
	wtsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v2/search/stocks":
			_, _ = w.Write([]byte(`{"result":{"stocks":[{"stockCode":"US20220809012","stockName":"TSLL"}]}}`))
		case r.URL.Path == "/api/v2/stock-infos/US20220809012":
			_, _ = w.Write([]byte(`{"result":{"symbol":"TSLL","name":"TSLL","currency":"USD","status":"N","market":{"code":"NSQ","displayName":"NASDAQ"}}}`))
		case r.URL.Path == "/api/v1/c-chart/us-s/US20220809012/min:3":
			_, _ = w.Write([]byte(`{"result":{"code":"US20220809012","exchange":"NSQ","candles":[{"dt":"2026-03-11T09:30:00+09:00","base":14.4,"open":14.4,"high":14.5,"low":14.3,"close":14.45,"volume":1000}]}}`))
		default:
			t.Fatalf("unexpected WTS request: %s", r.URL.Path)
		}
	}))
	t.Cleanup(wtsSrv.Close)

	off := official.New(official.Credentials{APIKey: "k", SecretKey: "s"},
		filepath.Join(t.TempDir(), "t.json"),
		official.WithBaseURL(offSrv.URL), official.WithHTTPClient(offSrv.Client()))
	wts := client.New(client.Config{HTTPClient: wtsSrv.Client(),
		APIBaseURL: wtsSrv.URL, InfoBaseURL: wtsSrv.URL, CertBaseURL: wtsSrv.URL,
		Session: &session.Session{Cookies: map[string]string{"SESSION": "x"},
			Headers: map[string]string{"App-Version": "test"},
			Storage: map[string]string{"localStorage:qr-tabId": "tab"}}})
	h := New(wts, off, Policy{Prefer: "auto", Fallback: true}, io.Discard)

	chart, err := h.GetChart(context.Background(), "AAPL", "3m", 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(chart.Candles) != 1 || chart.Interval != "3m" || chart.Symbol != "TSLL" {
		t.Fatalf("unexpected WTS chart: %+v", chart)
	}
}

// TestGetChartUsesOfficialForSupportedInterval proves 1m still prefers the
// official endpoint (adjusted like the WTS path) when credentials exist.
func TestGetChartUsesOfficialForSupportedInterval(t *testing.T) {
	offSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/candles":
			if got := r.URL.Query().Get("adjusted"); got != "true" {
				t.Fatalf("official chart must request adjusted candles, got adjusted=%q", got)
			}
			_, _ = w.Write([]byte(`{"result":{"candles":[{"timestamp":"2026-03-11T09:31:00+09:00","openPrice":"100","highPrice":"101","lowPrice":"99","closePrice":"100.5","volume":"10","currency":"USD"}],"nextBefore":""}}`))
		default:
			t.Fatalf("unexpected official request: %s", r.URL.Path)
		}
	}))
	t.Cleanup(offSrv.Close)
	wtsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("WTS must not be called for supported 1m, got %s", r.URL.Path)
	}))
	t.Cleanup(wtsSrv.Close)

	off := official.New(official.Credentials{APIKey: "k", SecretKey: "s"},
		filepath.Join(t.TempDir(), "t.json"),
		official.WithBaseURL(offSrv.URL), official.WithHTTPClient(offSrv.Client()))
	wts := client.New(client.Config{HTTPClient: wtsSrv.Client(),
		APIBaseURL: wtsSrv.URL, InfoBaseURL: wtsSrv.URL, CertBaseURL: wtsSrv.URL,
		Session: &session.Session{Cookies: map[string]string{"SESSION": "x"},
			Headers: map[string]string{"App-Version": "test"},
			Storage: map[string]string{"localStorage:qr-tabId": "tab"}}})
	h := New(wts, off, Policy{Prefer: "auto", Fallback: true}, io.Discard)

	chart, err := h.GetChart(context.Background(), "AAPL", "1m", 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(chart.Candles) != 1 || chart.Interval != "1m" {
		t.Fatalf("unexpected official chart: %+v", chart)
	}
}
