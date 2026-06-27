package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"strings"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// ANSI color codes for chart rendering. Populated only when stdout is a TTY
// and NO_COLOR is unset — non-tty pipes (cron, AI-agent capture) get plain
// text so structured output and agent context stay clean.
var (
	chartColorReset string
	chartColorRed   string
	chartColorBlue  string
	chartColorBold  string
	chartColorDim   string
)

func init() {
	if os.Getenv("NO_COLOR") != "" || !isTerminalWriter(os.Stdout) {
		return
	}
	chartColorReset = "\033[0m"
	chartColorRed = "\033[31m"
	chartColorBlue = "\033[34m"
	chartColorBold = "\033[1m"
	chartColorDim = "\033[2m"
}

func WriteChart(w io.Writer, format Format, chart domain.Chart) error {
	switch format {
	case FormatTable:
		return writeChartTable(w, chart)
	case FormatJSON:
		return writeChartJSON(w, chart)
	case FormatCSV:
		return writeChartCSV(w, chart)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func writeChartJSON(w io.Writer, chart domain.Chart) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(chart)
}

func writeChartCSV(w io.Writer, chart domain.Chart) error {
	writer := csv.NewWriter(w)
	if err := writer.Write([]string{"time", "open", "high", "low", "close", "volume"}); err != nil {
		return err
	}
	for _, c := range chart.Candles {
		if err := writer.Write([]string{
			c.Time.Format("2006-01-02T15:04:05Z07:00"),
			formatFloat(c.Open),
			formatFloat(c.High),
			formatFloat(c.Low),
			formatFloat(c.Close),
			formatFloat(c.Volume),
		}); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func writeChartTable(w io.Writer, chart domain.Chart) error {
	if len(chart.Candles) == 0 {
		_, err := fmt.Fprintln(w, "no candle data")
		return err
	}

	last := chart.Candles[len(chart.Candles)-1].Close
	base := chart.Base
	change := last - base
	var changeRate float64
	if base != 0 {
		changeRate = change / base * 100
	}

	changeColor := chartColorBlue
	changeSign := ""
	if change > 0 {
		changeColor = chartColorRed
		changeSign = "+"
	}

	name := chart.Name
	if name == "" {
		name = chart.Symbol
	}
	fmt.Fprintf(w, "%s%s (%s)%s  %s  %s%s%s (%s%.2f%%)  %s · %d candles\n\n",
		chartColorBold, name, chart.ProductCode, chartColorReset,
		formatPriceKR(last),
		changeColor, changeSign+formatPriceKR(change), chartColorReset,
		changeSign, changeRate,
		chart.Interval, len(chart.Candles),
	)

	return renderCandleChart(w, chart.Candles, 12)
}

func tickSizeKR(price float64) float64 {
	switch {
	case price < 2000:
		return 1
	case price < 5000:
		return 5
	case price < 20000:
		return 10
	case price < 50000:
		return 50
	case price < 200000:
		return 100
	case price < 500000:
		return 500
	default:
		return 1000
	}
}

func renderCandleChart(w io.Writer, candles []domain.Candle, desiredRows int) error {
	if len(candles) == 0 {
		return nil
	}

	rawMin, rawMax := candles[0].Low, candles[0].High
	for _, c := range candles {
		if c.Low < rawMin {
			rawMin = c.Low
		}
		if c.High > rawMax {
			rawMax = c.High
		}
	}

	mid := (rawMin + rawMax) / 2
	tick := tickSizeKR(mid)
	rawRange := rawMax - rawMin
	if rawRange == 0 {
		rawRange = tick
	}

	rawStep := rawRange / float64(desiredRows)
	ticksPerRow := int(math.Ceil(rawStep / tick))
	if ticksPerRow < 1 {
		ticksPerRow = 1
	}
	stepPrice := float64(ticksPerRow) * tick

	minPrice := math.Floor(rawMin/stepPrice)*stepPrice - stepPrice
	maxPrice := math.Ceil(rawMax/stepPrice)*stepPrice + stepPrice
	rows := int(math.Round((maxPrice - minPrice) / stepPrice))
	if rows < 1 {
		rows = 1
	}
	priceRange := maxPrice - minPrice
	totalSubrows := 2 * rows

	const eps = 1e-9
	toUpperSr := func(price float64) int {
		x := (maxPrice - price) / priceRange * float64(totalSubrows)
		sr := int(math.Floor(x + eps))
		if sr < 0 {
			sr = 0
		}
		if sr >= totalSubrows {
			sr = totalSubrows - 1
		}
		return sr
	}
	toLowerSr := func(price float64) int {
		x := (maxPrice - price) / priceRange * float64(totalSubrows)
		sr := int(math.Ceil(x-eps)) - 1
		if sr < 0 {
			sr = 0
		}
		if sr >= totalSubrows {
			sr = totalSubrows - 1
		}
		return sr
	}

	type metric struct {
		highSr, lowSr, bodyTopSr, bodyBotSr int
		up                                  bool
	}
	metrics := make([]metric, len(candles))
	for i, c := range candles {
		bodyTop, bodyBot := c.Open, c.Close
		if c.Close >= c.Open {
			bodyTop, bodyBot = c.Close, c.Open
		}
		metrics[i] = metric{
			highSr:    toUpperSr(c.High),
			lowSr:     toLowerSr(c.Low),
			bodyTopSr: toUpperSr(bodyTop),
			bodyBotSr: toLowerSr(bodyBot),
			up:        c.Close >= c.Open,
		}
	}

	var sb strings.Builder
	labelWidth := 9
	for r := 0; r < rows; r++ {
		rowTopPrice := maxPrice - float64(r)*stepPrice
		sb.WriteString(fmt.Sprintf("%*s ┤", labelWidth, formatPriceKR(rowTopPrice)))

		upperSub := 2 * r
		lowerSub := 2*r + 1

		for _, m := range metrics {
			color := chartColorBlue
			if m.up {
				color = chartColorRed
			}

			upperInBody := upperSub >= m.bodyTopSr && upperSub <= m.bodyBotSr
			lowerInBody := lowerSub >= m.bodyTopSr && lowerSub <= m.bodyBotSr
			upperInWick := upperSub >= m.highSr && upperSub <= m.lowSr
			lowerInWick := lowerSub >= m.highSr && lowerSub <= m.lowSr

			var ch string
			switch {
			case upperInBody && lowerInBody:
				ch = "█"
			case upperInBody:
				ch = "▀"
			case lowerInBody:
				ch = "▄"
			case upperInWick || lowerInWick:
				ch = "│"
			default:
				ch = " "
			}

			if ch == " " {
				sb.WriteString("   ")
			} else {
				sb.WriteString(" " + color + ch + chartColorReset + " ")
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString(strings.Repeat(" ", labelWidth+1))
	sb.WriteString("└")
	sb.WriteString(strings.Repeat("─", len(candles)*3))
	sb.WriteString("\n")

	labelStep := 1
	if len(candles) > 12 {
		labelStep = len(candles) / 6
		if labelStep < 1 {
			labelStep = 1
		}
	}
	sb.WriteString(strings.Repeat(" ", labelWidth+2))
	for i := 0; i < len(candles); i += labelStep {
		label := "     "
		if !candles[i].Time.IsZero() {
			label = candles[i].Time.Local().Format("15:04")
		}
		span := labelStep
		if i+labelStep > len(candles) {
			span = len(candles) - i
		}
		sb.WriteString(fmt.Sprintf("%-*s", 3*span, label))
	}
	sb.WriteString("\n")

	_, err := io.WriteString(w, sb.String())
	return err
}

func renderSparkline(candles []domain.Candle, width int) string {
	if len(candles) == 0 {
		return ""
	}

	closes := make([]float64, len(candles))
	min, max := candles[0].Close, candles[0].Close
	for i, c := range candles {
		closes[i] = c.Close
		if c.Close < min {
			min = c.Close
		}
		if c.Close > max {
			max = c.Close
		}
	}

	if width > 1 && len(closes) > width {
		sampled := make([]float64, width)
		for i := 0; i < width; i++ {
			idx := i * (len(closes) - 1) / (width - 1)
			sampled[i] = closes[idx]
		}
		closes = sampled
	}

	blocks := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	spread := max - min
	var sb strings.Builder
	for _, v := range closes {
		level := 0
		if spread > 0 {
			level = int((v - min) / spread * 7)
			if level > 7 {
				level = 7
			}
		}
		sb.WriteRune(blocks[level])
	}

	last := candles[len(candles)-1].Close
	first := candles[0].Close
	color := chartColorBlue
	if last >= first {
		color = chartColorRed
	}
	return color + sb.String() + chartColorReset
}

func formatPriceKR(p float64) string {
	negative := p < 0
	if negative {
		p = -p
	}
	intPart := int64(p + 0.5)
	s := fmt.Sprintf("%d", intPart)
	n := len(s)
	var out strings.Builder
	if negative {
		out.WriteString("-")
	}
	if n <= 3 {
		out.WriteString(s)
		return out.String()
	}
	pre := n % 3
	if pre > 0 {
		out.WriteString(s[:pre])
		if n > pre {
			out.WriteString(",")
		}
	}
	for i := pre; i < n; i += 3 {
		out.WriteString(s[i : i+3])
		if i+3 < n {
			out.WriteString(",")
		}
	}
	return out.String()
}
