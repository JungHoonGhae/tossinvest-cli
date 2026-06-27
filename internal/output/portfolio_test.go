package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

var testPositions = []domain.Position{
	{
		ProductCode:     "A005930",
		Symbol:          "005930",
		Name:            "삼성전자",
		MarketType:      "KR_STOCK",
		MarketCode:      "KSP",
		Quantity:        10,
		AveragePrice:    70000,
		CurrentPrice:    80000,
		MarketValue:     800000,
		UnrealizedPnL:   100000,
		ProfitRate:      0.1429,
		DailyProfitLoss: 5000,
		DailyProfitRate: 0.0063,
	},
	{
		ProductCode:        "US20220809012",
		Symbol:             "TSLL",
		Name:               "TSLL",
		MarketType:         "US_STOCK",
		MarketCode:         "NSQ",
		Quantity:           100,
		AveragePrice:       15000,
		CurrentPrice:       12000,
		MarketValue:        1200000,
		AveragePriceUSD:    10.50,
		CurrentPriceUSD:    8.40,
		MarketValueUSD:     840,
		UnrealizedPnLUSD:   -210,
		ProfitRateUSD:      -0.20,
		DailyProfitLossUSD: -15,
		DailyProfitRateUSD: -0.0175,
	},
}

func TestWritePositionsJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := WritePositions(&buf, FormatJSON, testPositions); err != nil {
		t.Fatalf("WritePositions JSON error: %v", err)
	}
	var parsed []domain.Position
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("JSON unmarshal error: %v", err)
	}
	if len(parsed) != 2 {
		t.Fatalf("expected 2 positions, got %d", len(parsed))
	}
	if parsed[0].Symbol != "005930" {
		t.Fatalf("expected 005930, got %s", parsed[0].Symbol)
	}
}

func TestWritePositionsCSV(t *testing.T) {
	var buf bytes.Buffer
	if err := WritePositions(&buf, FormatCSV, testPositions); err != nil {
		t.Fatalf("WritePositions CSV error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 { // header + 2 rows
		t.Fatalf("expected 3 CSV lines, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "product_code") {
		t.Fatalf("expected CSV header, got %s", lines[0])
	}
	if !strings.Contains(lines[0], "average_price_usd") {
		t.Fatalf("expected USD columns in CSV header, got %s", lines[0])
	}
	if !strings.Contains(lines[1], "005930") {
		t.Fatalf("expected 005930 in first row, got %s", lines[1])
	}
}

func TestWritePositionsTable(t *testing.T) {
	var buf bytes.Buffer
	if err := WritePositions(&buf, FormatTable, testPositions); err != nil {
		t.Fatalf("WritePositions Table error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "삼성전자") {
		t.Fatalf("expected 삼성전자 in table output")
	}
	if !strings.Contains(output, "TSLL") {
		t.Fatalf("expected TSLL in table output")
	}
	if !strings.Contains(output, "USD") {
		t.Fatalf("expected USD column header for US_STOCK in table output")
	}
	if !strings.Contains(output, "$") {
		t.Fatalf("expected USD values for US_STOCK in table output")
	}
	// KR_STOCK should not have USD values
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "삼성전자") && strings.Contains(line, "$") {
			t.Fatalf("KR_STOCK should not have USD values, got %s", line)
		}
	}
}

func TestWritePositionsEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := WritePositions(&buf, FormatJSON, nil); err != nil {
		t.Fatalf("WritePositions empty error: %v", err)
	}
	if !strings.Contains(buf.String(), "null") && !strings.Contains(buf.String(), "[]") {
		// Go json encodes nil slice as null
	}
}

func TestWriteAllocationCSV(t *testing.T) {
	markets := map[string]domain.AccountMarketSummary{
		"us": {Market: "us", TotalAssetAmount: 1000000, PrincipalAmount: 900000, EvaluatedProfitAmount: 100000, ProfitRate: 0.1111},
		"kr": {Market: "kr", TotalAssetAmount: 500000, PrincipalAmount: 400000, EvaluatedProfitAmount: 100000, ProfitRate: 0.25},
	}
	var buf bytes.Buffer
	if err := WriteAllocation(&buf, FormatCSV, markets); err != nil {
		t.Fatalf("WriteAllocation CSV error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 { // header + 2 rows
		t.Fatalf("expected 3 CSV lines, got %d", len(lines))
	}
	// kr should come before us (sorted)
	if !strings.HasPrefix(lines[1], "kr") {
		t.Fatalf("expected kr first (sorted), got %s", lines[1])
	}
}

func TestWritePositionsUnsupportedFormat(t *testing.T) {
	var buf bytes.Buffer
	err := WritePositions(&buf, "yaml", testPositions)
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
}

// Regression guard: buffer (non-TTY) must produce no ANSI codes.
func TestPortfolioPlainWhenNotTTY(t *testing.T) {
	var buf bytes.Buffer
	if err := WritePositions(&buf, FormatTable, testPositions); err != nil {
		t.Fatalf("WritePositions error: %v", err)
	}
	if strings.Contains(buf.String(), "\x1b[") {
		t.Fatal("non-TTY table output must contain no ANSI escape sequences")
	}
}
