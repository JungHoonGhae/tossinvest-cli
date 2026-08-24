package output

import (
	"encoding/csv"
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
		return writeJSON(w, map[string]any{
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
		return writeJSON(w, summary)
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
		return writeJSON(w, p)
	case FormatCSV:
		writer := csv.NewWriter(w)
		if err := writer.Write([]string{"metric", "non_prime", "prime", "benefit"}); err != nil {
			return err
		}
		rows := [][4]string{
			{"exchange_fee", strconv.Itoa(p.Exchange.NonPrimeFee), strconv.Itoa(p.Exchange.PrimeFee), strconv.Itoa(p.Exchange.BenefitFee)},
			{"interest_krw", strconv.Itoa(p.InterestKRW.NonPrimeInterest), strconv.Itoa(p.InterestKRW.PrimeInterest), strconv.Itoa(p.InterestKRW.BenefitInterest)},
			{"interest_usd", strconv.Itoa(p.InterestUSD.NonPrimeInterest), strconv.Itoa(p.InterestUSD.PrimeInterest), strconv.Itoa(p.InterestUSD.BenefitInterest)},
			// 누적은 비교 대상이 없어 benefit 열에만 값이 있다.
			{"cumulative_exchange", "", "", strconv.FormatFloat(p.Cumulative.Exchange, 'f', -1, 64)},
			{"cumulative_interest_krw", "", "", strconv.FormatFloat(p.Cumulative.InterestKRW, 'f', -1, 64)},
			{"cumulative_interest_usd", "", "", strconv.FormatFloat(p.Cumulative.InterestUSD, 'f', -1, 64)},
			{"cumulative_total_krw", "", "", strconv.FormatFloat(p.Cumulative.TotalKRW, 'f', -1, 64)},
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
		// 위 표는 이번 달치다. 가입 이후 누적은 다른 질문이라 따로 적는다.
		if _, err := fmt.Fprintf(w, i18n.T("output.accountPrime.cumulative"),
			formatKRW(p.Cumulative.TotalKRW), formatKRW(p.Cumulative.Exchange),
			formatKRW(p.Cumulative.InterestKRW), formatKRW(p.Cumulative.InterestUSD)); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// commissionRow is one trading surface's line. Code is the stable machine
// identifier (CSV); LabelKey is the localized display name (table).
type commissionRow struct {
	Code     string
	LabelKey string
	Tier     domain.CommissionTier
}

// commissionRows flattens the schedule so CSV and table stay in sync. The
// value column is not a single unit: equities carry a percentage of notional,
// US options a flat per-contract fee — see formatCommissionValue.
func commissionRows(s domain.CommissionSchedule) []commissionRow {
	rows := []commissionRow{
		{"korea", "output.accountCommission.koreaLabel", s.Korea},
		{"us", "output.accountCommission.usLabel", s.US},
	}
	if s.USOptions != nil {
		rows = append(rows, commissionRow{"us_options", "output.accountCommission.usOptionsLabel", *s.USOptions})
	}
	return rows
}

// formatCommissionValue renders a tier's headline number. The API already
// reports rates in percent (KR 0.015 means 0.015%), so this must NOT reuse
// formatPercent, which multiplies by 100.
func formatCommissionValue(t domain.CommissionTier) string {
	if t.PerContract > 0 {
		return "$" + formatFloat(t.PerContract) + i18n.T("output.accountCommission.perContractSuffix")
	}
	return formatFloat(t.RatePercent) + "%"
}

// WriteAccountCommission renders the account's commission schedule per trading
// surface. Distinct from WriteCommission, which is per-symbol.
func WriteAccountCommission(w io.Writer, format Format, s domain.CommissionSchedule) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, s)
	case FormatCSV:
		writer := csv.NewWriter(w)
		if err := writer.Write([]string{"surface", "rate_percent", "per_contract", "has_reduction", "reduction_end_at"}); err != nil {
			return err
		}
		for _, row := range commissionRows(s) {
			if err := writer.Write([]string{
				row.Code,
				formatFloat(row.Tier.RatePercent),
				formatFloat(row.Tier.PerContract),
				strconv.FormatBool(row.Tier.HasReduction),
				row.Tier.ReductionEndAt,
			}); err != nil {
				return err
			}
		}
		writer.Flush()
		return writer.Error()
	case FormatTable:
		if _, err := fmt.Fprint(w, i18n.T("output.accountCommission.header")); err != nil {
			return err
		}
		// 값 열은 단위가 행마다 다르다(% vs $/계약). 열을 나누면 대부분의 계좌에서
		// 한쪽이 빈 칸이라, 단위를 값에 붙여 한 열로 둔다 — formatCommissionValue.
		for _, row := range commissionRows(s) {
			note := ""
			if row.Tier.HasReduction {
				note = fmt.Sprintf(i18n.T("output.accountCommission.reduction"), row.Tier.ReductionEndAt)
			}
			line := fmt.Sprintf(i18n.T("output.accountCommission.row"),
				i18n.T(row.LabelKey), formatCommissionValue(row.Tier), note)
			// 비고가 비면 값 열의 패딩이 줄 끝에 남는다. 눈에는 안 보이지만
			// 출력을 파이프로 받는 쪽에서는 다르게 읽힌다.
			if _, err := fmt.Fprintln(w, strings.TrimRight(line, " \n")); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// WriteAccountInterest renders one year's deposit-interest payments.
// Months with no payment are omitted — the server returns all 12 regardless,
// and printing ten empty rows buries the two that matter.
func WriteAccountInterest(w io.Writer, format Format, ai domain.AccountInterest) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, ai)
	case FormatCSV:
		writer := csv.NewWriter(w)
		if err := writer.Write([]string{"month", "date", "amount", "tax", "payment_amount", "start_date", "end_date", "estimated"}); err != nil {
			return err
		}
		for _, m := range ai.Monthly {
			for _, p := range m.Payments {
				if err := writer.Write([]string{
					strconv.Itoa(m.Month),
					p.Date,
					formatFloat(p.Amount),
					formatFloat(p.Tax),
					formatFloat(p.PaymentAmount),
					p.StartDate,
					p.EndDate,
					strconv.FormatBool(p.Estimated),
				}); err != nil {
					return err
				}
			}
		}
		writer.Flush()
		return writer.Error()
	case FormatTable:
		if _, err := fmt.Fprintf(w, i18n.T("output.accountInterest.header"), ai.Year, formatFloat(ai.Total)); err != nil {
			return err
		}
		paid := 0
		for _, m := range ai.Monthly {
			for _, p := range m.Payments {
				paid++
				mark := ""
				if p.Estimated {
					// 예상 이자는 아직 안 들어온 돈이다. 실지급액과 같은 열에
					// 그냥 찍으면 확정 수령액으로 읽힌다.
					mark = i18n.T("output.accountInterest.estimatedMark")
				}
				line := fmt.Sprintf(i18n.T("output.accountInterest.row"),
					p.Date, formatFloat(p.PaymentAmount), mark)
				if _, err := fmt.Fprintln(w, strings.TrimRight(line, " \n")); err != nil {
					return err
				}
				if _, err := fmt.Fprintf(w, i18n.T("output.accountInterest.detail"),
					formatFloat(p.Amount), formatFloat(p.Tax), p.StartDate, p.EndDate); err != nil {
					return err
				}
			}
		}
		if paid == 0 {
			if _, err := fmt.Fprint(w, i18n.T("output.accountInterest.empty")); err != nil {
				return err
			}
			if len(ai.AvailableYears) > 0 {
				years := make([]string, len(ai.AvailableYears))
				for i, y := range ai.AvailableYears {
					years[i] = strconv.Itoa(y)
				}
				if _, err := fmt.Fprintf(w, i18n.T("output.accountInterest.availableYears"), strings.Join(years, ", ")); err != nil {
					return err
				}
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}
