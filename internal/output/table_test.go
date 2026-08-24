package output

import (
	"bytes"
	"github.com/charmbracelet/lipgloss"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

func TestLipglossWidthSmoke(t *testing.T) {
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
		if got := lipgloss.Width(tt.input); got != tt.want {
			t.Errorf("lipgloss.Width(%q) = %d, want %d", tt.input, got, tt.want)
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
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 4 { // header + separator + 2 data rows
		t.Fatalf("expected 4 lines, got %d:\n%s", len(lines), out)
	}

	// Verify all lines have equal display width (proper column alignment)
	headerWidth := lipgloss.Width(lines[0])
	for i, line := range lines[1:] {
		if w := lipgloss.Width(line); w != headerWidth {
			t.Errorf("line %d width=%d, want %d (header width):\n%s", i+1, w, headerWidth, out)
		}
	}

	// Verify row 0 (the first data row) correctly right-aligns numeric columns
	// instead of being erroneously forced to left-align.
	if !strings.Contains(lines[2], "  10 ") || !strings.Contains(lines[2], " 70,000") {
		t.Errorf("row 0 (line 2) failed to right-align numeric values:\n%s", out)
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
