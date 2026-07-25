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

// 전부 합성 더미 값이다 — 실계좌 데이터는 테스트에 넣지 않는다.

func periodClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return New(Config{
		HTTPClient:  srv.Client(),
		CertBaseURL: srv.URL,
		Session:     &session.Session{Cookies: map[string]string{"SESSION": "s"}},
	})
}

// 날짜를 안 주면 rangeType=all 로 전체 기간을, 주면 날짜 모드로 나가야 한다.
// rangeType 은 사용자에게 노출되지 않으므로 이 계약이 유일한 방어선이다.
func TestPeriodProfitRangeTypeContract(t *testing.T) {
	cases := []struct {
		name      string
		from, to  string
		wantRange string
		wantDates bool
	}{
		{"날짜 없음 → all", "", "", "all", false},
		{"날짜 있음 → 날짜 모드", "20260101", "20260725", "month", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got map[string]any
			c := periodClient(t, func(w http.ResponseWriter, r *http.Request) {
				b, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(b, &got)
				_, _ = w.Write([]byte(`{"result":{"earningAmount":{"krw":1000,"usd":1.5},
					"earningRate":{"krw":2.5},"purchaseAmount":{"krw":40000}}}`))
			})

			res, err := c.GetPeriodProfit(context.Background(), "sales", tc.from, tc.to)
			if err != nil {
				t.Fatal(err)
			}
			if got["rangeType"] != tc.wantRange {
				t.Errorf("rangeType = %v, want %q", got["rangeType"], tc.wantRange)
			}
			if _, has := got["startDate"]; has != tc.wantDates {
				t.Errorf("startDate 존재 = %v, want %v", has, tc.wantDates)
			}
			if got["profitType"] != "sales" {
				t.Errorf("profitType = %v", got["profitType"])
			}
			if res.EarningAmount.KRW != 1000 {
				t.Errorf("EarningAmount.KRW = %v, want 1000", res.EarningAmount.KRW)
			}
			if res.Type != "sales" {
				t.Errorf("Type = %q", res.Type)
			}
		})
	}
}

// 페이징은 last=true 까지 걷고, 호출은 통화 하나로 정확히 한 번만 한다.
//
// 라이브에서 확인: KRW/USD 두 호출은 동일한 행 집합을 돌려주고 profit_rate 만
// 다르다(기준 통화). 초기 구현은 둘 다 조회해 합쳤는데 그러면 모든 행이 중복됐다.
// 이 테스트가 그 회귀를 막는다.
func TestDailyProfitWalksEveryPageOnce(t *testing.T) {
	seen := map[string]int{}
	c := periodClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Currency string `json:"currency"`
			Page     int    `json:"page"`
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		seen[body.Currency]++

		if body.Page == 0 {
			_, _ = w.Write([]byte(`{"result":{"last":false,"content":[
				{"marketType":"kr","symbol":"000000","productName":"더미종목","baseDate":"20260102",
				 "quantity":3,"profitRate":1.5,
				 "profitLossAmount":{"krw":100},"sellAmount":{"krw":900},"buyAmount":{"krw":800}}]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"result":{"last":true,"content":[
			{"marketType":"kr","symbol":"111111","productName":"더미종목2","baseDate":"20260103",
			 "quantity":1,"profitRate":-0.5,
			 "profitLossAmount":{"krw":-50},"sellAmount":{"krw":100},"buyAmount":{"krw":150}}]}}`))
	})

	got, err := c.GetDailyProfit(context.Background(), "20260101", "20260131", "")
	if err != nil {
		t.Fatal(err)
	}
	// 2페이지 × 1통화 = 2행. 4행이면 두 통화를 합쳐 중복시킨 것이다.
	if len(got.Stocks) != 2 {
		t.Fatalf("행 수 = %d, want 2 (통화를 합치면 중복된다)", len(got.Stocks))
	}
	if seen[DefaultProfitCurrency] != 2 {
		t.Errorf("%s 요청 수 = %d, want 2 (last=true 까지)", DefaultProfitCurrency, seen[DefaultProfitCurrency])
	}
	if other := seen["USD"]; other != 0 {
		t.Errorf("기본값인데 USD 로도 %d 번 호출했다 — 행이 중복된다", other)
	}
	if got.Currency != DefaultProfitCurrency {
		t.Errorf("Currency = %q, want %q (수익률 기준을 기록해야 한다)", got.Currency, DefaultProfitCurrency)
	}
	if got.Stocks[0].Date != "2026-01-02" {
		t.Errorf("Date = %q, want 2026-01-02", got.Stocks[0].Date)
	}
}

// 통화를 지정하면 그 기준으로 호출하고, 결과에 기준을 기록한다.
func TestDailyProfitHonoursCurrencyBasis(t *testing.T) {
	var seen []string
	c := periodClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Currency string `json:"currency"`
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		seen = append(seen, body.Currency)
		_, _ = w.Write([]byte(`{"result":{"last":true,"content":[]}}`))
	})

	got, err := c.GetDailyProfit(context.Background(), "", "", "USD")
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != 1 || seen[0] != "USD" {
		t.Fatalf("호출된 통화 = %v, want [USD]", seen)
	}
	if got.Currency != "USD" {
		t.Errorf("Currency = %q, want USD", got.Currency)
	}
}

// 빈 페이지가 오면 last 플래그가 없어도 멈춘다 (무한 루프 방지).
func TestDailyProfitStopsOnEmptyPage(t *testing.T) {
	calls := 0
	c := periodClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"result":{"last":false,"content":[]}}`))
	})
	if _, err := c.GetDailyProfit(context.Background(), "", "", "KRW"); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("호출 수 = %d, want 1 — 빈 페이지에서 멈춰야 한다", calls)
	}
}

// 라이브에서 드러난 형식: 이 엔드포인트는 질의에 쓴 YYYYMMDD 를 그대로 돌려주지 않고
// 표시용 "YY.M.D"(월·일 패딩 없음)를 준다. 픽스처만 봤으면 놓쳤을 부분이라 고정한다.
func TestFormatBaseDate(t *testing.T) {
	cases := map[string]string{
		"26.7.15":  "2026-07-15", // 라이브 실측 형식
		"26.4.23":  "2026-04-23",
		"26.12.5":  "2026-12-05", // 두 자리 월
		"20260715": "2026-07-15", // 형제 엔드포인트가 쓰는 형식
		"":         "",           // 알 수 없는 형식은 손대지 않는다
		"weird":    "weird",
		"26.13.99": "26.13.99", // 범위를 벗어나면 원본 유지
	}
	for in, want := range cases {
		if got := formatBaseDate(in); got != want {
			t.Errorf("formatBaseDate(%q) = %q, want %q", in, got, want)
		}
	}
}
