package output

import (
	"fmt"
	"io"
	"math"
	"strings"
	"unicode/utf8"
)

// renderTable writes a formatted table with the first column left-aligned
// and all other columns right-aligned.
func renderTable(w io.Writer, headers []string, rows [][]string) error {
	return renderTableColored(w, headers, rows, nil)
}

// renderTableColored is like renderTable but accepts a separate displayRows slice
// whose cells may contain ANSI escape sequences.  plainRows is always used for
// column-width computation (so ANSI codes never distort padding), while
// displayRows is used for the actual cell content that is written to w.
// When displayRows is nil the function behaves identically to renderTable.
func renderTableColored(w io.Writer, headers []string, plainRows [][]string, displayRows [][]string) error {
	if displayRows == nil {
		displayRows = plainRows
	}
	colWidths := make([]int, len(headers))
	for i, h := range headers {
		colWidths[i] = displayWidth(h)
	}
	for _, row := range plainRows {
		for i, cell := range row {
			if i < len(colWidths) {
				if dw := displayWidth(cell); dw > colWidths[i] {
					colWidths[i] = dw
				}
			}
		}
	}

	writeRow := func(cells []string, plainCells []string) {
		for i, cell := range cells {
			if i >= len(colWidths) {
				break
			}
			if i > 0 {
				fmt.Fprint(w, "  ")
			}
			plain := cell
			if i < len(plainCells) {
				plain = plainCells[i]
			}
			pad := colWidths[i] - displayWidth(plain)
			if pad < 0 {
				pad = 0
			}
			if i == 0 {
				fmt.Fprint(w, cell+strings.Repeat(" ", pad))
			} else {
				fmt.Fprint(w, strings.Repeat(" ", pad)+cell)
			}
		}
		fmt.Fprint(w, "\n")
	}

	writeRow(headers, headers)

	sep := make([]string, len(headers))
	for i, cw := range colWidths {
		sep[i] = strings.Repeat("─", cw)
	}
	writeRow(sep, sep)

	for i, row := range displayRows {
		var plain []string
		if i < len(plainRows) {
			plain = plainRows[i]
		}
		writeRow(row, plain)
	}

	return nil
}

func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		if r >= 0x1100 && isWide(r) {
			w += 2
		} else {
			w++
		}
	}
	_ = utf8.RuneCountInString
	return w
}

func isWide(r rune) bool {
	return (r >= 0x1100 && r <= 0x115F) ||
		(r >= 0x2E80 && r <= 0x303E) ||
		(r >= 0x3040 && r <= 0x33BF) ||
		(r >= 0x3400 && r <= 0x4DBF) ||
		(r >= 0x4E00 && r <= 0xA4CF) ||
		(r >= 0xA960 && r <= 0xA97C) ||
		(r >= 0xAC00 && r <= 0xD7A3) ||
		(r >= 0xF900 && r <= 0xFAFF) ||
		(r >= 0xFE30 && r <= 0xFE6F) ||
		(r >= 0xFF01 && r <= 0xFF60) ||
		(r >= 0xFFE0 && r <= 0xFFE6) ||
		(r >= 0x20000 && r <= 0x2FFFD) ||
		(r >= 0x30000 && r <= 0x3FFFD)
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
