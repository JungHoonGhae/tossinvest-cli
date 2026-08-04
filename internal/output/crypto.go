package output

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
)

// WriteCryptoPrices renders the KRW crypto tape. JSON/CSV are
// language-invariant; the table view shows price, move, and the premium gap
// against the global market.
func WriteCryptoPrices(w io.Writer, format Format, p domain.CryptoPrices) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, p)
	case FormatCSV:
		writer := csv.NewWriter(w)
		header := []string{"symbol", "product_code", "close", "change", "change_rate", "open", "high", "low", "volume", "value", "high_52w", "low_52w", "premium", "premium_rate"}
		if err := writer.Write(header); err != nil {
			return err
		}
		for _, c := range p.Prices {
			row := []string{
				c.Symbol, c.ProductCode,
				strconv.FormatFloat(c.Close, 'f', -1, 64),
				strconv.FormatFloat(c.Change, 'f', -1, 64),
				strconv.FormatFloat(c.ChangeRate, 'f', -1, 64),
				strconv.FormatFloat(c.Open, 'f', -1, 64),
				strconv.FormatFloat(c.High, 'f', -1, 64),
				strconv.FormatFloat(c.Low, 'f', -1, 64),
				strconv.FormatFloat(c.Volume, 'f', -1, 64),
				strconv.FormatFloat(c.Value, 'f', -1, 64),
				strconv.FormatFloat(c.High52w, 'f', -1, 64),
				strconv.FormatFloat(c.Low52w, 'f', -1, 64),
				strconv.FormatFloat(c.Premium, 'f', -1, 64),
				strconv.FormatFloat(c.PremiumRate, 'f', -1, 64),
			}
			if err := writer.Write(row); err != nil {
				return err
			}
		}
		writer.Flush()
		return writer.Error()
	case FormatTable:
		if len(p.Prices) == 0 {
			_, err := fmt.Fprintln(w, i18n.T("output.crypto.empty"))
			return err
		}
		if _, err := io.WriteString(w, i18n.T("output.crypto.header")); err != nil {
			return err
		}
		for _, c := range p.Prices {
			if _, err := fmt.Fprintf(w, i18n.T("output.crypto.line"),
				c.Symbol, formatFloat(c.Close), formatSignedPercent(c.ChangeRate),
				formatFloat(c.High), formatFloat(c.Low)); err != nil {
				return err
			}
			// 프리미엄은 부호가 뜻의 전부다 — 음수면 국내가 글로벌보다 싸다.
			// 원 단위로 끊는다 — 서버는 소수 12자리까지 주는데 그대로 흘리면
			// 표가 읽히지 않는다. JSON/CSV 는 원값을 유지한다.
			if _, err := fmt.Fprintf(w, i18n.T("output.crypto.premiumLine"),
				formatSignedPercent(c.PremiumRate), fmt.Sprintf("%+.0f", c.Premium)); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// formatSignedPercent keeps the sign visible. The repo's formatPercent
// multiplies by 100; these rates already arrive as percent, so reusing it here
// would show a 0.4% premium as 40%.
func formatSignedPercent(v float64) string {
	return fmt.Sprintf("%+.2f%%", v)
}
