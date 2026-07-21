package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

func f64(v float64) *float64 { return &v }

func dummyProfit() domain.ProfitOverview {
	return domain.ProfitOverview{
		TotalAssetAmount: domain.DualCurrency{KRW: 1000000, USD: f64(700.5)},
		EarningAmount:    domain.DualCurrency{KRW: 50000, USD: f64(35)},
		Sales:            domain.ProfitByType{Amount: domain.DualCurrency{KRW: -1234, USD: f64(-0.9)}, EarningRate: domain.DualCurrency{KRW: -3.2}},
		Dividend:         domain.ProfitByType{Amount: domain.DualCurrency{KRW: 9999, USD: f64(7.1)}},
		Interest:         536,
	}
}

func TestWriteProfitOverviewJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteProfitOverview(&buf, FormatJSON, dummyProfit()); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(buf.String(), `"interest": 536`) {
		t.Errorf("missing interest in JSON: %s", buf.String())
	}
}

func TestWriteProfitOverviewTable(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteProfitOverview(&buf, FormatTable, dummyProfit()); err != nil {
		t.Fatalf("err: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"매매손익", "배당", "예탁금이자", "Total assets"} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q: %s", want, out)
		}
	}
}
