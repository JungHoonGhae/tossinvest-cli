package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/config"
)

func TestWriteConfigStatusJSON(t *testing.T) {
	status := config.Status{
		ConfigFile:    "/tmp/config.json",
		Exists:        true,
		SchemaVersion: 2,
		Trading: config.Trading{
			Place: true,
			Sell:  true,
		},
	}
	var buf bytes.Buffer
	if err := WriteConfigStatus(&buf, FormatJSON, status); err != nil {
		t.Fatalf("WriteConfigStatus JSON error: %v", err)
	}
	if !strings.Contains(buf.String(), `"place":true`) && !strings.Contains(buf.String(), `"place": true`) {
		t.Fatalf("expected place:true in JSON output, got %s", buf.String())
	}
}

func TestWriteConfigStatusTable(t *testing.T) {
	status := config.Status{
		ConfigFile:    "/tmp/config.json",
		Exists:        true,
		SchemaVersion: 2,
		Trading: config.Trading{
			Place: true,
			Sell:  false,
		},
	}
	var buf bytes.Buffer
	if err := WriteConfigStatus(&buf, FormatTable, status); err != nil {
		t.Fatalf("WriteConfigStatus Table error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "Trading Place: true") {
		t.Fatalf("expected Trading Place: true in table output, got %s", output)
	}
	if !strings.Contains(output, "Trading Sell: false") {
		t.Fatalf("expected Trading Sell: false, got %s", output)
	}
	if !strings.Contains(output, "Experimental Paper Trading: false") {
		t.Fatalf("expected paper experiment state, got %s", output)
	}
}

func TestWriteConfigStatusCSV(t *testing.T) {
	status := config.Status{
		ConfigFile:    "/tmp/config.json",
		Exists:        true,
		SchemaVersion: 2,
		Trading:       config.Trading{},
	}
	var buf bytes.Buffer
	if err := WriteConfigStatus(&buf, FormatCSV, status); err != nil {
		t.Fatalf("WriteConfigStatus CSV error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 CSV lines, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "allow_live_order_actions") {
		t.Fatalf("expected allow_live_order_actions column in CSV header, got %s", lines[0])
	}
	if !strings.Contains(lines[0], "experimental_paper_trading") {
		t.Fatalf("expected experimental_paper_trading column in CSV header, got %s", lines[0])
	}
}
