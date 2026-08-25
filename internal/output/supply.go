package output

import (
	"encoding/csv"
	"fmt"
	"io"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
)

// WriteSupplySeries renders one supply series (투자자별·공매도·신용·대차·프로그램).
//
// Each kind gets its own columns — a single generic table would be mostly empty
// cells, since a record only carries the fields of its own kind.
//
// 미집계(nil)는 "-" 로 낸다. 0 으로 찍으면 "순매수 0" 과 구분되지 않는데, 수급에서
// 그 둘은 정반대 신호다.
func WriteSupplySeries(w io.Writer, format Format, s domain.SupplySeries) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, s)
	case FormatCSV:
		return writeSupplyCSV(w, s)
	case FormatTable:
		if len(s.Records) == 0 {
			_, err := fmt.Fprintln(w, i18n.T("output.supply.empty"))
			return err
		}
		if _, err := fmt.Fprintf(w, i18n.T("output.supply.header"),
			i18n.T("output.supply.kind."+string(s.Kind)), s.Symbol); err != nil {
			return err
		}
		headers, aligns := supplyTableLayout(s.Kind)
		var rows [][]string
		for _, r := range s.Records {
			rows = append(rows, supplyRow(s.Kind, r))
		}
		if err := renderTable(w, headers, rows, aligns...); err != nil {
			return err
		}
		// 커서가 있으면 알려준다 — 없으면 사용자가 더 볼 수 있는지 알 방법이 없다.
		if s.NextUntil != "" {
			if _, err := fmt.Fprintf(w, i18n.T("output.supply.more"), s.NextUntil); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// num renders a nullable figure. nil is "-", never 0.
func supplyNum(v *float64) string {
	if v == nil {
		return "-"
	}
	return formatFloat(*v)
}

func vol(v *domain.TradingVolume) string {
	if v == nil {
		return "-"
	}
	return formatFloat(v.NetBuy)
}

func credit(d *domain.CreditDetail) string {
	if d == nil {
		return "-"
	}
	return formatFloat(d.BalanceQuantity)
}

// supplyTableLayout returns the header row and column alignments for a given supply kind.
func supplyTableLayout(kind domain.SupplyKind) ([]string, []Align) {
	right := AlignRight
	left := AlignLeft
	date := i18n.T("output.supply.table.date")
	switch kind {
	case domain.SupplyInvestor:
		return []string{date,
				i18n.T("output.supply.table.investor.individual"),
				i18n.T("output.supply.table.investor.foreigner"),
				i18n.T("output.supply.table.investor.institution"),
				i18n.T("output.supply.table.investor.other")},
			[]Align{left, right, right, right, right}
	case domain.SupplyShort:
		return []string{date,
				i18n.T("output.supply.table.short.volume"),
				i18n.T("output.supply.table.short.amount"),
				i18n.T("output.supply.table.short.rate")},
			[]Align{left, right, right, right}
	case domain.SupplyCredit:
		return []string{date,
				i18n.T("output.supply.table.credit.marginLoan"),
				i18n.T("output.supply.table.credit.stockLoan")},
			[]Align{left, right, right}
	case domain.SupplyLending:
		return []string{date,
				i18n.T("output.supply.table.lending.execution"),
				i18n.T("output.supply.table.lending.repayment"),
				i18n.T("output.supply.table.lending.balance")},
			[]Align{left, right, right, right}
	case domain.SupplyProgram:
		return []string{date,
				i18n.T("output.supply.table.program.arbitrage"),
				i18n.T("output.supply.table.program.nonArbitrage")},
			[]Align{left, right, right}
	}
	return []string{date}, []Align{left}
}

// supplyRow converts a supply record to a string row matching supplyTableLayout.
func supplyRow(kind domain.SupplyKind, r domain.SupplyRecord) []string {
	switch kind {
	case domain.SupplyInvestor:
		return []string{r.Date, vol(r.Individual), vol(r.Foreigner), vol(r.Institution), vol(r.OtherCorporation)}
	case domain.SupplyShort:
		return []string{r.Date, supplyNum(r.ShortVolume), supplyNum(r.ShortAmount), supplyNum(r.ShortVolumeRate)}
	case domain.SupplyCredit:
		return []string{r.Date, credit(r.MarginLoan), credit(r.StockLoan)}
	case domain.SupplyLending:
		return []string{r.Date, supplyNum(r.LendingExecution), supplyNum(r.LendingRepayment), supplyNum(r.LendingBalanceQty)}
	case domain.SupplyProgram:
		return []string{r.Date, vol(r.Arbitrage), vol(r.NonArbitrage)}
	}
	return []string{r.Date}
}

var supplyCSVHeaders = map[domain.SupplyKind][]string{
	domain.SupplyInvestor: {"date", "individual_net", "foreigner_net", "institution_net", "other_corporation_net", "foreigner_holding_rate"},
	domain.SupplyShort:    {"date", "short_volume", "short_amount", "short_volume_rate", "short_amount_rate"},
	domain.SupplyCredit:   {"date", "margin_loan_balance", "margin_loan_rate", "stock_loan_balance", "stock_loan_rate"},
	domain.SupplyLending:  {"date", "execution", "repayment", "balance_quantity", "balance_amount"},
	domain.SupplyProgram:  {"date", "arbitrage_net", "non_arbitrage_net"},
}

func writeSupplyCSV(w io.Writer, s domain.SupplySeries) error {
	writer := csv.NewWriter(w)
	header, ok := supplyCSVHeaders[s.Kind]
	if !ok {
		return fmt.Errorf("unknown supply kind: %s", s.Kind)
	}
	if err := writer.Write(header); err != nil {
		return err
	}
	for _, r := range s.Records {
		var row []string
		switch s.Kind {
		case domain.SupplyInvestor:
			rate := ""
			if r.ForeignerHolding != nil {
				rate = formatFloat(r.ForeignerHolding.HoldingRate)
			}
			row = []string{r.Date, vol(r.Individual), vol(r.Foreigner), vol(r.Institution), vol(r.OtherCorporation), rate}
		case domain.SupplyShort:
			row = []string{r.Date, supplyNum(r.ShortVolume), supplyNum(r.ShortAmount), supplyNum(r.ShortVolumeRate), supplyNum(r.ShortAmountRate)}
		case domain.SupplyCredit:
			mb, mr, sb, sr := "-", "-", "-", "-"
			if r.MarginLoan != nil {
				mb, mr = formatFloat(r.MarginLoan.BalanceQuantity), formatFloat(r.MarginLoan.BalanceRate)
			}
			if r.StockLoan != nil {
				sb, sr = formatFloat(r.StockLoan.BalanceQuantity), formatFloat(r.StockLoan.BalanceRate)
			}
			row = []string{r.Date, mb, mr, sb, sr}
		case domain.SupplyLending:
			row = []string{r.Date, supplyNum(r.LendingExecution), supplyNum(r.LendingRepayment), supplyNum(r.LendingBalanceQty), supplyNum(r.LendingBalanceAmt)}
		case domain.SupplyProgram:
			row = []string{r.Date, vol(r.Arbitrage), vol(r.NonArbitrage)}
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}
