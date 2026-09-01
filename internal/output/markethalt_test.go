package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

func haltFixture(activated bool) domain.MarketHalt {
	return domain.MarketHalt{Events: []domain.MarketHaltEvent{
		{Market: "KSP", MarketName: "KOSPI", Type: "circuit_breaker", Activated: activated},
		{Market: "KSQ", MarketName: "KOSDAQ", Type: "sidecar", Activated: false},
	}}
}

// 평상시에도 스위치가 전부 보여야 한다. 발동된 것만 그리면 "정상인 날" 과
// "조회가 실패한 날" 이 화면상 똑같아진다.
func TestWriteMarketHaltListsEverySwitchWhenNormal(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteMarketHalt(&buf, FormatTable, haltFixture(false)); err != nil {
		t.Fatalf("WriteMarketHalt: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"KOSPI", "KOSDAQ"} {
		if !strings.Contains(out, want) {
			t.Errorf("normal-day table must still list %q:\n%s", want, out)
		}
	}
}

func TestWriteMarketHaltCSVCarriesActivated(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteMarketHalt(&buf, FormatCSV, haltFixture(true)); err != nil {
		t.Fatalf("WriteMarketHalt: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "KSP,KOSPI,circuit_breaker,true") {
		t.Errorf("CSV lost the activated flag:\n%s", out)
	}
}

// missing 은 표에서만 보이면 안 된다. JSON·CSV 로 읽는 쪽이 종목 누락을 모르면
// "데이터 없음" 이 조용한 데이터 손실이 된다.
func TestWriteChartsCarriesMissingInEveryFormat(t *testing.T) {
	batch := domain.ChartBatch{
		Charts:  []domain.Chart{{Symbol: "005930", Interval: "10m", Base: 100, Candles: []domain.Candle{{Close: 105}}}},
		Missing: []string{"999999"},
	}
	for _, f := range []Format{FormatJSON, FormatCSV, FormatTable} {
		var buf bytes.Buffer
		if err := WriteCharts(&buf, f, batch); err != nil {
			t.Fatalf("WriteCharts(%v): %v", f, err)
		}
		if !strings.Contains(buf.String(), "999999") {
			t.Errorf("format %v dropped the missing symbol:\n%s", f, buf.String())
		}
	}
}

func TestWriteStockReasonsCarriesMissingInEveryFormat(t *testing.T) {
	batch := domain.StockReasons{
		Reasons: []domain.StockReason{{Symbol: "005930", ProductCode: "A005930", Description: "사유"}},
		Missing: []string{"999999"},
	}
	for _, f := range []Format{FormatJSON, FormatCSV, FormatTable} {
		var buf bytes.Buffer
		if err := WriteStockReasons(&buf, f, batch); err != nil {
			t.Fatalf("WriteStockReasons(%v): %v", f, err)
		}
		if !strings.Contains(buf.String(), "999999") {
			t.Errorf("format %v dropped the missing symbol:\n%s", f, buf.String())
		}
	}
}

func TestWriteStockReasonsCarriesMissingWhenEveryReasonIsMissing(t *testing.T) {
	batch := domain.StockReasons{Missing: []string{"111111", "222222"}}
	for _, f := range []Format{FormatJSON, FormatCSV, FormatTable} {
		var buf bytes.Buffer
		if err := WriteStockReasons(&buf, f, batch); err != nil {
			t.Fatalf("WriteStockReasons(%v): %v", f, err)
		}
		for _, symbol := range batch.Missing {
			if !strings.Contains(buf.String(), symbol) {
				t.Errorf("format %v dropped all-missing symbol %q:\n%s", f, symbol, buf.String())
			}
		}
	}
}

func TestWriteStockReasonsCSVPreservesBatchSequence(t *testing.T) {
	batch := domain.StockReasons{
		Reasons: []domain.StockReason{
			{Symbol: "A", ProductCode: "A000001", Description: "first"},
			{Symbol: "C", ProductCode: "A000003", Description: "third"},
		},
		Missing: []string{"B"},
		Sequence: []domain.BatchSequenceEntry{
			{Symbol: "A"},
			{Symbol: "B", Missing: true},
			{Symbol: "C"},
		},
	}
	var buf bytes.Buffer
	if err := WriteStockReasons(&buf, FormatCSV, batch); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 4 || !strings.HasPrefix(lines[1], "A,") || !strings.HasPrefix(lines[2], "B,,") || !strings.HasPrefix(lines[3], "C,") {
		t.Fatalf("CSV lost request sequence: %v", lines)
	}
}

func TestWriteChartsCSVPreservesBatchSequence(t *testing.T) {
	batch := domain.ChartBatch{
		Charts: []domain.Chart{
			{Symbol: "A", ProductCode: "A000001", Candles: []domain.Candle{{Close: 1}}},
			{Symbol: "C", ProductCode: "A000003", Candles: []domain.Candle{{Close: 3}}},
		},
		Missing: []string{"B"},
		Sequence: []domain.BatchSequenceEntry{
			{Symbol: "A"},
			{Symbol: "B", Missing: true},
			{Symbol: "C"},
		},
	}
	var buf bytes.Buffer
	if err := WriteCharts(&buf, FormatCSV, batch); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 4 || !strings.HasPrefix(lines[1], "A,") || !strings.HasPrefix(lines[2], "B,,") || !strings.HasPrefix(lines[3], "C,") {
		t.Fatalf("chart CSV lost request sequence: %v", lines)
	}
}

func TestWriteChartsCSVPreservesFoundChartWithoutCandles(t *testing.T) {
	batch := domain.ChartBatch{
		Charts:   []domain.Chart{{Symbol: "A", ProductCode: "A000001", Interval: "10m"}},
		Sequence: []domain.BatchSequenceEntry{{Symbol: "A"}},
	}
	var buf bytes.Buffer
	if err := WriteCharts(&buf, FormatCSV, batch); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "A,A000001,10m") {
		t.Fatalf("found zero-candle chart disappeared from CSV: %q", buf.String())
	}
}
