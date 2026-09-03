package output

import (
	"fmt"
	"io"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

func WritePriceAlerts(w io.Writer, format Format, alerts domain.PriceAlerts) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, alerts)
	case FormatCSV:
		rows := make([][]string, 0, len(alerts.Alerts))
		for _, alert := range alerts.Alerts {
			rows = append(rows, []string{alerts.ProductCode, formatFloat(alert.TargetPrice), alert.Currency})
		}
		return writeCSV(w, []string{"product_code", "target_price", "currency"}, rows)
	case FormatTable:
		rows := make([][]string, 0, len(alerts.Alerts))
		for _, alert := range alerts.Alerts {
			rows = append(rows, []string{alerts.ProductCode, formatFloat(alert.TargetPrice), alert.Currency})
		}
		return renderTable(w, []string{"PRODUCT", "TARGET PRICE", "CURRENCY"}, rows, AlignLeft, AlignRight, AlignLeft)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func WriteHiddenHoldings(w io.Writer, format Format, holdings domain.HiddenHoldings) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, holdings)
	case FormatCSV:
		rows := make([][]string, 0, len(holdings.Holdings))
		for _, holding := range holdings.Holdings {
			rows = append(rows, []string{
				holding.ProductCode,
				holding.Name,
				holding.Type,
				formatFloat(holding.TradableQuantity),
			})
		}
		return writeCSV(w, []string{"product_code", "name", "type", "tradable_quantity"}, rows)
	case FormatTable:
		rows := make([][]string, 0, len(holdings.Holdings))
		for _, holding := range holdings.Holdings {
			rows = append(rows, []string{
				holding.ProductCode,
				holding.Name,
				holding.Type,
				formatFloat(holding.TradableQuantity),
			})
		}
		return renderTable(w, []string{"PRODUCT", "NAME", "TYPE", "TRADABLE QTY"}, rows, AlignLeft, AlignLeft, AlignLeft, AlignRight)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}
