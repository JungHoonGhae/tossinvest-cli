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
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
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
		if _, err := fmt.Fprintf(w, i18n.T("output.account.primaryKey"), primaryKey); err != nil {
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
			i18n.T("output.account.summary.lines"),
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

// WriteAccountPrime renders the account's Toss Prime membership status and
// fee/interest benefit comparison.
func WriteAccountPrime(w io.Writer, format Format, p domain.PrimeStatus) error {
	switch format {
	case FormatJSON:
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(p)
	case FormatCSV:
		writer := csv.NewWriter(w)
		if err := writer.Write([]string{"metric", "non_prime", "prime", "benefit"}); err != nil {
			return err
		}
		rows := [][4]string{
			{"exchange_fee", strconv.Itoa(p.Exchange.NonPrimeFee), strconv.Itoa(p.Exchange.PrimeFee), strconv.Itoa(p.Exchange.BenefitFee)},
			{"interest_krw", strconv.Itoa(p.InterestKRW.NonPrimeInterest), strconv.Itoa(p.InterestKRW.PrimeInterest), strconv.Itoa(p.InterestKRW.BenefitInterest)},
			{"interest_usd", strconv.Itoa(p.InterestUSD.NonPrimeInterest), strconv.Itoa(p.InterestUSD.PrimeInterest), strconv.Itoa(p.InterestUSD.BenefitInterest)},
		}
		for _, row := range rows {
			if err := writer.Write(row[:]); err != nil {
				return err
			}
		}
		writer.Flush()
		return writer.Error()
	case FormatTable:
		if p.IsMember {
			primeType := ""
			if p.PrimeType != nil {
				primeType = *p.PrimeType
			}
			endAt := ""
			if p.BenefitsEndAt != nil {
				endAt = *p.BenefitsEndAt
			}
			if _, err := fmt.Fprintf(w, i18n.T("output.accountPrime.member.header"), primeType, endAt); err != nil {
				return err
			}
		} else {
			if _, err := fmt.Fprint(w, i18n.T("output.accountPrime.nonMember.header")); err != nil {
				return err
			}
		}
		header := i18n.T("output.accountPrime.comparisonHeader")
		if _, err := fmt.Fprintf(w, header, "", i18n.T("output.accountPrime.columnNonMember"), i18n.T("output.accountPrime.columnPrime"), i18n.T("output.accountPrime.columnBenefit")); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, header,
			i18n.T("output.accountPrime.exchangeLabel"),
			strconv.Itoa(p.Exchange.NonPrimeFee), strconv.Itoa(p.Exchange.PrimeFee), strconv.Itoa(p.Exchange.BenefitFee),
		); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, header,
			i18n.T("output.accountPrime.interestKrwLabel"),
			strconv.Itoa(p.InterestKRW.NonPrimeInterest), strconv.Itoa(p.InterestKRW.PrimeInterest), strconv.Itoa(p.InterestKRW.BenefitInterest),
		); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, header,
			i18n.T("output.accountPrime.interestUsdLabel"),
			strconv.Itoa(p.InterestUSD.NonPrimeInterest), strconv.Itoa(p.InterestUSD.PrimeInterest), strconv.Itoa(p.InterestUSD.BenefitInterest),
		); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}
