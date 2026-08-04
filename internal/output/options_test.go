package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// 더미 값 — 실시세 아님.
func TestWriteOptionExpiriesOmitsEmptyNote(t *testing.T) {
	e := domain.OptionExpiries{Symbol: "DUMMY", Expiries: []domain.OptionExpiry{
		{MaturityDate: "2026-01-02", DisplayLiquidation: "거래 종료"},
		{MaturityDate: "2026-01-09"},
	}}
	var buf bytes.Buffer
	if err := WriteOptionExpiries(&buf, FormatTable, e); err != nil {
		t.Fatalf("WriteOptionExpiries: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "거래 종료") {
		t.Errorf("display note missing:\n%s", out)
	}
	// 주석 없는 만기는 날짜만 — 꼬리 공백이 남지 않아야 한다.
	if strings.Contains(out, "2026-01-09  \n") {
		t.Errorf("trailing note separator left:\n%q", out)
	}
}

func TestWriteOptionExpiriesEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteOptionExpiries(&buf, FormatTable, domain.OptionExpiries{Symbol: "DUMMY"}); err != nil {
		t.Fatalf("WriteOptionExpiries: %v", err)
	}
	// 옵션 미지원 종목이라는 힌트를 줘야 한다 — 빈 출력은 버그처럼 보인다.
	if buf.Len() == 0 {
		t.Error("empty expiries printed nothing")
	}
}

func TestWriteOptionChainKeepsServerOrder(t *testing.T) {
	c := domain.OptionChain{Symbol: "DUMMY", MaturityDate: "2026-01-09", Rows: []domain.OptionChainRow{
		{StrikePrice: 100, CallOpenInterest: 5},
		{StrikePrice: 105, PutOpenInterest: 7},
	}}
	var buf bytes.Buffer
	if err := WriteOptionChain(&buf, FormatTable, c); err != nil {
		t.Fatalf("WriteOptionChain: %v", err)
	}
	out := buf.String()
	if strings.Index(out, "100") > strings.Index(out, "105") {
		t.Errorf("strike order changed:\n%s", out)
	}
}

func TestWriteOptionChainCSV(t *testing.T) {
	c := domain.OptionChain{Rows: []domain.OptionChainRow{{StrikePrice: 100, CallGUID: "OPT_C1", CallOpenInterest: 5}}}
	var buf bytes.Buffer
	if err := WriteOptionChain(&buf, FormatCSV, c); err != nil {
		t.Fatalf("WriteOptionChain: %v", err)
	}
	if !strings.Contains(buf.String(), "OPT_C1") {
		t.Errorf("CSV missing call guid:\n%s", buf.String())
	}
}
