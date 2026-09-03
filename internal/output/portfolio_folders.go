package output

import (
	"fmt"
	"io"
	"strconv"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// WritePortfolioFolders renders the grouped, fee-aware portfolio view without
// exposing the session-bound folder or item identifiers used by mutations.
func WritePortfolioFolders(w io.Writer, format Format, view domain.PortfolioFolders) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, view)
	case FormatCSV:
		rows := make([][]string, 0, len(view.Folders))
		for _, folder := range view.Folders {
			rows = append(rows, portfolioFolderCSVRow("folder", folder, domain.PortfolioFolderItem{}))
			for _, item := range folder.Items {
				rows = append(rows, portfolioFolderCSVRow("holding", folder, item))
			}
		}
		return writeCSV(w, []string{
			"row_type", "folder_name", "folder_type", "folder_default",
			"product_code", "symbol", "name", "quantity", "tradable_quantity", "market_division",
			"evaluated_krw", "evaluated_usd", "evaluated_after_fees_krw", "evaluated_after_fees_usd",
			"profit_loss_krw", "profit_loss_usd", "profit_loss_after_fees_krw", "profit_loss_after_fees_usd",
			"profit_loss_rate_krw", "profit_loss_rate_usd", "profit_loss_rate_after_fees_krw", "profit_loss_rate_after_fees_usd",
			"commission_krw", "commission_usd",
		}, rows)
	case FormatTable:
		if _, err := fmt.Fprintf(w,
			"Portfolio: evaluated=%s (%s), after fees=%s (%s), P/L=%s (%s), after fees=%s (%s)\nHidden holdings: %d (amount=%s)\n",
			formatKRW(view.EvaluatedAmount.KRW), formatUSD(view.EvaluatedAmount.USD),
			formatKRW(view.EvaluatedAmountAfterFees.KRW), formatUSD(view.EvaluatedAmountAfterFees.USD),
			formatKRW(view.ProfitLossAmount.KRW), formatUSD(view.ProfitLossAmount.USD),
			formatKRW(view.ProfitLossAfterFees.KRW), formatUSD(view.ProfitLossAfterFees.USD),
			view.Hidden.Count, formatKRW(view.Hidden.Amount),
		); err != nil {
			return err
		}
		for _, folder := range view.Folders {
			if _, err := fmt.Fprintf(w,
				"\n%s [%s]: evaluated=%s, after fees=%s, P/L=%s, after fees=%s\n",
				folder.Name, folder.Type,
				formatKRW(folder.EvaluatedAmount.KRW), formatKRW(folder.EvaluatedAmountAfterFees.KRW),
				formatKRW(folder.ProfitLossAmount.KRW), formatKRW(folder.ProfitLossAfterFees.KRW),
			); err != nil {
				return err
			}
			rows := make([][]string, 0, len(folder.Items))
			for _, item := range folder.Items {
				rows = append(rows, []string{
					fmt.Sprintf("%s (%s)", item.Name, item.Symbol),
					formatQty(item.Quantity),
					formatQty(item.TradableQuantity),
					formatKRW(item.EvaluatedAmountAfterFees.KRW),
					formatKRW(item.ProfitLossAfterFees.KRW),
					formatPct(item.ProfitLossRateAfterFees.KRW),
					formatKRW(item.Commission.KRW),
				})
			}
			if err := renderTable(w, []string{"Holding", "Quantity", "Tradable", "After fees", "P/L after fees", "Rate after fees", "Commission"}, rows); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func portfolioFolderCSVRow(rowType string, folder domain.PortfolioFolder, item domain.PortfolioFolderItem) []string {
	evaluated := folder.EvaluatedAmount
	evaluatedAfterFees := folder.EvaluatedAmountAfterFees
	profitLoss := folder.ProfitLossAmount
	profitLossAfterFees := folder.ProfitLossAfterFees
	profitLossRate := folder.ProfitLossRate
	profitLossRateAfterFees := folder.ProfitLossRateAfterFees
	if rowType == "holding" {
		evaluated = item.EvaluatedAmount
		evaluatedAfterFees = item.EvaluatedAmountAfterFees
		profitLoss = item.ProfitLossAmount
		profitLossAfterFees = item.ProfitLossAfterFees
		profitLossRate = item.ProfitLossRate
		profitLossRateAfterFees = item.ProfitLossRateAfterFees
	}
	return []string{
		rowType, folder.Name, folder.Type, strconv.FormatBool(folder.Default),
		item.ProductCode, item.Symbol, item.Name, formatFloat(item.Quantity), formatFloat(item.TradableQuantity), item.MarketDivision,
		formatFloat(evaluated.KRW), formatFloat(evaluated.USD), formatFloat(evaluatedAfterFees.KRW), formatFloat(evaluatedAfterFees.USD),
		formatFloat(profitLoss.KRW), formatFloat(profitLoss.USD), formatFloat(profitLossAfterFees.KRW), formatFloat(profitLossAfterFees.USD),
		formatFloat(profitLossRate.KRW), formatFloat(profitLossRate.USD), formatFloat(profitLossRateAfterFees.KRW), formatFloat(profitLossRateAfterFees.USD),
		formatFloat(item.Commission.KRW), formatFloat(item.Commission.USD),
	}
}
