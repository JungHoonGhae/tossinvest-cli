package output

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"strings"
	"testing"
)

func TestTranscriptOutputPreservesNullsAndMultilineText(t *testing.T) {
	data := domain.EarningTranscript{EventID: 42, Available: true, Paragraphs: []domain.TranscriptParagraph{{ID: "p", OriginalText: "a,b\nnext", StartSeconds: 1.25, EndSeconds: 2.5}}}
	var b bytes.Buffer
	if err := WriteEarningTranscript(&b, FormatJSON, data); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	row := got["paragraphs"].([]any)[0].(map[string]any)
	if row["translation"] != nil || row["summary"] != nil || row["start_seconds"] != 1.25 {
		t.Fatal("lost publication state/time")
	}
	b.Reset()
	if err := WriteEarningTranscript(&b, FormatCSV, data); err != nil {
		t.Fatal(err)
	}
	records, err := csv.NewReader(&b).ReadAll()
	if err != nil || records[1][4] != "a,b\nnext" {
		t.Fatalf("CSV %v %v", records, err)
	}
	b.Reset()
	if err := WriteEarningTranscript(&b, FormatTable, domain.EarningTranscript{}); err != nil || !strings.Contains(b.String(), i18n.T("output.transcript.empty")) {
		t.Fatalf("empty transcript %q %v", b.String(), err)
	}
}
func TestReportOutputPreservesUnavailableSectionsAndSourceIDs(t *testing.T) {
	data := domain.EarningReport{EventID: 42, WatchPoints: &domain.EarningWatchPoints{Sources: []domain.EarningSource{{ID: "source-1", Title: "Source", Provider: "Agency"}}}}
	for _, format := range []Format{FormatJSON, FormatCSV, FormatTable} {
		var b bytes.Buffer
		if err := WriteEarningReport(&b, format, data); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(b.String(), "source-1") {
			t.Fatal("missing source identity")
		}
		if format == FormatJSON && !strings.Contains(b.String(), `"analysis": null`) {
			t.Fatal("missing unavailable analysis")
		}
	}
}
