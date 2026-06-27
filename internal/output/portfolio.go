package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

func WritePositions(w io.Writer, format Format, positions []domain.Position) error {
	switch format {
	case FormatJSON:
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(positions)
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
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(markets)
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

	headers := []string{"종목", "수량", "매입가", "현재가", "평가금", "손익", "수익률", "일간손익", "일간률"}
	if hasUS {
		headers = append(headers, "USD 손익", "USD 률")
	}

	var plainRows, coloredRows [][]string
	for _, p := range positions {
		label := fmt.Sprintf("%s (%s)", truncateName(p.Name, 16), p.Symbol)
		pnlStr := formatKRW(p.UnrealizedPnL)
		rateStr := formatPct(p.ProfitRate)
		dailyStr := formatKRW(p.DailyProfitLoss)
		dailyRateStr := formatPct(p.DailyProfitRate)

		plain := []string{
			label,
			formatQty(p.Quantity),
			formatKRW(p.AveragePrice),
			formatKRW(p.CurrentPrice),
			formatKRW(p.MarketValue),
			pnlStr,
			rateStr,
			dailyStr,
			dailyRateStr,
		}
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
				plain = append(plain, usdPnlStr, usdRateStr)
				colored = append(colored,
					profitText(usdPnlStr, p.UnrealizedPnLUSD, enabled),
					profitText(usdRateStr, p.ProfitRateUSD, enabled),
				)
			} else {
				plain = append(plain, "", "")
				colored = append(colored, "", "")
			}
		}
		plainRows = append(plainRows, plain)
		coloredRows = append(coloredRows, colored)
	}

	return renderTableColored(w, headers, plainRows, coloredRows)
}

func sortedMarketKeys(markets map[string]domain.AccountMarketSummary) []string {
	keys := make([]string, 0, len(markets))
	for key := range markets {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
