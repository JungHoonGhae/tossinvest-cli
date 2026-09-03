package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
)

var testWatchlistItems = []domain.WatchlistItem{
	{Group: "관심", ProductCode: "A005930", Symbol: "005930", Name: "삼성전자", Currency: "KRW", Base: 70000, Last: 72000},
	{Group: "관심", Symbol: "AAPL", Name: "Apple", Currency: "USD", Base: 150.0, Last: 145.0},
}

func TestWriteWatchlistCSVPreservesReleasedPrefix(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := WriteWatchlist(&buf, FormatCSV, testWatchlistItems[:1]); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if lines[0] != "group,symbol,name,currency,base,last,product_code" {
		t.Fatalf("CSV header reordered released columns: %q", lines[0])
	}
	if lines[1] != "관심,005930,삼성전자,KRW,70000,72000,A005930" {
		t.Fatalf("CSV row does not match header: %q", lines[1])
	}
}

func TestWriteWatchlistTable(t *testing.T) {
	prev := i18n.Lang()
	i18n.SetLang("ko")
	defer i18n.SetLang(prev)

	var buf bytes.Buffer
	if err := WriteWatchlist(&buf, FormatTable, testWatchlistItems); err != nil {
		t.Fatalf("WriteWatchlist error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "삼성전자") {
		t.Fatal("expected 삼성전자 in watchlist table")
	}
	if !strings.Contains(output, "AAPL") {
		t.Fatal("expected AAPL in watchlist table")
	}
	if !strings.Contains(output, "등락") {
		t.Fatal("expected 등락 column in watchlist table")
	}
}

// Regression guard: buffer (non-TTY) must produce no ANSI codes.
func TestWatchlistPlainWhenNotTTY(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteWatchlist(&buf, FormatTable, testWatchlistItems); err != nil {
		t.Fatalf("WriteWatchlist error: %v", err)
	}
	if strings.Contains(buf.String(), "\x1b[") {
		t.Fatal("non-TTY WriteWatchlist table output must contain no ANSI escape sequences")
	}
}

var testWatchlistGroups = []domain.WatchlistGroup{
	{ID: 1, Name: "미국주식", Type: "USER_MADE", ItemCount: 5},
	{ID: 2, Name: "국내주식", Type: "USER_MADE", ItemCount: 10},
}

func TestWriteWatchlistGroupsTable(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteWatchlistGroups(&buf, FormatTable, testWatchlistGroups); err != nil {
		t.Fatalf("WriteWatchlistGroups table error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "미국주식") || !strings.Contains(output, "국내주식") {
		t.Fatalf("expected folder names in table output, got %q", output)
	}
}

func TestWriteWatchlistGroupsJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteWatchlistGroups(&buf, FormatJSON, testWatchlistGroups); err != nil {
		t.Fatalf("WriteWatchlistGroups JSON error: %v", err)
	}
	if !strings.Contains(buf.String(), `"name": "미국주식"`) {
		t.Fatalf("expected JSON output with folder name, got %q", buf.String())
	}
}

func TestWriteWatchlistGroupsCSV(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteWatchlistGroups(&buf, FormatCSV, testWatchlistGroups); err != nil {
		t.Fatalf("WriteWatchlistGroups CSV error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "id,name,type,item_count") {
		t.Fatalf("expected CSV header, got %q", output)
	}
	if !strings.Contains(output, "1,미국주식,USER_MADE,5") {
		t.Fatalf("expected CSV row, got %q", output)
	}
}
