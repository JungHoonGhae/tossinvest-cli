package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

func TestWriteAISignalDetailRendersReasoningAndRelatedFlow(t *testing.T) {
	t.Parallel()
	detail := domain.AISignalDetail{
		Found: true, ProductCode: "A005930", ProductType: "STOCKS", SignalID: "signal-1",
		Description: "핵심 설명", Keywords: []string{"반도체", "수급"},
		Issue:          domain.AISignalIssue{AssetName: "예시 종목", AssetCode: "A005930", Description: []string{"첫 문장"}, ProfitLossRate: 3.25},
		News:           []domain.BriefingNews{{ID: "news-1", Title: "관련 뉴스", Agency: "예시 통신사"}},
		RelatedCallout: "연관 흐름",
		Related: []domain.AISignalRelatedReasoning{{
			SignalID: "related-1", AssetCode: "A000660", AssetName: "연관 종목",
			Description:  []string{"연관 설명"},
			Relationship: domain.AISignalRelationship{SubjectName: "예시 종목", Relation: "공급", ObjectName: "연관 종목"},
		}},
	}

	var table bytes.Buffer
	if err := WriteAISignalDetail(&table, FormatTable, detail); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"예시 종목", "핵심 설명", "첫 문장", "관련 뉴스", "연관 흐름", "공급"} {
		if !strings.Contains(table.String(), want) {
			t.Fatalf("table missing %q:\n%s", want, table.String())
		}
	}

	var csv bytes.Buffer
	if err := WriteAISignalDetail(&csv, FormatCSV, detail); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"issue", "news", "related", "signal-1", "related-1"} {
		if !strings.Contains(csv.String(), want) {
			t.Fatalf("csv missing %q:\n%s", want, csv.String())
		}
	}
}

func TestWriteAISignalDetailExplainsNoCurrentSignal(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	err := WriteAISignalDetail(&out, FormatTable, domain.AISignalDetail{
		ProductCode: "A005930", ProductType: "STOCKS", Found: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "A005930") {
		t.Fatalf("empty-state output must identify the product: %q", out.String())
	}
}
