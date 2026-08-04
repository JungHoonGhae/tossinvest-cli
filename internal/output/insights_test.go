package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// 더미 값 — 실계좌 데이터 아님.
func TestWriteStockReasoningOmitsEmptyType(t *testing.T) {
	r := domain.StockReasoning{
		Symbol: "000000", Title: "왜 올랐을까?", Summary: "더미 요약.",
		RelatedStock: []domain.RelatedStock{
			{Symbol: "111111", Name: "더미A", InvestmentTypeValue: "관심"},
			{Symbol: "222222", Name: "더미B"}, // 유형 없음
		},
	}
	var buf bytes.Buffer
	if err := WriteStockReasoning(&buf, FormatTable, r); err != nil {
		t.Fatalf("WriteStockReasoning: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "(관심)") {
		t.Errorf("type missing:\n%s", out)
	}
	// 빈 괄호가 남으면 데이터가 있는데 못 읽은 것처럼 보인다.
	if strings.Contains(out, "()") {
		t.Errorf("empty parens rendered:\n%s", out)
	}
}

func TestWriteStockReasoningEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteStockReasoning(&buf, FormatTable, domain.StockReasoning{}); err != nil {
		t.Fatalf("WriteStockReasoning: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("empty reasoning printed nothing")
	}
}

// 정상 계좌는 모든 날짜가 nil 이다. 0값을 찍으면 epoch 날짜가 나와
// 연체 계좌처럼 읽힌다.
func TestWriteMarginNoticeHidesNilDates(t *testing.T) {
	var buf bytes.Buffer
	n := domain.MarginNotice{Currency: "KRW", NoticeType: "NONE"}
	if err := WriteMarginNotice(&buf, FormatTable, n); err != nil {
		t.Fatalf("WriteMarginNotice: %v", err)
	}
	out := buf.String()
	for _, bad := range []string{"1970", "0001-01-01"} {
		if strings.Contains(out, bad) {
			t.Errorf("epoch date leaked (%s):\n%s", bad, out)
		}
	}
	if strings.Contains(out, "due") || strings.Contains(out, "기한") {
		t.Errorf("nil deadline rendered a line:\n%s", out)
	}
}

func TestWriteMarginNoticeShowsSetDates(t *testing.T) {
	due := "2026-08-06T00:00:00"
	var buf bytes.Buffer
	n := domain.MarginNotice{Currency: "KRW", NoticeType: "DEPOSIT", ReceivableAmount: 1000, DeadlineAt: &due}
	if err := WriteMarginNotice(&buf, FormatTable, n); err != nil {
		t.Fatalf("WriteMarginNotice: %v", err)
	}
	if !strings.Contains(buf.String(), due) {
		t.Errorf("deadline missing:\n%s", buf.String())
	}
}

func TestWriteStockSignalsEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteStockSignals(&buf, FormatTable, domain.StockSignals{Symbol: "000000"}); err != nil {
		t.Fatalf("WriteStockSignals: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("empty signals printed nothing")
	}
}

func TestWriteSearchResultsCSV(t *testing.T) {
	s := domain.SearchResults{Query: "더미", Results: []domain.SearchHit{
		{Keyword: "더미전자", ProductCode: "A000000", Symbol: "000000", Market: "KSP"},
	}}
	var buf bytes.Buffer
	if err := WriteSearchResults(&buf, FormatCSV, s); err != nil {
		t.Fatalf("WriteSearchResults: %v", err)
	}
	if !strings.Contains(buf.String(), "A000000") {
		t.Errorf("CSV missing product code:\n%s", buf.String())
	}
}
