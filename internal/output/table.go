package output

import (
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// Align defines the text alignment for a table column.
type Align int

const (
	AlignLeft Align = iota
	AlignRight
)

// renderTable writes a formatted table.
// Cells may contain ANSI escape sequences; lipgloss handles them properly.
//
// Alignment defaults: column 0 = AlignLeft, columns 1..N = AlignRight.
// When aligns has fewer entries than columns, trailing columns fall back to
// these defaults.
func renderTable(w io.Writer, headers []string, rows [][]string, aligns ...Align) error {
	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderRow(false).
		BorderColumn(false).
		BorderTop(false).
		BorderBottom(false).
		BorderLeft(false).
		BorderRight(false).
		BorderHeader(true).
		Headers(headers...).
		Rows(rows...)

	t.StyleFunc(func(row, col int) lipgloss.Style {
		s := lipgloss.NewStyle().PaddingRight(2)

		align := AlignRight
		if col == 0 {
			align = AlignLeft
		}
		if col < len(aligns) {
			align = aligns[col]
		}

		if align == AlignRight {
			s = s.Align(lipgloss.Right)
		} else {
			s = s.Align(lipgloss.Left)
		}
		return s
	})

	_, err := fmt.Fprintln(w, t)
	return err
}

func formatKRW(v float64) string {
	neg := v < 0
	v = math.Abs(v)
	whole := int64(v)
	frac := v - float64(whole)

	s := formatWithCommas(whole)
	if frac > 0.0001 {
		s += strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", frac)[1:], "0"), ".")
	}
	if neg {
		return "-" + s
	}
	return s
}

func formatUSD(v float64) string {
	neg := v < 0
	v = math.Abs(v)
	s := fmt.Sprintf("$%.2f", v)
	if neg {
		return "-" + s
	}
	return s
}

func formatPct(v float64) string {
	return fmt.Sprintf("%.2f%%", v*100)
}

func formatQty(v float64) string {
	if v == math.Trunc(v) {
		return fmt.Sprintf("%.0f", v)
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.6f", v), "0"), ".")
}

func formatWithCommas(n int64) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}

func truncateName(name string, maxRunes int) string {
	runes := []rune(name)
	if len(runes) <= maxRunes {
		return name
	}
	return string(runes[:maxRunes-1]) + "…"
}

func usdOrDash(d domain.DualCurrency) string {
	if d.USD == nil {
		return "-"
	}
	return formatUSD(*d.USD)
}
