package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

var testWatchlistItems = []domain.WatchlistItem{
	{Group: "관심", Symbol: "005930", Name: "삼성전자", Currency: "KRW", Base: 70000, Last: 72000},
	{Group: "관심", Symbol: "AAPL", Name: "Apple", Currency: "USD", Base: 150.0, Last: 145.0},
}

func TestWriteWatchlistTable(t *testing.T) {
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
