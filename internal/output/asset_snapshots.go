package output

import (
	"fmt"
	"io"
	"strconv"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// WriteAssetPerformance renders the one-month daily portfolio valuation trend.
func WriteAssetPerformance(w io.Writer, format Format, performance domain.AssetPerformance) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, performance)
	case FormatCSV:
		rows := make([][]string, 0, len(performance.Points))
		for _, point := range performance.Points {
			rows = append(rows, []string{
				point.BaseDate,
				formatFloat(point.PrincipalAmount.KRW),
				formatFloat(point.EvaluatedAmount.KRW),
				formatFloat(point.ProfitLossRate.KRW),
				formatFloat(point.PrincipalAmount.USD),
				formatFloat(point.EvaluatedAmount.USD),
				formatFloat(point.ProfitLossRate.USD),
				strconv.FormatBool(point.Realtime),
			})
		}
		return writeCSV(w,
			[]string{"date", "principal_krw", "evaluated_krw", "profit_loss_rate_krw", "principal_usd", "evaluated_usd", "profit_loss_rate_usd", "realtime"},
			rows,
		)
	case FormatTable:
		return writeAssetPerformanceTable(w, performance)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func writeAssetPerformanceTable(w io.Writer, performance domain.AssetPerformance) error {
	if _, err := fmt.Fprintf(w, "Scope: %s  Range: %s/%s\n", performance.Scope, performance.Range, performance.StepUnit); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Change: KRW %s  USD %s\n", formatKRW(performance.EvaluatedAmountDiff.KRW), formatUSD(performance.EvaluatedAmountDiff.USD)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "High: %s  KRW %s  USD %s\nLow:  %s  KRW %s  USD %s\n\n",
		performance.MaxEvaluated.BaseDate, formatKRW(performance.MaxEvaluated.Amount.KRW), formatUSD(performance.MaxEvaluated.Amount.USD),
		performance.MinEvaluated.BaseDate, formatKRW(performance.MinEvaluated.Amount.KRW), formatUSD(performance.MinEvaluated.Amount.USD)); err != nil {
		return err
	}
	rows := make([][]string, 0, len(performance.Points))
	for _, point := range performance.Points {
		rows = append(rows, []string{
			point.BaseDate,
			formatKRW(point.PrincipalAmount.KRW),
			formatKRW(point.EvaluatedAmount.KRW),
			formatPct(point.ProfitLossRate.KRW),
			formatUSD(point.PrincipalAmount.USD),
			formatUSD(point.EvaluatedAmount.USD),
			formatPct(point.ProfitLossRate.USD),
			fmt.Sprint(point.Realtime),
		})
	}
	return renderTable(w,
		[]string{"DATE", "PRINCIPAL KRW", "EVALUATED KRW", "RETURN KRW", "PRINCIPAL USD", "EVALUATED USD", "RETURN USD", "REALTIME"},
		rows,
		AlignLeft, AlignRight, AlignRight, AlignRight, AlignRight, AlignRight, AlignRight, AlignLeft,
	)
}

// WriteAssetSnapshots renders one cursor page of dated portfolio valuations.
func WriteAssetSnapshots(w io.Writer, format Format, page domain.AssetSnapshotPage) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, page)
	case FormatCSV:
		rows := make([][]string, 0, len(page.Snapshots))
		for _, point := range page.Snapshots {
			rows = append(rows, []string{
				point.BaseDate,
				formatFloat(point.PrincipalAmount.KRW),
				formatFloat(point.EvaluatedAmount.KRW),
				formatFloat(point.ProfitLossAmount.KRW),
				formatFloat(point.ProfitLossRate.KRW),
				formatFloat(point.PrincipalAmount.USD),
				formatFloat(point.EvaluatedAmount.USD),
				formatFloat(point.ProfitLossAmount.USD),
				formatFloat(point.ProfitLossRate.USD),
				strconv.FormatBool(point.Realtime),
				strconv.FormatBool(point.EvaluationComplete),
			})
		}
		return writeCSV(w,
			[]string{"date", "principal_krw", "evaluated_krw", "profit_loss_krw", "profit_loss_rate_krw", "principal_usd", "evaluated_usd", "profit_loss_usd", "profit_loss_rate_usd", "realtime", "evaluation_complete"},
			rows,
		)
	case FormatTable:
		return writeAssetSnapshotsTable(w, page)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func writeAssetSnapshotsTable(w io.Writer, page domain.AssetSnapshotPage) error {
	if _, err := fmt.Fprintf(w, "Scope: %s  Returned: %d  Page size: %d\n\n", page.Scope, len(page.Snapshots), page.PageSize); err != nil {
		return err
	}
	rows := make([][]string, 0, len(page.Snapshots))
	for _, point := range page.Snapshots {
		rows = append(rows, []string{
			point.BaseDate,
			formatKRW(point.PrincipalAmount.KRW),
			formatKRW(point.EvaluatedAmount.KRW),
			formatKRW(point.ProfitLossAmount.KRW),
			formatPct(point.ProfitLossRate.KRW),
			formatUSD(point.PrincipalAmount.USD),
			formatUSD(point.EvaluatedAmount.USD),
			formatUSD(point.ProfitLossAmount.USD),
			formatPct(point.ProfitLossRate.USD),
			strconv.FormatBool(point.Realtime),
			strconv.FormatBool(point.EvaluationComplete),
		})
	}
	if err := renderTable(w,
		[]string{"DATE", "PRINCIPAL KRW", "EVALUATED KRW", "P/L KRW", "RETURN KRW", "PRINCIPAL USD", "EVALUATED USD", "P/L USD", "RETURN USD", "REALTIME", "COMPLETE"},
		rows,
		AlignLeft, AlignRight, AlignRight, AlignRight, AlignRight, AlignRight, AlignRight, AlignRight, AlignRight, AlignLeft, AlignLeft,
	); err != nil {
		return err
	}
	if page.HasNext && page.NextCursor != "" {
		_, err := fmt.Fprintf(w, "Next cursor: %s\n", page.NextCursor)
		return err
	}
	return nil
}

// WriteAssetSnapshot renders one dated all-market valuation.
func WriteAssetSnapshot(w io.Writer, format Format, detail domain.AssetSnapshotDetail) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, detail)
	case FormatCSV:
		rows := make([][]string, 0, 1+len(detail.Markets))
		appendSummary := func(market string, principal, evaluated, profitLoss, rate domain.AssetAmount) {
			rows = append(rows, []string{
				"summary", market, "", "", "", "", "", "", "", "", "",
				formatFloat(principal.KRW), formatFloat(principal.USD),
				formatFloat(evaluated.KRW), formatFloat(evaluated.USD),
				formatFloat(profitLoss.KRW), formatFloat(profitLoss.USD),
				formatFloat(rate.KRW), formatFloat(rate.USD),
			})
		}
		appendSummary("total", detail.PrincipalAmount, detail.EvaluatedAmount, detail.ProfitLossAmount, detail.ProfitLossRate)
		for _, market := range detail.Markets {
			appendSummary(market.Market, market.PrincipalAmount, market.EvaluatedAmount, market.ProfitLossAmount, market.ProfitLossRate)
			for _, holding := range market.Holdings {
				rows = append(rows, []string{
					"holding", market.Market, holding.ProductCode, holding.ISIN, holding.Symbol, holding.Name, holding.MarketDivision, holding.Type,
					formatFloat(holding.Quantity), formatFloat(holding.PurchasePrice.KRW), formatFloat(holding.PurchasePrice.USD),
					formatFloat(holding.PurchaseAmount.KRW), formatFloat(holding.PurchaseAmount.USD),
					formatFloat(holding.EvaluatedAmount.KRW), formatFloat(holding.EvaluatedAmount.USD),
					formatFloat(holding.ProfitLossAmount.KRW), formatFloat(holding.ProfitLossAmount.USD),
					formatFloat(holding.ProfitLossRate.KRW), formatFloat(holding.ProfitLossRate.USD),
				})
			}
		}
		return writeCSV(w, []string{
			"row_type", "market", "product_code", "isin", "symbol", "name", "market_division", "type", "quantity",
			"purchase_price_krw", "purchase_price_usd", "principal_or_purchase_krw", "principal_or_purchase_usd",
			"evaluated_krw", "evaluated_usd", "profit_loss_krw", "profit_loss_usd", "profit_loss_rate_krw", "profit_loss_rate_usd",
		}, rows)
	case FormatTable:
		return writeAssetSnapshotTable(w, detail)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func writeAssetSnapshotTable(w io.Writer, detail domain.AssetSnapshotDetail) error {
	if _, err := fmt.Fprintf(w, "Scope: %s  Date: %s  Complete: %t\n\nPortfolio\n", detail.Scope, detail.BaseDate, detail.EvaluationComplete); err != nil {
		return err
	}
	summaryRows := [][]string{{
		"total",
		formatKRW(detail.PrincipalAmount.KRW), formatKRW(detail.EvaluatedAmount.KRW), formatKRW(detail.ProfitLossAmount.KRW), formatPct(detail.ProfitLossRate.KRW),
		formatUSD(detail.PrincipalAmount.USD), formatUSD(detail.EvaluatedAmount.USD), formatUSD(detail.ProfitLossAmount.USD), formatPct(detail.ProfitLossRate.USD),
	}}
	for _, market := range detail.Markets {
		summaryRows = append(summaryRows, []string{
			market.Market,
			formatKRW(market.PrincipalAmount.KRW), formatKRW(market.EvaluatedAmount.KRW), formatKRW(market.ProfitLossAmount.KRW), formatPct(market.ProfitLossRate.KRW),
			formatUSD(market.PrincipalAmount.USD), formatUSD(market.EvaluatedAmount.USD), formatUSD(market.ProfitLossAmount.USD), formatPct(market.ProfitLossRate.USD),
		})
	}
	if err := renderTable(w,
		[]string{"MARKET", "PRINCIPAL KRW", "EVALUATED KRW", "P/L KRW", "RETURN KRW", "PRINCIPAL USD", "EVALUATED USD", "P/L USD", "RETURN USD"},
		summaryRows,
		AlignLeft, AlignRight, AlignRight, AlignRight, AlignRight, AlignRight, AlignRight, AlignRight, AlignRight,
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "\nHoldings"); err != nil {
		return err
	}
	var holdingRows [][]string
	for _, market := range detail.Markets {
		for _, holding := range market.Holdings {
			holdingRows = append(holdingRows, []string{
				market.Market,
				holding.Symbol,
				holding.Name,
				holding.MarketDivision,
				formatQty(holding.Quantity),
				formatKRW(holding.PurchaseAmount.KRW),
				formatKRW(holding.EvaluatedAmount.KRW),
				formatKRW(holding.ProfitLossAmount.KRW),
				formatPct(holding.ProfitLossRate.KRW),
				formatUSD(holding.PurchaseAmount.USD),
				formatUSD(holding.EvaluatedAmount.USD),
				formatUSD(holding.ProfitLossAmount.USD),
				formatPct(holding.ProfitLossRate.USD),
			})
		}
	}
	return renderTable(w,
		[]string{"MARKET", "SYMBOL", "NAME", "VENUE", "QTY", "PURCHASE KRW", "EVALUATED KRW", "P/L KRW", "RETURN KRW", "PURCHASE USD", "EVALUATED USD", "P/L USD", "RETURN USD"},
		holdingRows,
		AlignLeft, AlignLeft, AlignLeft, AlignLeft, AlignRight, AlignRight, AlignRight, AlignRight, AlignRight, AlignRight, AlignRight, AlignRight, AlignRight,
	)
}
