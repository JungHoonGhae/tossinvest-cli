package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/youcom"
)

func testResearchResult() youcom.ResearchResult {
	return youcom.ResearchResult{
		Output: youcom.Output{
			Content: "삼성전자는 최근 반도체 업황 회복에 힘입어...",
			Sources: []youcom.Source{
				{URL: "https://example.com/a", Title: "Samsung Q2 earnings", Snippets: []string{"snippet one"}},
				{URL: "https://example.com/b", Title: "", Snippets: nil},
			},
		},
	}
}

func TestWriteYouComResearchTable(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteYouComResearch(&buf, FormatTable, testResearchResult()); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "삼성전자는 최근 반도체 업황 회복에 힘입어") {
		t.Errorf("expected content in output: %q", out)
	}
	if !strings.Contains(out, "1. Samsung Q2 earnings — https://example.com/a") {
		t.Errorf("expected numbered source with title and url: %q", out)
	}
	// Source with no title falls back to showing the URL as the title too.
	if !strings.Contains(out, "2. https://example.com/b — https://example.com/b") {
		t.Errorf("expected fallback-to-url title for source without title: %q", out)
	}
}

func TestWriteYouComResearchTableEmpty(t *testing.T) {
	prev := i18n.Lang()
	i18n.SetLang("en")
	defer i18n.SetLang(prev)

	var buf bytes.Buffer
	if err := WriteYouComResearch(&buf, FormatTable, youcom.ResearchResult{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "No research content returned") {
		t.Errorf("expected empty notice: %q", buf.String())
	}
}

func TestWriteYouComResearchJSON(t *testing.T) {
	var buf bytes.Buffer
	res := testResearchResult()
	if err := WriteYouComResearch(&buf, FormatJSON, res); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `"content": "삼성전자는 최근 반도체 업황 회복에 힘입어..."`) {
		t.Errorf("unexpected JSON content: %q", out)
	}
	if !strings.Contains(out, `"url": "https://example.com/a"`) {
		t.Errorf("unexpected JSON sources: %q", out)
	}
}

func TestWriteYouComResearchJSONLanguageInvariant(t *testing.T) {
	res := testResearchResult()

	var enBuf, koBuf bytes.Buffer
	i18n.SetLang("en")
	if err := WriteYouComResearch(&enBuf, FormatJSON, res); err != nil {
		t.Fatal(err)
	}
	i18n.SetLang("ko")
	if err := WriteYouComResearch(&koBuf, FormatJSON, res); err != nil {
		t.Fatal(err)
	}
	if enBuf.String() != koBuf.String() {
		t.Errorf("JSON output must not vary by locale: en=%q ko=%q", enBuf.String(), koBuf.String())
	}
}

func TestWriteYouComResearchCSV(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteYouComResearch(&buf, FormatCSV, testResearchResult()); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.HasPrefix(out, "type,index,title,url,text\n") {
		t.Errorf("unexpected CSV header: %q", out)
	}
	if !strings.Contains(out, "content,,,,") {
		t.Errorf("expected a content row: %q", out)
	}
	if !strings.Contains(out, "source,1,Samsung Q2 earnings,https://example.com/a,snippet one") {
		t.Errorf("expected a source row: %q", out)
	}
}

func TestWriteYouComResearchUnsupportedFormat(t *testing.T) {
	var buf bytes.Buffer
	err := WriteYouComResearch(&buf, Format("xml"), testResearchResult())
	if err == nil {
		t.Fatal("expected an error for an unsupported format")
	}
}
