package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// 서버가 float32 정밀도를 그대로 흘린다. 표에서 그 자릿수를 다 찍으면 범위를 눈으로
// 비교할 수 없다.
func TestWriteScreenerFilterRangesTrimsPrecision(t *testing.T) {
	r := domain.ScreenerFilterRanges{Nation: "kr", Filters: []domain.ScreenerFilterRange{
		{FilterID: "PER", Min: f64(-8021.2705078125), Max: f64(6973.802734375), BasedAt: "2026-01-02"},
	}}
	var buf bytes.Buffer
	if err := WriteScreenerFilterRanges(&buf, FormatTable, r); err != nil {
		t.Fatalf("WriteScreenerFilterRanges: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "8021.2705078125") {
		t.Errorf("raw float spill in table:\n%s", out)
	}
	if !strings.Contains(out, "-8021.27") {
		t.Errorf("value lost:\n%s", out)
	}
}

// 기준일이 없으면 꼬리에 "기준 " 만 남아 값을 못 읽은 것처럼 보인다.
func TestWriteScreenerFilterRangesNoDate(t *testing.T) {
	r := domain.ScreenerFilterRanges{Nation: "kr", Filters: []domain.ScreenerFilterRange{
		{FilterID: "배당_수익률", Min: f64(0), Max: f64(0.382)},
	}}
	var buf bytes.Buffer
	if err := WriteScreenerFilterRanges(&buf, FormatTable, r); err != nil {
		t.Fatalf("WriteScreenerFilterRanges: %v", err)
	}
	out := strings.TrimRight(buf.String(), "\n")
	if strings.HasSuffix(out, "기준") || strings.HasSuffix(out, "as of") {
		t.Errorf("dangling date label:\n%q", out)
	}
}

// 범위를 못 받은 필터는 0~0 이 아니라 사유를 보여야 한다.
func TestWriteScreenerFilterRangesUnavailable(t *testing.T) {
	r := domain.ScreenerFilterRanges{Nation: "kr", Filters: []domain.ScreenerFilterRange{
		{FilterID: "주가등락률", Unavailable: "screener.invalid.filter-condition-period"},
	}}
	var buf bytes.Buffer
	if err := WriteScreenerFilterRanges(&buf, FormatTable, r); err != nil {
		t.Fatalf("WriteScreenerFilterRanges: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "filter-condition-period") {
		t.Errorf("server reason dropped:\n%s", out)
	}
	if strings.Contains(out, "0 ~ 0") {
		t.Errorf("empty range rendered as zeros:\n%s", out)
	}
}
