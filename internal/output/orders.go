package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
)

func WriteOrders(w io.Writer, format Format, orders []domain.Order) error {
	return writeOrderList(w, format, i18n.T("output.orders.pending.title"), i18n.T("output.orders.pending.empty"), orders)
}

func WriteCompletedOrders(w io.Writer, format Format, orders []domain.Order) error {
	return writeOrderList(w, format, i18n.T("output.orders.completed.title"), i18n.T("output.orders.completed.empty"), orders)
}

func WriteOrder(w io.Writer, format Format, order domain.Order) error {
	switch format {
	case FormatJSON:
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(order)
	case FormatCSV:
		writer := csv.NewWriter(w)
		if err := writer.Write([]string{"id", "resolved_from_id", "symbol", "name", "market", "side", "status", "quantity", "filled_quantity", "price", "average_execution_price", "order_date", "submitted_at"}); err != nil {
			return err
		}
		submittedAt := ""
		if order.SubmittedAt != nil {
			submittedAt = order.SubmittedAt.Format("2006-01-02T15:04:05Z07:00")
		}
		if err := writer.Write([]string{
			order.ID,
			order.ResolvedFromID,
			order.Symbol,
			order.Name,
			order.Market,
			order.Side,
			order.Status,
			strconv.FormatFloat(order.Quantity, 'f', -1, 64),
			strconv.FormatFloat(order.FilledQuantity, 'f', -1, 64),
			strconv.FormatFloat(order.Price, 'f', -1, 64),
			strconv.FormatFloat(order.AverageExecutionPrice, 'f', -1, 64),
			order.OrderDate,
			submittedAt,
		}); err != nil {
			return err
		}
		writer.Flush()
		return writer.Error()
	case FormatTable:
		if _, err := fmt.Fprintf(w, i18n.T("output.order.orderId"), order.ID); err != nil {
			return err
		}
		if order.ResolvedFromID != "" {
			if _, err := fmt.Fprintf(w, i18n.T("output.order.resolvedFrom"), order.ResolvedFromID); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, i18n.T("output.order.symbol"), order.Symbol); err != nil {
			return err
		}
		if order.Name != "" {
			if _, err := fmt.Fprintf(w, i18n.T("output.order.name"), order.Name); err != nil {
				return err
			}
		}
		if order.Market != "" {
			if _, err := fmt.Fprintf(w, i18n.T("output.order.market"), order.Market); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, i18n.T("output.order.sideStatus"), order.Side, order.Status); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, i18n.T("output.order.quantity"), strconv.FormatFloat(order.Quantity, 'f', -1, 64)); err != nil {
			return err
		}
		if order.FilledQuantity > 0 {
			if _, err := fmt.Fprintf(w, i18n.T("output.order.filledQuantity"), strconv.FormatFloat(order.FilledQuantity, 'f', -1, 64)); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, i18n.T("output.order.price"), strconv.FormatFloat(order.Price, 'f', -1, 64)); err != nil {
			return err
		}
		if order.AverageExecutionPrice > 0 {
			if _, err := fmt.Fprintf(w, i18n.T("output.order.avgExecutionPrice"), strconv.FormatFloat(order.AverageExecutionPrice, 'f', -1, 64)); err != nil {
				return err
			}
		}
		if order.OrderDate != "" {
			if _, err := fmt.Fprintf(w, i18n.T("output.order.orderDate"), order.OrderDate); err != nil {
				return err
			}
		}
		if order.SubmittedAt != nil {
			if _, err := fmt.Fprintf(w, i18n.T("output.order.submittedAt"), order.SubmittedAt.Format("2006-01-02 15:04:05Z07:00")); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func writeOrderList(w io.Writer, format Format, title, emptyMessage string, orders []domain.Order) error {
	switch format {
	case FormatJSON:
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(orders)
	case FormatCSV:
		writer := csv.NewWriter(w)
		if err := writer.Write([]string{"id", "symbol", "name", "market", "side", "status", "quantity", "filled_quantity", "price", "average_execution_price", "order_date", "submitted_at"}); err != nil {
			return err
		}
		for _, order := range orders {
			submittedAt := ""
			if order.SubmittedAt != nil {
				submittedAt = order.SubmittedAt.Format("2006-01-02T15:04:05Z07:00")
			}
			if err := writer.Write([]string{
				order.ID,
				order.Symbol,
				order.Name,
				order.Market,
				order.Side,
				order.Status,
				strconv.FormatFloat(order.Quantity, 'f', -1, 64),
				strconv.FormatFloat(order.FilledQuantity, 'f', -1, 64),
				strconv.FormatFloat(order.Price, 'f', -1, 64),
				strconv.FormatFloat(order.AverageExecutionPrice, 'f', -1, 64),
				order.OrderDate,
				submittedAt,
			}); err != nil {
				return err
			}
		}
		writer.Flush()
		return writer.Error()
	case FormatTable:
		if len(orders) == 0 {
			_, err := fmt.Fprintln(w, emptyMessage)
			return err
		}
		if _, err := fmt.Fprintf(w, i18n.T("output.orders.count"), title, len(orders)); err != nil {
			return err
		}
		headers := []string{
			i18n.T("output.orders.header.symbol"),
			i18n.T("output.orders.header.side"),
			i18n.T("output.orders.header.status"),
			i18n.T("output.orders.header.quantity"),
			i18n.T("output.orders.header.filled"),
			i18n.T("output.orders.header.price"),
			i18n.T("output.orders.header.orderId"),
		}
		var rows [][]string
		for _, order := range orders {
			rows = append(rows, []string{
				order.Symbol,
				order.Side,
				order.Status,
				formatQty(order.Quantity),
				formatQty(order.FilledQuantity),
				formatKRW(order.Price),
				order.ID,
			})
		}
		return renderTable(w, headers, rows)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}
