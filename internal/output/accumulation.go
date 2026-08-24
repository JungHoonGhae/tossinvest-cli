package output

import (
	"encoding/csv"
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
		return writeJSON(w, p)
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
		headers := []string{"SYMBOL", "NAME", "STATUS", "TYPE", "SCHEDULE", "AMOUNT", "ROUND"}
		aligns := []Align{AlignLeft, AlignLeft, AlignLeft, AlignLeft, AlignLeft, AlignRight, AlignRight}
		var rows [][]string
		for _, plan := range p.Plans {
			amount := plan.InvestQuantity
			unit := "주"
			if plan.PlanType == "AMOUNT" {
				amount = plan.InvestAmount
				unit = plan.Currency
			}
			rows = append(rows, []string{
				plan.Symbol,
				plan.StockName,
				accumulationStatus(plan),
				plan.PlanType,
				"every " + plan.Iteration,
				fmt.Sprintf("%.2f %s", amount, unit),
				fmt.Sprintf("%d회", plan.SucceededRound),
			})
		}
		return renderTable(w, headers, rows, aligns...)
	}
}

func accumulationStatus(p domain.AccumulationPlan) string {
	if p.IsPaused {
		return "Paused"
	}
	return "Active"
}
