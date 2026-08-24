package output

import (
	"fmt"
	"io"
	"strconv"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// WriteAutoTrades renders the account's automated-trading rules.
func WriteAutoTrades(w io.Writer, format Format, l domain.AutoTradeList) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, l)
	case FormatCSV:
		var csvRows [][]string
		for _, a := range l.Items {
			csvRows = append(csvRows, []string{
				strconv.FormatInt(a.ID, 10), a.Type, a.Status, a.Symbol, a.Market,
				strconv.FormatFloat(a.Quantity, 'f', -1, 64),
				strconv.FormatFloat(a.TriggerPrice, 'f', -1, 64),
				strconv.FormatFloat(a.OrderPrice, 'f', -1, 64),
				a.Currency, a.CreatedAt,
			})
		}
		return writeCSV(w, []string{"id", "type", "status", "symbol", "market", "quantity", "trigger_price", "order_price", "currency", "created_at"}, csvRows)
	}

	if len(l.Items) == 0 {
		_, err := fmt.Fprintln(w, "설정된 자동매매가 없습니다.")
		return err
	}
	for _, a := range l.Items {
		name := a.Name
		if name == "" {
			// The list endpoint leaves productName null; the code is the only
			// identifier available without a second lookup per row.
			name = a.Symbol
		}
		if _, err := fmt.Fprintf(w, "%-14s %-14s %s (%s)\n", a.Type, a.Status, name, a.Market); err != nil {
			return err
		}
		qty := strconv.FormatFloat(a.Quantity, 'f', -1, 64)
		if a.AllQuantity {
			qty += " (전량)"
		}
		if _, err := fmt.Fprintf(w, "  수량 %s · 감시가 %s · 주문가 %s %s · %s\n",
			qty,
			strconv.FormatFloat(a.TriggerPrice, 'f', -1, 64),
			strconv.FormatFloat(a.OrderPrice, 'f', -1, 64),
			a.Currency, a.TradeType); err != nil {
			return err
		}
		if a.CreatedAt != "" {
			if _, err := fmt.Fprintf(w, "  설정 %s\n", a.CreatedAt); err != nil {
				return err
			}
		}
	}
	if l.HasNext {
		if _, err := fmt.Fprintln(w, "\n(다음 페이지가 더 있습니다)"); err != nil {
			return err
		}
	}
	return nil
}
