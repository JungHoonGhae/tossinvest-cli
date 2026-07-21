package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

func dummyAccumulationPlans() domain.AccumulationPlans {
	return domain.AccumulationPlans{
		Plans: []domain.AccumulationPlan{
			{
				Symbol: "DUMMY", StockName: "Dummy Corp", Currency: "KRW",
				PlanType: "AMOUNT", Iteration: "DAILY",
				InvestAmount: 10000, IsPaused: false, SucceededRound: 3,
				InvestStartDate: "2026-01-01",
			},
			{
				Symbol: "PAWS", StockName: "Paused Inc", Currency: "USD",
				PlanType: "QUANTITY", Iteration: "WEEKLY",
				InvestQuantity: 1, IsPaused: true, SucceededRound: 0,
				InvestStartDate: "2026-02-01",
			},
		},
	}
}

func TestWriteAccumulationPlansJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteAccumulationPlans(&buf, FormatJSON, dummyAccumulationPlans()); err != nil {
		t.Fatalf("WriteAccumulationPlans JSON error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"symbol": "DUMMY"`) {
		t.Errorf("missing DUMMY symbol in JSON: %s", out)
	}
	if !strings.Contains(out, `"is_paused": true`) {
		t.Errorf("missing is_paused:true in JSON: %s", out)
	}
}

func TestWriteAccumulationPlansTableShowsStatus(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteAccumulationPlans(&buf, FormatTable, dummyAccumulationPlans()); err != nil {
		t.Fatalf("WriteAccumulationPlans table error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Active") {
		t.Errorf("expected Active status in table: %s", out)
	}
	if !strings.Contains(out, "Paused") {
		t.Errorf("expected Paused status in table: %s", out)
	}
}

func TestWriteAccumulationPlansTableEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteAccumulationPlans(&buf, FormatTable, domain.AccumulationPlans{}); err != nil {
		t.Fatalf("WriteAccumulationPlans empty error: %v", err)
	}
	if !strings.Contains(buf.String(), "no accumulation plans") {
		t.Errorf("expected empty-state message, got %q", buf.String())
	}
}

func TestWriteAccumulationPlansCSV(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteAccumulationPlans(&buf, FormatCSV, dummyAccumulationPlans()); err != nil {
		t.Fatalf("WriteAccumulationPlans CSV error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 { // header + 2 rows
		t.Fatalf("want 3 CSV lines, got %d: %q", len(lines), buf.String())
	}
	if !strings.HasPrefix(lines[0], "symbol,stock_name,status") {
		t.Errorf("unexpected CSV header: %q", lines[0])
	}
}
