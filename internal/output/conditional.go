package output

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/orderintent"
)

// WriteConditionalOrders renders a list of conditional orders.
func WriteConditionalOrders(w io.Writer, format Format, l domain.ConditionalOrderList) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, l)
	case FormatCSV:
		var csvRows [][]string
		for _, o := range l.Orders {
			csvRows = append(csvRows, []string{o.ID, o.Type, o.Status, o.Symbol, strconv.FormatFloat(o.Quantity, 'f', -1, 64), o.OrderType, o.ExpireDate})
		}
		return writeCSV(w, []string{"id", "type", "status", "symbol", "quantity", "order_type", "expire_date"}, csvRows)
	case FormatTable:
		headers := []string{
			i18n.T("output.conditional.header.id"),
			i18n.T("output.conditional.header.type"),
			i18n.T("output.conditional.header.status"),
			i18n.T("output.conditional.header.symbol"),
			i18n.T("output.conditional.header.qty"),
			i18n.T("output.conditional.header.orderType"),
			i18n.T("output.conditional.header.expire"),
		}
		rows := make([][]string, 0, len(l.Orders))
		for _, o := range l.Orders {
			rows = append(rows, []string{o.ID, o.Type, o.Status, o.Symbol, strconv.FormatFloat(o.Quantity, 'f', -1, 64), o.OrderType, o.ExpireDate})
		}
		return renderTable(w, headers, rows)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// WriteConditionalOrder renders one conditional order: a summary line then a
// leg table (first, and second when present).
func WriteConditionalOrder(w io.Writer, format Format, o domain.ConditionalOrder) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, o)
	case FormatCSV:
		cw := csv.NewWriter(w)
		if err := cw.Write([]string{"leg", "cond_type", "status", "trigger_price", "order_price", "target_profit_rate"}); err != nil {
			return err
		}
		legRow := func(name string, c domain.ConditionalOrderCondition) []string {
			return []string{name, c.Type, c.Status,
				strconv.FormatFloat(c.TriggerPrice, 'f', -1, 64),
				strconv.FormatFloat(c.OrderPrice, 'f', -1, 64),
				strconv.FormatFloat(c.TargetProfitRate, 'f', -1, 64)}
		}
		if err := cw.Write(legRow(i18n.T("output.conditional.leg.first"), o.First)); err != nil {
			return err
		}
		if o.Second != nil {
			if err := cw.Write(legRow(i18n.T("output.conditional.leg.second"), *o.Second)); err != nil {
				return err
			}
		}
		cw.Flush()
		return cw.Error()
	case FormatTable:
		fmt.Fprintf(w, "%s %s  %s  %s  x%s  %s  ~%s\n",
			o.ID, o.Type, o.Status, o.Symbol,
			strconv.FormatFloat(o.Quantity, 'f', -1, 64), o.OrderType, o.ExpireDate)
		headers := []string{
			i18n.T("output.conditional.leg.header.leg"),
			i18n.T("output.conditional.leg.header.condType"),
			i18n.T("output.conditional.header.status"),
			i18n.T("output.conditional.leg.header.trigger"),
			i18n.T("output.conditional.leg.header.orderPrice"),
		}
		legRow := func(name string, c domain.ConditionalOrderCondition) []string {
			return []string{name, c.Type, c.Status,
				strconv.FormatFloat(c.TriggerPrice, 'f', -1, 64),
				strconv.FormatFloat(c.OrderPrice, 'f', -1, 64)}
		}
		rows := [][]string{legRow(i18n.T("output.conditional.leg.first"), o.First)}
		if o.Second != nil {
			rows = append(rows, legRow(i18n.T("output.conditional.leg.second"), *o.Second))
		}
		return renderTable(w, headers, rows)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// WriteConditionalPlacePreview renders a create preview: summary + confirm token.
func WriteConditionalPlacePreview(w io.Writer, intent orderintent.ConditionalPlaceIntent, token string) error {
	fmt.Fprintf(w, "%s  %s x%s  %s  %s\n",
		intent.Symbol, intent.Type, strconv.FormatFloat(intent.Quantity, 'f', -1, 64), intent.OrderType, intent.ExpireDate)
	leg := func(name string, l orderintent.ConditionLeg) {
		fmt.Fprintf(w, "  %s: %s trigger=%s order=%s\n", name, l.OrderSide,
			strconv.FormatFloat(l.TriggerPrice, 'f', -1, 64), strconv.FormatFloat(l.OrderPrice, 'f', -1, 64))
	}
	leg(i18n.T("output.conditional.leg.first"), intent.First)
	if intent.Second != nil {
		leg(i18n.T("output.conditional.leg.second"), *intent.Second)
	}
	fmt.Fprintf(w, "%s: %s\n", i18n.T("order.conditional.confirmToken"), token)
	return nil
}
