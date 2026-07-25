package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
)

func TestInvestModeFor(t *testing.T) {
	cases := []struct {
		code             string
		wantView, wantIM string
	}{
		{"A005930", "krx_all", "krx"},           // KR stock
		{"A114800", "krx_all", "krx"},           // KR ETF
		{"US20220809012", "unified", "unified"}, // US stock code
		{"AAPL", "unified", "unified"},          // US ticker
	}
	for _, c := range cases {
		gv, gi := investModeFor(c.code)
		if gv != c.wantView || gi != c.wantIM {
			t.Errorf("investModeFor(%q) = (%q,%q), want (%q,%q)", c.code, gv, gi, c.wantView, c.wantIM)
		}
	}
}

// US symbols have no daily price band — GetPriceLimits must reject them with a
// clear message before any network call (a US product code looks-like a code so
// resolveProductCode returns it directly without hitting search).
func TestGetPriceLimitsRejectsUSSymbol(t *testing.T) {
	c := New(Config{})
	_, err := c.GetPriceLimits(context.Background(), "US19801212001")
	if err == nil {
		t.Fatal("expected error for US symbol, got nil")
	}
	if !strings.Contains(err.Error(), "KRX") {
		t.Errorf("expected KRX-only message, got: %v", err)
	}
}

func TestRunScreenerRawRejectsInvalidJSON(t *testing.T) {
	c := New(Config{})
	_, err := c.RunScreenerRaw(context.Background(), "not json", "kr", 5)
	if err == nil {
		t.Fatal("expected error for invalid JSON filter")
	}
	if !strings.Contains(err.Error(), "JSON") {
		t.Errorf("expected JSON validation message, got: %v", err)
	}
}

func TestGetTradingFlowsRejectsUSSymbol(t *testing.T) {
	c := New(Config{})
	_, err := c.GetTradingFlows(context.Background(), "US19801212001", 5)
	if err == nil {
		t.Fatal("expected error for US symbol, got nil")
	}
	if !strings.Contains(err.Error(), "KRX") {
		t.Errorf("expected KRX-only message, got: %v", err)
	}
}

// 프리셋의 filters 배열은 도메인까지 그대로 실려 나와야 한다.
//
// 이게 없으면 `--filter` 를 쓸 방법이 사실상 없다: 필터 어휘(한글 id)는 토스 web
// 번들에만 있어서, 프리셋을 꺼내 고치는 게 유일한 실전 경로다. 예전에는 여기서
// filters 가 버려져 help 가 안내하는 워크플로가 실제로는 불가능했다.
func TestScreenerPresetsCarryFilters(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":[{"id":"14","name":"더미 프리셋","description":"설명",
			"filters":[{"id":"배당_수익률","conditions":[{"id":"NUMBER_RANGE_DEFAULT",
			"type":"NUMBER_RANGE","value":{"from":0.01,"to":null}}]}]}]}`))
	}))
	defer srv.Close()

	c := New(Config{HTTPClient: srv.Client(), CertBaseURL: srv.URL,
		Session: &session.Session{Cookies: map[string]string{"SESSION": "s"}}})

	got, err := c.GetScreenerPresets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Presets) != 1 {
		t.Fatalf("프리셋 수 = %d, want 1", len(got.Presets))
	}
	p := got.Presets[0]
	if len(p.Filters) == 0 {
		t.Fatal("Filters 가 비었다 — 프리셋을 출발점으로 쓰는 워크플로가 깨진다")
	}
	if !strings.Contains(string(p.Filters), "배당_수익률") {
		t.Errorf("Filters 원본이 보존되지 않았다: %s", p.Filters)
	}
	// 꺼낸 그대로 --filter 로 되먹일 수 있어야 하므로 유효한 JSON 이어야 한다.
	if !json.Valid(p.Filters) {
		t.Error("Filters 가 유효한 JSON 이 아니다 — --filter 로 되먹일 수 없다")
	}
}
