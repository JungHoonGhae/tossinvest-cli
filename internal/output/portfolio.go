package output

import (
	"encoding/csv"
	"fmt"
	"io"
	"sort"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
)

func WritePositions(w io.Writer, format Format, positions []domain.Position) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, positions)
	case FormatCSV:
		writer := csv.NewWriter(w)
		if err := writer.Write([]string{"product_code", "symbol", "name", "market_type", "market_code", "quantity", "average_price", "current_price", "market_value", "unrealized_pnl", "profit_rate", "daily_profit_loss", "daily_profit_rate", "average_price_usd", "current_price_usd", "market_value_usd", "unrealized_pnl_usd", "profit_rate_usd", "daily_profit_loss_usd", "daily_profit_rate_usd"}); err != nil {
			return err
		}
		for _, position := range positions {
			if err := writer.Write([]string{
				position.ProductCode,
				position.Symbol,
				position.Name,
				position.MarketType,
				position.MarketCode,
				formatFloat(position.Quantity),
				formatFloat(position.AveragePrice),
				formatFloat(position.CurrentPrice),
				formatFloat(position.MarketValue),
				formatFloat(position.UnrealizedPnL),
				formatFloat(position.ProfitRate),
				formatFloat(position.DailyProfitLoss),
				formatFloat(position.DailyProfitRate),
				formatFloat(position.AveragePriceUSD),
				formatFloat(position.CurrentPriceUSD),
				formatFloat(position.MarketValueUSD),
				formatFloat(position.UnrealizedPnLUSD),
				formatFloat(position.ProfitRateUSD),
				formatFloat(position.DailyProfitLossUSD),
				formatFloat(position.DailyProfitRateUSD),
			}); err != nil {
				return err
			}
		}
		writer.Flush()
		return writer.Error()
	case FormatTable:
		enabled := colorEnabled(w, format)
		return writePositionsTable(w, positions, enabled)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func WriteAllocation(w io.Writer, format Format, markets map[string]domain.AccountMarketSummary) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, markets)
	case FormatCSV:
		writer := csv.NewWriter(w)
		if err := writer.Write([]string{"market", "total_asset_amount", "principal_amount", "evaluated_profit_amount", "profit_rate"}); err != nil {
			return err
		}
		keys := sortedMarketKeys(markets)
		for _, key := range keys {
			market := markets[key]
			if err := writer.Write([]string{
				key,
				formatFloat(market.TotalAssetAmount),
				formatFloat(market.PrincipalAmount),
				formatFloat(market.EvaluatedProfitAmount),
				formatFloat(market.ProfitRate),
			}); err != nil {
				return err
			}
		}
		writer.Flush()
		return writer.Error()
	case FormatTable:
		keys := sortedMarketKeys(markets)
		for _, key := range keys {
			market := markets[key]
			if _, err := fmt.Fprintf(
				w,
				"- %s: total=%s principal=%s profit=%s rate=%.2f%%\n",
				key,
				formatFloat(market.TotalAssetAmount),
				formatFloat(market.PrincipalAmount),
				formatFloat(market.EvaluatedProfitAmount),
				market.ProfitRate*100,
			); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func writePositionsTable(w io.Writer, positions []domain.Position, enabled bool) error {
	hasUS := false
	for _, p := range positions {
		if p.MarketType == "US_STOCK" {
			hasUS = true
			break
		}
	}

	headers := []string{
		i18n.T("output.positions.header.symbol"),
		i18n.T("output.positions.header.quantity"),
		i18n.T("output.positions.header.avgPrice"),
		i18n.T("output.positions.header.currentPrice"),
		i18n.T("output.positions.header.marketValue"),
		i18n.T("output.positions.header.pnl"),
		i18n.T("output.positions.header.rate"),
		i18n.T("output.positions.header.dailyPnl"),
		i18n.T("output.positions.header.dailyRate"),
	}
	if hasUS {
		headers = append(headers, i18n.T("output.positions.header.usdPnl"), i18n.T("output.positions.header.usdRate"))
	}

	var coloredRows [][]string
	for _, p := range positions {
		label := fmt.Sprintf("%s (%s)", truncateName(p.Name, 16), p.Symbol)
		pnlStr := formatKRW(p.UnrealizedPnL)
		rateStr := formatPct(p.ProfitRate)
		dailyStr := formatKRW(p.DailyProfitLoss)
		dailyRateStr := formatPct(p.DailyProfitRate)

		colored := []string{
			label,
			formatQty(p.Quantity),
			formatKRW(p.AveragePrice),
			formatKRW(p.CurrentPrice),
			formatKRW(p.MarketValue),
			profitText(pnlStr, p.UnrealizedPnL, enabled),
			profitText(rateStr, p.ProfitRate, enabled),
			profitText(dailyStr, p.DailyProfitLoss, enabled),
			profitText(dailyRateStr, p.DailyProfitRate, enabled),
		}
		if hasUS {
			if p.MarketType == "US_STOCK" {
				usdPnlStr := formatUSD(p.UnrealizedPnLUSD)
				usdRateStr := formatPct(p.ProfitRateUSD)

				colored = append(colored,
					profitText(usdPnlStr, p.UnrealizedPnLUSD, enabled),
					profitText(usdRateStr, p.ProfitRateUSD, enabled),
				)
			} else {

				colored = append(colored, "", "")
			}
		}
		coloredRows = append(coloredRows, colored)
	}

	return renderTable(w, headers, coloredRows)
}

func sortedMarketKeys(markets map[string]domain.AccountMarketSummary) []string {
	keys := make([]string, 0, len(markets))
	for key := range markets {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
