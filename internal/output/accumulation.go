package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// WriteAccumulationPlans renders "stock accumulation" (주식모으기) plans.
// JSON/CSV are language-invariant; the table view shows Active/Paused status
// plainly (the feature IsPaused is inverted for readability).
func WriteAccumulationPlans(w io.Writer, format Format, p domain.AccumulationPlans) error {
	switch format {
	case FormatJSON:
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(p)
	case FormatCSV:
		writer := csv.NewWriter(w)
		header := []string{
			"symbol", "stock_name", "status", "plan_type", "iteration",
			"invest_amount", "invest_quantity", "currency",
			"succeeded_round", "invest_start_date",
		}
		if err := writer.Write(header); err != nil {
			return err
		}
		for _, plan := range p.Plans {
			row := []string{
				plan.Symbol, plan.StockName, accumulationStatus(plan),
				plan.PlanType, plan.Iteration,
				strconv.FormatFloat(plan.InvestAmount, 'f', -1, 64),
				strconv.FormatFloat(plan.InvestQuantity, 'f', -1, 64),
				plan.Currency,
				strconv.Itoa(plan.SucceededRound), plan.InvestStartDate,
			}
			if err := writer.Write(row); err != nil {
				return err
			}
		}
		writer.Flush()
		return writer.Error()
	default: // table
		if len(p.Plans) == 0 {
			_, err := fmt.Fprintln(w, "(no accumulation plans)")
			return err
		}
		for _, plan := range p.Plans {
			amount := plan.InvestQuantity
			unit := "주"
			if plan.PlanType == "AMOUNT" {
				amount = plan.InvestAmount
				unit = plan.Currency
			}
			if _, err := fmt.Fprintf(w, "  %-6s %-18s %-8s %-8s every %-8s %.2f %s (%d회 완료)\n",
				plan.Symbol, plan.StockName, accumulationStatus(plan), plan.PlanType,
				plan.Iteration, amount, unit, plan.SucceededRound,
			); err != nil {
				return err
			}
		}
		return nil
	}
}

func accumulationStatus(p domain.AccumulationPlan) string {
	if p.IsPaused {
		return "Paused"
	}
	return "Active"
}
