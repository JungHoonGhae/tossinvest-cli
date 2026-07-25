package output

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

func usdPtr(v float64) *float64 { return &v }

// 날짜는 API 의 YYYYMMDD 로 들어오고 사람에게는 YYYY-MM-DD 로 보여야 한다.
func TestPeriodLabelFormatsDates(t *testing.T) {
	cases := map[[2]string]string{
		{"", ""}:                 "전체 기간",
		{"20260101", "20260725"}: "2026-01-01 ~ 2026-07-25",
	}
	for in, want := range cases {
		if got := periodLabel(in[0], in[1]); got != want {
			t.Errorf("periodLabel(%q,%q) = %q, want %q", in[0], in[1], got, want)
		}
	}
}

func TestWritePeriodProfit(t *testing.T) {
	p := domain.PeriodProfit{
		Type: "sales", From: "20260101", To: "20260725",
		EarningAmount:  domain.DualCurrency{KRW: -1421, USD: usdPtr(-1.02)},
		EarningRate:    domain.DualCurrency{KRW: -66.9},
		PurchaseAmount: domain.DualCurrency{KRW: 2124},
		FetchedAt:      time.Now(),
	}
	var buf bytes.Buffer
	if err := WritePeriodProfit(&buf, FormatTable, p); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"sales", "2026-01-01 ~ 2026-07-25", "수익률", "-66.90"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q 가 없다:\n%s", want, out)
		}
	}
	// USD 가 없으면 대시로
	p.EarningAmount.USD = nil
	var buf2 bytes.Buffer
	if err := WritePeriodProfit(&buf2, FormatTable, p); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf2.String(), "-") {
		t.Error("USD 부재 표시가 없다")
	}
}

// 통화는 행마다가 아니라 헤더에 한 번 — 조회 전체의 기준이기 때문.
func TestWriteDailyProfitShowsBasisInHeader(t *testing.T) {
	d := domain.DailyProfit{
		From: "20260101", To: "20260725", Currency: "KRW",
		Stocks: []domain.DailyProfitStock{
			{Date: "2026-07-15", Symbol: "A000000", Name: "더미종목", Quantity: 10,
				ProfitLoss: domain.DualCurrency{KRW: -1000}, ProfitRate: -12.13},
			{Date: "2026-06-11", Symbol: "DUMMY", Name: "더미2", Quantity: 46,
				ProfitLoss: domain.DualCurrency{KRW: 500}, ProfitRate: 3.2},
		},
		FetchedAt: time.Now(),
	}
	var buf bytes.Buffer
	if err := WriteDailyProfit(&buf, FormatTable, d); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "수익률 기준 KRW") {
		t.Errorf("기준 통화가 헤더에 없다:\n%s", out)
	}
	if strings.Count(out, "KRW") > 3 {
		t.Error("기준 통화가 행마다 반복되는 것 같다 — 헤더에 한 번이면 된다")
	}
	// 합계가 맞아야 한다
	if !strings.Contains(out, "합계") || !strings.Contains(out, "-500") {
		t.Errorf("합계(-1000+500=-500)가 틀렸다:\n%s", out)
	}
}

func TestWriteDailyProfitEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteDailyProfit(&buf, FormatTable,
		domain.DailyProfit{From: "20260101", To: "20260131", Currency: "KRW"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "없습니다") {
		t.Errorf("빈 결과 안내가 없다: %s", buf.String())
	}
}
