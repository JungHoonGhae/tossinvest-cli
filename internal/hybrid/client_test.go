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
	c := New(nil, nil, Policy{}, nil)

	if _, err := c.BuyingPower(context.Background(), "KRW"); !errors.Is(err, ErrOfficialKeyRequired) {
		t.Errorf("BuyingPower: want ErrOfficialKeyRequired, got %v", err)
	}
	if _, err := c.MarketCalendar(context.Background(), "KR", ""); !errors.Is(err, ErrOfficialKeyRequired) {
		t.Errorf("MarketCalendar: want ErrOfficialKeyRequired, got %v", err)
	}
	if _, err := c.Stocks(context.Background(), []string{"AAPL"}); !errors.Is(err, ErrOfficialKeyRequired) {
		t.Errorf("Stocks: want ErrOfficialKeyRequired, got %v", err)
	}
	if _, err := c.Rankings(context.Background(), "MARKET_TRADING_AMOUNT", "KR", "1d", false, 0); !errors.Is(err, ErrOfficialKeyRequired) {
		t.Errorf("Rankings: want ErrOfficialKeyRequired, got %v", err)
	}
	if _, err := c.MarketIndicatorPrices(context.Background(), []string{"KOSPI"}); !errors.Is(err, ErrOfficialKeyRequired) {
		t.Errorf("MarketIndicatorPrices: want ErrOfficialKeyRequired, got %v", err)
	}
	if _, err := c.MarketIndicatorCandles(context.Background(), "KOSPI", "1d", 5, ""); !errors.Is(err, ErrOfficialKeyRequired) {
		t.Errorf("MarketIndicatorCandles: want ErrOfficialKeyRequired, got %v", err)
	}
	if _, err := c.MarketInvestorTrading(context.Background(), "KOSPI", "1d", 0, ""); !errors.Is(err, ErrOfficialKeyRequired) {
		t.Errorf("MarketInvestorTrading: want ErrOfficialKeyRequired, got %v", err)
	}
	if _, err := c.ConditionalOrders(context.Background(), "", "", "", 0); !errors.Is(err, ErrOfficialKeyRequired) {
		t.Errorf("ConditionalOrders: want ErrOfficialKeyRequired, got %v", err)
	}
	if _, err := c.ConditionalOrder(context.Background(), "co-1"); !errors.Is(err, ErrOfficialKeyRequired) {
		t.Errorf("ConditionalOrder: want ErrOfficialKeyRequired, got %v", err)
	}
	if err := c.CancelConditionalOrder(context.Background(), orderintent.ConditionalCancelIntent{ID: "co-1"}); !errors.Is(err, ErrOfficialKeyRequired) {
		t.Errorf("CancelConditionalOrder: want ErrOfficialKeyRequired, got %v", err)
	}
	if _, err := c.CreateConditionalOrder(context.Background(), orderintent.ConditionalPlaceIntent{Symbol: "005930", Type: "SINGLE"}); !errors.Is(err, ErrOfficialKeyRequired) {
		t.Errorf("CreateConditionalOrder: want ErrOfficialKeyRequired, got %v", err)
	}
	if err := c.ModifyConditionalOrder(context.Background(), orderintent.ConditionalModifyIntent{ID: "co-1", Type: "SINGLE"}); !errors.Is(err, ErrOfficialKeyRequired) {
		t.Errorf("ModifyConditionalOrder: want ErrOfficialKeyRequired, got %v", err)
	}
}

func TestNewOfficialOnlyReadsRespectPinnedWTS(t *testing.T) {
	// A caller can construct the router with an official adapter and still pin
	// this run to WTS. Official-only reads must respect the policy rather than
	// escaping through the raw adapter.
	c := New(nil, &official.Client{}, Policy{Prefer: "wts"}, nil)

	if _, err := c.BuyingPower(context.Background(), "KRW"); !errors.Is(err, ErrOfficialKeyRequired) {
		t.Errorf("BuyingPower: want ErrOfficialKeyRequired, got %v", err)
	}
	if _, err := c.MarketCalendar(context.Background(), "KR", ""); !errors.Is(err, ErrOfficialKeyRequired) {
		t.Errorf("MarketCalendar: want ErrOfficialKeyRequired, got %v", err)
	}
	if _, err := c.Stocks(context.Background(), []string{"AAPL"}); !errors.Is(err, ErrOfficialKeyRequired) {
		t.Errorf("Stocks: want ErrOfficialKeyRequired, got %v", err)
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
