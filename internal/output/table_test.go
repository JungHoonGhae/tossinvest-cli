package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

func TestDisplayWidth(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"AAPL", 4},
		{"삼성전자", 8},
		{"TSLA 테슬라", 11}, // 4 + 1 + 6 = 11
		{"100,000 KRW", 11},
		{"", 0},
	}
	for _, tt := range tests {
		if got := displayWidth(tt.input); got != tt.want {
			t.Errorf("displayWidth(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestRenderTableAlign(t *testing.T) {
	headers := []string{"SYMBOL", "NAME", "QTY", "PRICE"}
	rows := [][]string{
		{"005930", "삼성전자", "10", "70,000"},
		{"AAPL", "Apple Inc.", "5", "150,000"},
	}
	aligns := []Align{AlignLeft, AlignLeft, AlignRight, AlignRight}

	var buf bytes.Buffer
	if err := renderTable(&buf, headers, rows, aligns...); err != nil {
		t.Fatalf("renderTable failed: %v", err)
	}

	out := buf.String()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 4 { // header + separator + 2 data rows
		t.Fatalf("expected 4 lines, got %d:\n%s", len(lines), out)
	}

	// Verify all rows have consistent visual display width
	hWidth := displayWidth(lines[0])
	sWidth := displayWidth(lines[1])
	r1Width := displayWidth(lines[2])
	r2Width := displayWidth(lines[3])

	if hWidth != sWidth || hWidth != r1Width || hWidth != r2Width {
		t.Errorf("Row display widths inconsistent: header=%d, sep=%d, row1=%d, row2=%d\n%s",
			hWidth, sWidth, r1Width, r2Width, out)
	}
}

func TestFormatHelpers(t *testing.T) {
	if got := formatKRW(1234567); got != "1,234,567" {
		t.Errorf("formatKRW(1234567) = %q, want %q", got, "1,234,567")
	}
	if got := formatKRW(-500); got != "-500" {
		t.Errorf("formatKRW(-500) = %q, want %q", got, "-500")
	}
	if got := formatUSD(123.45); got != "$123.45" {
		t.Errorf("formatUSD(123.45) = %q, want %q", got, "$123.45")
	}
	if got := formatUSD(-12.5); got != "-$12.50" {
		t.Errorf("formatUSD(-12.5) = %q, want %q", got, "-$12.50")
	}
	if got := formatPct(0.1234); got != "12.34%" {
		t.Errorf("formatPct(0.1234) = %q, want %q", got, "12.34%")
	}
	if got := formatQty(10); got != "10" {
		t.Errorf("formatQty(10) = %q, want %q", got, "10")
	}
	if got := formatQty(10.5); got != "10.5" {
		t.Errorf("formatQty(10.5) = %q, want %q", got, "10.5")
	}

	u := 10.5
	if got := usdOrDash(domain.DualCurrency{USD: &u}); got != "$10.50" {
		t.Errorf("usdOrDash with USD = %q, want %q", got, "$10.50")
	}
	if got := usdOrDash(domain.DualCurrency{USD: nil}); got != "-" {
		t.Errorf("usdOrDash with nil = %q, want %q", got, "-")
	}
}
