package output

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
)

func WriteTransactions(w io.Writer, format Format, items []domain.Transaction) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, items)
	case FormatCSV:
		writer := csv.NewWriter(w)
		if err := writer.Write([]string{
			"date", "datetime", "market", "currency", "category", "type", "display_type", "code", "display_name",
			"stock_code", "stock_name", "quantity", "amount", "adjusted_amount",
			"commission", "tax", "balance", "settlement_date", "summary",
			"reference_type", "reference_id", "sort_key",
		}); err != nil {
			return err
		}
		for _, it := range items {
			if err := writer.Write([]string{
				it.Date,
				it.DateTime,
				it.Market,
				it.Currency,
				it.Category,
				it.Type,
				it.DisplayType,
				it.Code,
				it.DisplayName,
				it.StockCode,
				it.StockName,
				strconv.FormatFloat(it.Quantity, 'f', -1, 64),
				strconv.FormatFloat(it.Amount, 'f', -1, 64),
				strconv.FormatFloat(it.AdjustedAmount, 'f', -1, 64),
				strconv.FormatFloat(it.CommissionAmount, 'f', -1, 64),
				strconv.FormatFloat(it.TaxAmount, 'f', -1, 64),
				strconv.FormatFloat(it.BalanceAmount, 'f', -1, 64),
				it.SettlementDate,
				it.Summary,
				it.ReferenceType,
				it.ReferenceID,
				it.SortKey,
			}); err != nil {
				return err
			}
		}
		writer.Flush()
		return writer.Error()
	case FormatTable:
		if len(items) == 0 {
			_, err := fmt.Fprint(w, i18n.T("output.transactions.empty"))
			return err
		}
		if _, err := fmt.Fprintf(w, i18n.T("output.transactions.count"), len(items)); err != nil {
			return err
		}
		headers := []string{
			i18n.T("output.transactions.header.when"),
			i18n.T("output.transactions.header.market"),
			i18n.T("output.transactions.header.type"),
			i18n.T("output.transactions.header.symbol"),
			i18n.T("output.transactions.header.quantity"),
			i18n.T("output.transactions.header.amount"),
			i18n.T("output.transactions.header.netCash"),
			i18n.T("output.transactions.header.balance"),
		}
		rows := make([][]string, 0, len(items))
		for _, it := range items {
			when := it.DateTime
			if when == "" {
				when = it.Date
			}
			if when == "" {
				when = it.SettlementDate
			}
			if len(when) > 19 {
				when = when[:19]
			}
			label := it.DisplayName
			if it.Summary != "" && it.Summary != it.DisplayName {
				label = it.DisplayName + " (" + it.Summary + ")"
			}
			name := it.StockName
			if name == "" {
				name = it.StockCode
			}
			rows = append(rows, []string{
				when,
				strings.ToUpper(it.Market),
				label,
				truncateName(name, 14),
				formatQty(it.Quantity),
				formatAmountByMarket(it.Market, it.Amount),
				formatAmountByMarket(it.Market, it.AdjustedAmount),
				formatAmountByMarket(it.Market, it.BalanceAmount),
			})
		}
		return renderTable(w, headers, rows)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func WriteTransactionsOverview(w io.Writer, format Format, overview domain.TransactionOverview) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, overview)
	case FormatCSV:
		writer := csv.NewWriter(w)
		if err := writer.Write([]string{"section", "date", "krw", "usd", "note"}); err != nil {
			return err
		}
		writeBuckets := func(section string, list []domain.SettlementBucket) error {
			for _, b := range list {
				if err := writer.Write([]string{
					section, b.Date,
					strconv.FormatFloat(b.KRW, 'f', -1, 64),
					strconv.FormatFloat(b.USD, 'f', -1, 64),
					"",
				}); err != nil {
					return err
				}
			}
			return nil
		}
		if err := writer.Write([]string{"orderable", "", strconv.FormatFloat(overview.OrderableKRW, 'f', -1, 64), strconv.FormatFloat(overview.OrderableUSD, 'f', -1, 64), ""}); err != nil {
			return err
		}
		if err := writeBuckets("withdrawable", overview.Withdrawable); err != nil {
			return err
		}
		if err := writeBuckets("display_withdrawable", overview.DisplayWithdrawable); err != nil {
			return err
		}
		if err := writeBuckets("deposit", overview.Deposit); err != nil {
			return err
		}
		for _, e := range overview.EstimateSettlement {
			if err := writer.Write([]string{
				"settlement", e.Date,
				strconv.FormatFloat(e.BuyAmount, 'f', -1, 64),
				strconv.FormatFloat(e.SellAmount, 'f', -1, 64),
				"buy/sell",
			}); err != nil {
				return err
			}
		}
		for _, b := range overview.WithdrawableBottomSheet {
			if err := writer.Write([]string{
				"withdrawable_breakdown", "",
				strconv.FormatFloat(b.KRW, 'f', -1, 64),
				strconv.FormatFloat(b.USD, 'f', -1, 64),
				b.Title,
			}); err != nil {
				return err
			}
		}
		writer.Flush()
		return writer.Error()
	case FormatTable:
		market := strings.ToUpper(overview.Market)
		if _, err := fmt.Fprintf(w, i18n.T("output.transactions.overview.market"), market); err != nil {
			return err
		}
		// Toss returns the US orderable/settlement amounts both in USD and in their
		// KRW equivalence; they represent the same money, not two separate pools.
		// Table view shows only the market's primary currency; JSON preserves both.
		primary := primaryCurrencyCode(overview.Market)
		if _, err := fmt.Fprintf(w, i18n.T("output.transactions.overview.orderable"), primary,
			pickPrimaryAmount(overview.Market, overview.OrderableKRW, overview.OrderableUSD)); err != nil {
			return err
		}
		dateHeader := i18n.T("output.transactions.overview.dateHeader")
		if err := renderBucketsForMarket(w, i18n.T("output.transactions.overview.withdrawable"), dateHeader, primary, overview.Market, overview.Withdrawable); err != nil {
			return err
		}
		if err := renderBucketsForMarket(w, i18n.T("output.transactions.overview.displayWithdrawable"), dateHeader, primary, overview.Market, overview.DisplayWithdrawable); err != nil {
			return err
		}
		if err := renderBucketsForMarket(w, i18n.T("output.transactions.overview.depositSchedule"), dateHeader, primary, overview.Market, overview.Deposit); err != nil {
			return err
		}
		if len(overview.EstimateSettlement) > 0 {
			fmt.Fprint(w, i18n.T("output.transactions.overview.settlementTitle"))
			headers := []string{
				i18n.T("output.transactions.overview.settlement.header.date"),
				i18n.T("output.transactions.overview.settlement.header.buy"),
				i18n.T("output.transactions.overview.settlement.header.sell"),
			}
			rows := make([][]string, 0, len(overview.EstimateSettlement))
			for _, e := range overview.EstimateSettlement {
				rows = append(rows, []string{
					e.Date,
					formatAmountByMarket(overview.Market, e.BuyAmount),
					formatAmountByMarket(overview.Market, e.SellAmount),
				})
			}
			if err := renderTable(w, headers, rows); err != nil {
				return err
			}
		}
		if len(overview.WithdrawableBottomSheet) > 0 {
			fmt.Fprint(w, i18n.T("output.transactions.overview.breakdownTitle"))
			headers := []string{i18n.T("output.transactions.overview.breakdown.header.item"), primary}
			rows := make([][]string, 0, len(overview.WithdrawableBottomSheet))
			for _, b := range overview.WithdrawableBottomSheet {
				rows = append(rows, []string{b.Title, pickPrimaryAmount(overview.Market, b.KRW, b.USD)})
			}
			if err := renderTable(w, headers, rows); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func formatAmountByMarket(market string, v float64) string {
	if strings.EqualFold(market, "us") {
		return formatUSD(v)
	}
	return formatKRW(v)
}

// primaryCurrencyCode returns the column header label for the market's primary
// currency (USD for us, KRW for everything else including kr).
func primaryCurrencyCode(market string) string {
	if strings.EqualFold(market, "us") {
		return "USD"
	}
	return "KRW"
}

// pickPrimaryAmount formats the market-primary currency amount. Toss returns USD
// market values as both KRW-equivalent and USD; they are the same money so table
// view shows only USD for US and KRW for KR.
func pickPrimaryAmount(market string, krw, usd float64) string {
	if strings.EqualFold(market, "us") {
		return formatUSD(usd)
	}
	return formatKRW(krw)
}

// renderBucketsForMarket prints a settlement bucket table with a single amount
// column matching the market's primary currency.
func renderBucketsForMarket(w io.Writer, title, dateHeader, primary, market string, buckets []domain.SettlementBucket) error {
	if len(buckets) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(w, "\n"+title); err != nil {
		return err
	}
	headers := []string{dateHeader, primary}
	rows := make([][]string, 0, len(buckets))
	for _, b := range buckets {
		rows = append(rows, []string{b.Date, pickPrimaryAmount(market, b.KRW, b.USD)})
	}
	return renderTable(w, headers, rows)
}
