package hybrid

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/client"
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
}
