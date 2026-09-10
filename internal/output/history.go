package output

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/JungHoonGhae/tossinvest-cli/internal/briefing"
	"github.com/JungHoonGhae/tossinvest-cli/internal/history"
)

func historyRows(w io.Writer, format Format, header []string, rows [][]string) error {
	if format == FormatCSV {
		return writeCSV(w, header, rows)
	}
	return renderTable(w, header, rows)
}

func WriteHistorySync(w io.Writer, format Format, r history.SyncResult) error {
	if format == FormatJSON {
		return writeJSON(w, r)
	}
	id := ""
	warnings := ""
	if r.Snapshot != nil {
		id = strconv.FormatInt(r.Snapshot.ID, 10)
		warnings = strings.Join(r.Snapshot.Warnings, "; ")
	}
	return historyRows(w, format, []string{"dry_run", "database", "from", "to", "market", "previous_snapshot", "snapshot", "confirm_token", "warnings", "note"}, [][]string{{strconv.FormatBool(r.DryRun), r.Database, r.Intent.From, r.Intent.To, r.Intent.Market, strconv.FormatInt(r.PreviousSnapshot, 10), id, r.ConfirmToken, warnings, r.Note}})
}

func WriteHistoryList(w io.Writer, format Format, items []history.Snapshot) error {
	if format == FormatJSON {
		return writeJSON(w, items)
	}
	rows := [][]string{}
	for _, s := range items {
		rows = append(rows, []string{strconv.FormatInt(s.ID, 10), s.CollectedAt, s.From, s.To, strings.Join(s.Markets, ","), strconv.Itoa(s.PositionCount), strconv.Itoa(s.TransactionCount), strconv.FormatBool(s.Complete), strings.Join(s.Warnings, "; ")})
	}
	return historyRows(w, format, []string{"id", "collected_at", "from", "to", "markets", "positions", "transactions", "complete", "warnings"}, rows)
}

func WriteHistoryPositions(w io.Writer, format Format, data history.Holdings) error {
	if format == FormatJSON {
		return writeJSON(w, data)
	}
	if format == FormatTable {
		if _, err := fmt.Fprintf(w, "Snapshot %d · %s · WTS historical holdings\n", data.Snapshot.ID, data.Snapshot.CollectedAt); err != nil {
			return err
		}
		return WritePositions(w, format, data.Positions)
	}
	rows := [][]string{}
	for _, p := range data.Positions {
		rows = append(rows, []string{strconv.FormatInt(data.Snapshot.ID, 10), data.Snapshot.CollectedAt, p.Symbol, p.Name, formatFloat(p.Quantity), formatFloat(p.MarketValue), formatFloat(p.MarketValueUSD), formatFloat(p.UnrealizedPnL)})
	}
	return writeCSV(w, []string{"snapshot_id", "collected_at", "symbol", "name", "quantity", "market_value_krw", "market_value_usd", "unrealized_pnl_krw"}, rows)
}

func WriteHistoryTransactions(w io.Writer, format Format, data history.Transactions) error {
	if format == FormatJSON {
		return writeJSON(w, data)
	}
	if format == FormatTable {
		if _, err := fmt.Fprintf(w, "Snapshot %d · %s · complete=%t · has_more=%t · next_offset=%d\n%s\n", data.Snapshot.ID, data.Snapshot.CollectedAt, data.Snapshot.Complete, data.HasMore, data.NextOffset, strings.Join(data.Snapshot.Warnings, "; ")); err != nil {
			return err
		}
	}
	rows := [][]string{}
	for _, t := range data.Items {
		rows = append(rows, []string{strconv.FormatInt(data.Snapshot.ID, 10), data.Snapshot.CollectedAt, t.Date, t.Market, t.Currency, t.Category, t.StockCode, t.StockName, t.Summary, formatFloat(t.Quantity), formatFloat(t.Amount), formatFloat(t.AdjustedAmount), strconv.FormatBool(data.Snapshot.Complete), strconv.FormatBool(data.HasMore), strconv.Itoa(data.NextOffset), strings.Join(data.Snapshot.Warnings, "; ")})
	}
	return historyRows(w, format, []string{"snapshot_id", "collected_at", "date", "market", "currency", "category", "stock_code", "stock_name", "summary", "quantity", "amount", "adjusted_amount", "complete", "has_more", "next_offset", "warnings"}, rows)
}

func WriteHistoryComparison(w io.Writer, format Format, data history.Comparison) error {
	if format == FormatJSON {
		return writeJSON(w, data)
	}
	if format == FormatTable {
		if _, err := fmt.Fprintln(w, data.Note); err != nil {
			return err
		}
	}
	rows := [][]string{}
	for _, c := range data.Changes {
		rows = append(rows, []string{strconv.FormatInt(data.Before.ID, 10), data.Before.CollectedAt, strconv.FormatInt(data.After.ID, 10), data.After.CollectedAt, c.Symbol, c.Status, formatFloat(c.QuantityBefore), formatFloat(c.QuantityAfter), formatFloat(c.QuantityDelta), formatFloat(c.ValueDeltaKRW), formatFloat(c.ValueDeltaUSD), formatFloat(c.PnLDeltaKRW)})
	}
	return historyRows(w, format, []string{"before_id", "before_at", "after_id", "after_at", "symbol", "status", "quantity_before", "quantity_after", "quantity_delta", "value_delta_krw", "value_delta_usd", "unrealized_pnl_delta_krw"}, rows)
}

func WritePortfolioBriefing(w io.Writer, format Format, data briefing.Result) error {
	if format == FormatJSON {
		return writeJSON(w, data)
	}
	rows := [][]string{{"coverage", "", data.Coverage, "", data.CollectedAt}}
	for _, name := range []string{"positions", "earnings", "news", "pending_orders"} {
		s := data.Sections[name]
		rows = append(rows, []string{"section", name, s.Status, s.Warning, s.FetchedAt})
	}
	for _, p := range data.Positions {
		rows = append(rows, []string{"position", p.Symbol, p.Name, "quantity=" + formatFloat(p.Quantity) + "; value_krw=" + formatFloat(p.MarketValue), data.Sections["positions"].FetchedAt})
	}
	for _, e := range data.Earnings {
		rows = append(rows, []string{"earnings", e.CompanyCode, e.Title, e.LiveAt, data.Sections["earnings"].FetchedAt})
	}
	for _, n := range data.News {
		rows = append(rows, []string{"news", n.ID, n.Title, n.Source + " · " + n.CreatedAt, data.Sections["news"].FetchedAt})
	}
	for _, o := range data.PendingOrders {
		rows = append(rows, []string{"pending_order", o.Symbol, o.Name, o.Side + " quantity=" + formatFloat(o.Quantity) + " price=" + formatFloat(o.Price), data.Sections["pending_orders"].FetchedAt})
	}
	return historyRows(w, format, []string{"kind", "key", "title", "detail", "fetched_at"}, rows)
}
