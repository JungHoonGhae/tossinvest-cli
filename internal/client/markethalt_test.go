package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
)

// 전부 합성 더미.
//
// 마지막 항목은 토스가 나중에 추가할 수 있는 **모르는 시장·유형**이다. 매핑에 없는
// 값이 조용히 사라지지 않고 원문 그대로 살아남는지 고정한다 — 새 halt 유형이 안 보이면
// "정상" 으로 오독된다.
const dummyIndicator = `{"result":{"marketEvents":[
 {"market":"KSP","type":"CIRCUIT_BREAKER","activated":false},
 {"market":"KSP","type":"SIDECAR","activated":true},
 {"market":"KSQ","type":"CIRCUIT_BREAKER","activated":false},
 {"market":"XXX","type":"BRAND_NEW_HALT","activated":true}
],"indicators":[],"landingUrl":"https://example.invalid"}}`

func haltServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/dashboard/wts/overview/indicator" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(dummyIndicator))
	}))
}

func newHaltClient(srv *httptest.Server) *Client {
	return New(Config{HTTPClient: srv.Client(), CertBaseURL: srv.URL,
		Session: &session.Session{Cookies: map[string]string{"SESSION": "s"}}})
}

func TestGetMarketHalt(t *testing.T) {
	srv := haltServer(t)
	defer srv.Close()

	got, err := newHaltClient(srv).GetMarketHalt(context.Background())
	if err != nil {
		t.Fatalf("GetMarketHalt: %v", err)
	}
	if len(got.Events) != 4 {
		t.Fatalf("expected 4 events, got %d", len(got.Events))
	}
	if got.Events[0].MarketName != "KOSPI" || got.Events[0].Type != "circuit_breaker" {
		t.Errorf("KSP/CIRCUIT_BREAKER mapped wrong: %+v", got.Events[0])
	}
	if !got.Events[1].Activated || got.Events[1].Type != "sidecar" {
		t.Errorf("sidecar firing lost: %+v", got.Events[1])
	}
	if got.Events[2].MarketName != "KOSDAQ" {
		t.Errorf("KSQ should map to KOSDAQ: %+v", got.Events[2])
	}
	// 모르는 값은 원문 유지.
	if got.Events[3].MarketName != "XXX" || got.Events[3].Type != "BRAND_NEW_HALT" {
		t.Errorf("unknown market/type should survive verbatim: %+v", got.Events[3])
	}
	if !got.Halted() {
		t.Error("Halted() must be true when any switch is firing")
	}
}

func TestMarketHaltNotHalted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"marketEvents":[
 {"market":"KSP","type":"CIRCUIT_BREAKER","activated":false}]}}`))
	}))
	defer srv.Close()

	got, err := newHaltClient(srv).GetMarketHalt(context.Background())
	if err != nil {
		t.Fatalf("GetMarketHalt: %v", err)
	}
	if got.Halted() {
		t.Error("Halted() must be false when nothing is firing")
	}
}
