package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

var testQuote = domain.Quote{
	ProductCode:    "A005930",
	Symbol:         "005930",
	Name:           "삼성전자",
	MarketCode:     "KSP",
	Market:         "코스피",
	Currency:       "KRW",
	ReferencePrice: 200500,
	Last:           199800,
	Change:         -700,
	ChangeRate:     -0.00349,
	Volume:         52301059,
	Status:         "N",
	FetchedAt:      time.Date(2026, 3, 21, 6, 0, 0, 0, time.UTC),
}

func TestWriteQuoteJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteQuote(&buf, FormatJSON, testQuote); err != nil {
		t.Fatalf("error: %v", err)
	}
	var parsed domain.Quote
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if parsed.Symbol != "005930" {
		t.Fatalf("expected 005930, got %s", parsed.Symbol)
	}
}

func TestWriteQuoteCSV(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteQuote(&buf, FormatCSV, testQuote); err != nil {
		t.Fatalf("error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if !strings.Contains(lines[1], "005930") {
		t.Fatalf("expected 005930 in CSV, got %s", lines[1])
	}
}

func TestWriteQuoteTable(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteQuote(&buf, FormatTable, testQuote); err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(buf.String(), "삼성전자") {
		t.Fatal("expected 삼성전자 in table output")
	}
}

func TestWriteQuotesJSON(t *testing.T) {
	quotes := []domain.Quote{testQuote, {Symbol: "TSLL", Name: "TSLL", Last: 12.12}}
	var buf bytes.Buffer
	if err := WriteQuotes(&buf, FormatJSON, quotes); err != nil {
		t.Fatalf("error: %v", err)
	}
	var parsed []domain.Quote
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(parsed) != 2 {
		t.Fatalf("expected 2 quotes, got %d", len(parsed))
	}
}

func TestWriteQuotesTable(t *testing.T) {
	quotes := []domain.Quote{testQuote, {Symbol: "TSLL", Name: "TSLL", Last: 12.12, Change: -0.85, ChangeRate: -0.065}}
	var buf bytes.Buffer
	if err := WriteQuotes(&buf, FormatTable, quotes); err != nil {
		t.Fatalf("error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "005930") || !strings.Contains(output, "TSLL") {
		t.Fatalf("expected both symbols in output, got %s", output)
	}
}

func TestWriteQuotesCSV(t *testing.T) {
	quotes := []domain.Quote{testQuote}
	var buf bytes.Buffer
	if err := WriteQuotes(&buf, FormatCSV, quotes); err != nil {
		t.Fatalf("error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
}

// Regression guards: buffer (non-TTY) must produce no ANSI codes.
func TestQuotePlainWhenNotTTY(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteQuote(&buf, FormatTable, testQuote); err != nil {
		t.Fatalf("WriteQuote error: %v", err)
	}
	if strings.Contains(buf.String(), "\x1b[") {
		t.Fatal("non-TTY WriteQuote table output must contain no ANSI escape sequences")
	}
}

func TestQuotesPlainWhenNotTTY(t *testing.T) {
	quotes := []domain.Quote{testQuote, {Symbol: "TSLL", Name: "TSLL", Last: 12.12, Change: -0.85, ChangeRate: -0.065}}
	var buf bytes.Buffer
	if err := WriteQuotes(&buf, FormatTable, quotes); err != nil {
		t.Fatalf("WriteQuotes error: %v", err)
	}
	if strings.Contains(buf.String(), "\x1b[") {
		t.Fatal("non-TTY WriteQuotes table output must contain no ANSI escape sequences")
	}
}
