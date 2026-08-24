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
