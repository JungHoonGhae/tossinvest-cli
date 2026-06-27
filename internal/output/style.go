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
// profitText/dimText/boldText are only called when colorEnabled() returned true,
// so forcing the render profile here is safe — the gate already decided.
// Profile value 4 = termenv.TrueColor (avoids importing termenv directly).
var colorRenderer = func() *lipgloss.Renderer {
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(4) // termenv.TrueColor
	return r
}()

var (
	styleUp   = colorRenderer.NewStyle().Foreground(lipgloss.Color("9"))  // 빨강 (KR: 상승/이익)
	styleDown = colorRenderer.NewStyle().Foreground(lipgloss.Color("12")) // 파랑 (KR: 하락/손실)
	styleDim  = colorRenderer.NewStyle().Faint(true)
	styleBold = colorRenderer.NewStyle().Bold(true)
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

func dimText(s string, enabled bool) string {
	if !enabled {
		return s
	}
	return styleDim.Render(s)
}

func boldText(s string, enabled bool) string {
	if !enabled {
		return s
	}
	return styleBold.Render(s)
}
