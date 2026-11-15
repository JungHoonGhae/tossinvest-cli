package output

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
)

// WriteLedgerTransactions renders one page of the official transaction ledger.
// JSON and CSV keep the API's decimal strings verbatim (null becomes "" in CSV);
// the table only adds thousands separators, so no amount is rounded anywhere.
func WriteLedgerTransactions(w io.Writer, format Format, page domain.LedgerTransactionList) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, page)
	case FormatCSV:
		rows := make([][]string, 0, len(page.Transactions))
		for _, t := range page.Transactions {
			rows = append(rows, []string{
				t.SettlementDate, deref(t.TransactionTime), strconv.FormatBool(t.Settled),
				t.MarketCountry, t.Currency, t.TransactionType, strconv.FormatBool(t.IsCancellation),
				deref(t.AssetClass), deref(t.Symbol), deref(t.ISINCode),
				deref(t.Quantity), deref(t.FilledPrice), deref(t.Amount),
				t.Commission, t.Tax, deref(t.NetAmount), deref(t.ExchangeRate),
			})
		}
		return writeCSV(w, []string{
			"settlement_date", "transaction_time", "settled", "market_country", "currency",
			"transaction_type", "is_cancellation", "asset_class", "symbol", "isin_code",
			"quantity", "filled_price", "amount", "commission", "tax", "net_amount", "exchange_rate",
		}, rows)
	case FormatTable:
		return writeLedgerTable(w, page)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func writeLedgerTable(w io.Writer, page domain.LedgerTransactionList) error {
	if len(page.Transactions) == 0 {
		key := "output.ledger.empty"
		if page.HasNext {
			// The API can return an empty page before more rows; it is not the end.
			key = "output.ledger.emptyPage"
		}
		if _, err := fmt.Fprint(w, i18n.T(key)); err != nil {
			return err
		}
		return writeLedgerNext(w, page)
	}
	if _, err := fmt.Fprint(w, i18n.Tf("output.transactions.count", len(page.Transactions))); err != nil {
		return err
	}
	headers := []string{
		i18n.T("output.ledger.header.settlementDate"),
		i18n.T("output.ledger.header.status"),
		i18n.T("output.transactions.header.market"),
		i18n.T("output.transactions.header.type"),
		i18n.T("output.transactions.header.symbol"),
		i18n.T("output.transactions.header.quantity"),
		i18n.T("output.ledger.header.price"),
		i18n.T("output.transactions.header.amount"),
		i18n.T("output.ledger.header.netAmount"),
		i18n.T("output.ledger.header.currency"),
	}
	rows := make([][]string, 0, len(page.Transactions))
	for _, t := range page.Transactions {
		status := i18n.T("output.ledger.settled")
		if !t.Settled {
			status = i18n.T("output.ledger.unsettled")
		}
		kind := ledgerTypeLabel(t.TransactionType)
		if t.IsCancellation {
			kind += " " + i18n.T("output.ledger.cancellation")
		}
		// Bonds have no symbol; the ISIN is their only identifier.
		id := deref(t.Symbol)
		if id == "" {
			id = deref(t.ISINCode)
		}
		rows = append(rows, []string{
			t.SettlementDate, status, t.MarketCountry, kind, dashIfEmpty(id),
			ledgerDecimal(t.Quantity), ledgerDecimal(t.FilledPrice),
			ledgerDecimal(t.Amount), ledgerDecimal(t.NetAmount), t.Currency,
		})
	}
	if err := renderTable(w, headers, rows,
		AlignLeft, AlignLeft, AlignLeft, AlignLeft, AlignLeft,
		AlignRight, AlignRight, AlignRight, AlignRight, AlignLeft,
	); err != nil {
		return err
	}
	return writeLedgerNext(w, page)
}

func writeLedgerNext(w io.Writer, page domain.LedgerTransactionList) error {
	if !page.HasNext || page.NextCursor == "" {
		return nil
	}
	_, err := fmt.Fprint(w, i18n.Tf("output.ledger.next", page.NextCursor))
	return err
}

// ledgerTypeLabel translates the eleven documented types and shows anything
// newer as the raw enum, which the spec says clients must accept.
func ledgerTypeLabel(typ string) string {
	key := "output.ledger.type." + typ
	if label := i18n.T(key); label != key {
		return label
	}
	return typ
}

// ledgerDecimal groups the integer part of an unsigned decimal string without
// parsing the fraction, so the table shows exactly the digits the API sent.
func ledgerDecimal(v *string) string {
	if v == nil || *v == "" {
		return "-"
	}
	whole, frac, hasFrac := strings.Cut(*v, ".")
	if strings.HasPrefix(whole, "-") || strings.HasPrefix(whole, "+") {
		return *v
	}
	n, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return *v
	}
	out := formatWithCommas(n)
	if hasFrac {
		out += "." + frac
	}
	return out
}

func dashIfEmpty(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
