package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

func WriteAccounts(w io.Writer, format Format, accounts []domain.Account, primaryKey string) error {
	switch format {
	case FormatJSON:
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(map[string]any{
			"primary_key": primaryKey,
			"accounts":    accounts,
		})
	case FormatCSV:
		writer := csv.NewWriter(w)
		if err := writer.Write([]string{"id", "display_name", "name", "type", "markets", "primary"}); err != nil {
			return err
		}
		for _, account := range accounts {
			if err := writer.Write([]string{
				account.ID,
				account.DisplayName,
				account.Name,
				account.Type,
				strings.Join(account.Markets, "|"),
				strconv.FormatBool(account.Primary),
			}); err != nil {
				return err
			}
		}
		writer.Flush()
		return writer.Error()
	case FormatTable:
		if _, err := fmt.Fprintf(w, "Primary Key: %s\n", primaryKey); err != nil {
			return err
		}
		for _, account := range accounts {
			if _, err := fmt.Fprintf(
				w,
				"- %s [%s] type=%s primary=%t markets=%s\n",
				account.DisplayName,
				account.ID,
				account.Type,
				account.Primary,
				strings.Join(account.Markets, ","),
			); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func WriteAccountSummary(w io.Writer, format Format, summary domain.AccountSummary) error {
	switch format {
	case FormatJSON:
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(summary)
	case FormatCSV:
		writer := csv.NewWriter(w)
		if err := writer.Write([]string{"metric", "value"}); err != nil {
			return err
		}
		rows := [][2]string{
			{"total_asset_amount", formatFloat(summary.TotalAssetAmount)},
			{"evaluated_profit_amount", formatFloat(summary.EvaluatedProfitAmount)},
			{"profit_rate", formatFloat(summary.ProfitRate)},
			{"orderable_amount_krw", formatFloat(summary.OrderableAmountKRW)},
			{"orderable_amount_usd", formatFloat(summary.OrderableAmountUSD)},
		}
		for _, row := range rows {
			if err := writer.Write([]string{row[0], row[1]}); err != nil {
				return err
			}
		}
		writer.Flush()
		return writer.Error()
	case FormatTable:
		enabled := colorEnabled(w, format)
		profitAmtStr := formatFloat(summary.EvaluatedProfitAmount)
		profitRateStr := fmt.Sprintf("%.2f%%", summary.ProfitRate*100)
		if _, err := fmt.Fprintf(
			w,
			"Total Asset Amount: %s\nEvaluated Profit Amount: %s\nProfit Rate: %s\nOrderable KRW: %s\nOrderable USD: %s\n",
			formatFloat(summary.TotalAssetAmount),
			profitText(profitAmtStr, summary.EvaluatedProfitAmount, enabled),
			profitText(profitRateStr, summary.ProfitRate, enabled),
			formatFloat(summary.OrderableAmountKRW),
			formatFloat(summary.OrderableAmountUSD),
		); err != nil {
			return err
		}

		keys := make([]string, 0, len(summary.Markets))
		for key := range summary.Markets {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			market := summary.Markets[key]
			if _, err := fmt.Fprintf(
				w,
				"- %s: total=%s principal=%s profit=%s rate=%.2f%% orderable_krw=%s orderable_usd=%s\n",
				key,
				formatFloat(market.TotalAssetAmount),
				formatFloat(market.PrincipalAmount),
				formatFloat(market.EvaluatedProfitAmount),
				market.ProfitRate*100,
				formatFloat(market.OrderableAmountKRW),
				formatFloat(market.OrderableAmountUSD),
			); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}
