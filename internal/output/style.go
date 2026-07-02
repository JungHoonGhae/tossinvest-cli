package output

import (
	"io"
	"os"

	"github.com/charmbracelet/lipgloss"
)

func isTerminalWriter(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func colorEnabled(w io.Writer, format Format) bool {
	return format == FormatTable && os.Getenv("NO_COLOR") == "" && isTerminalWriter(w)
}

// colorRenderer always outputs ANSI sequences regardless of TTY.
// profitText is only called when colorEnabled() returned true, so forcing the
// render profile here is safe — the gate already decided.
// Profile value 4 = termenv.TrueColor (avoids importing termenv directly).
var colorRenderer = func() *lipgloss.Renderer {
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(4) // termenv.TrueColor
	return r
}()

var (
	styleUp   = colorRenderer.NewStyle().Foreground(lipgloss.Color("9"))  // 빨강 (KR: 상승/이익)
	styleDown = colorRenderer.NewStyle().Foreground(lipgloss.Color("12")) // 파랑 (KR: 하락/손실)
)

func profitText(s string, value float64, enabled bool) string {
	if !enabled {
		return s
	}
	switch {
	case value > 0:
		return styleUp.Render(s)
	case value < 0:
		return styleDown.Render(s)
	default:
		return s
	}
}

// ColorEnabled reports whether ANSI styling should be applied for w under
// format — true only for table output on a real terminal with NO_COLOR
// unset. Exported so commands outside this package (e.g. `tossctl update`)
// can gate their own highlighted text using the same rule as the table
// renderers in this package, instead of duplicating the NO_COLOR/TTY check.
func ColorEnabled(w io.Writer, format Format) bool {
	return colorEnabled(w, format)
}

var (
	styleSuccess   = colorRenderer.NewStyle().Foreground(lipgloss.Color("10")) // 초록
	styleFail      = colorRenderer.NewStyle().Foreground(lipgloss.Color("9"))  // 빨강
	styleHighlight = colorRenderer.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
)

// StyleSuccess renders s in green when enabled, or returns s unchanged.
func StyleSuccess(s string, enabled bool) string {
	if !enabled {
		return s
	}
	return styleSuccess.Render(s)
}

// StyleFail renders s in red when enabled, or returns s unchanged.
func StyleFail(s string, enabled bool) string {
	if !enabled {
		return s
	}
	return styleFail.Render(s)
}

// StyleHighlight renders s in bold green when enabled, or returns s
// unchanged (used e.g. for the target version in an update confirmation).
func StyleHighlight(s string, enabled bool) string {
	if !enabled {
		return s
	}
	return styleHighlight.Render(s)
}
