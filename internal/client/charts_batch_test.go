package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
)

// 전부 합성 더미.
//
// 이 엔드포인트는 **데이터가 없는 종목을 오류 없이 빼고 준다.** 빈 차트로 채우면
// "데이터 없음" 과 "요청 실패" 가 구별되지 않으므로 missing 으로 분리해 돌려준다.
func TestGetStockChartsReportsOmittedSymbols(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"result":{"baseRange":"1d","baseStep":"10m","miniCharts":[
 {"code":"A000001","timezone":"Asia/Seoul","candles":[
   {"startDate":"2026-01-02T00:00:00Z","endDate":"2026-01-02T00:10:00Z",
    "open":100,"high":110,"low":95,"close":105,"base":100}]}
]}}`))
	}))
	defer srv.Close()

	c := New(Config{HTTPClient: srv.Client(), CertBaseURL: srv.URL,
		Session: &session.Session{Cookies: map[string]string{"SESSION": "s"}}})
	charts, missing, err := c.GetStockCharts(context.Background(), []string{"A000001", "A000002"})
	if err != nil {
		t.Fatalf("GetStockCharts: %v", err)
	}

	var sent struct {
		Codes []string `json:"codes"`
	}
	if err := json.Unmarshal(gotBody, &sent); err != nil {
		t.Fatalf("request body not JSON: %v", err)
	}
	if len(sent.Codes) != 2 {
		t.Errorf("both codes must go in ONE request, got %v", sent.Codes)
	}

	if len(charts) != 1 {
		t.Fatalf("expected 1 chart, got %d", len(charts))
	}
	if charts[0].Interval != "10m" {
		t.Errorf("Interval should carry the server's step, got %q", charts[0].Interval)
	}
	if charts[0].Base != 100 {
		t.Errorf("Base should be lifted off the candles, got %v", charts[0].Base)
	}
	if n := len(charts[0].Candles); n != 1 {
		t.Fatalf("expected 1 candle, got %d", n)
	}
	if charts[0].Candles[0].Close != 105 {
		t.Errorf("candle OHLC lost: %+v", charts[0].Candles[0])
	}
	// 빠진 종목은 빈 차트가 아니라 missing 으로 나와야 한다.
	if len(missing) != 1 || missing[0] != "A000002" {
		t.Errorf("omitted symbol must be reported, got missing=%v", missing)
	}
}
