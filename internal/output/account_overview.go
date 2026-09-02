package output

import (
	"fmt"
	"io"
	"strconv"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/privacy"
)

// WriteAccountOverview renders the all-account rollup. Account numbers are
// masked in every format unless the caller explicitly opts into --full.
func WriteAccountOverview(w io.Writer, format Format, overview domain.AccountOverview, full bool) error {
	view := overview
	if !full {
		view = privacy.RedactAccountOverview(overview)
	}

	switch format {
	case FormatJSON:
		return writeJSON(w, view)
	case FormatCSV:
		rows := make([][]string, 0, len(view.Accounts)+len(view.MinorAccounts))
		appendRows := func(kind string, items []domain.AccountOverviewItem) {
			for _, item := range items {
				rows = append(rows, []string{
					kind,
					item.AccountName,
					item.AccountNo,
					strconv.Itoa(item.PendingOrderCount),
					strconv.FormatInt(item.TotalAssetAmount, 10),
				})
			}
		}
		appendRows("regular", view.Accounts)
		appendRows("minor", view.MinorAccounts)
		return writeCSV(w, []string{"kind", "account_name", "account_no", "pending_order_count", "total_asset_amount"}, rows)
	case FormatTable:
		if _, err := fmt.Fprintf(w, i18n.T("output.account.overview.total"), formatFloat(float64(view.TotalAssetAmount))); err != nil {
			return err
		}
		if err := writeOverviewGroup(w, i18n.T("output.account.overview.regular"), view.Accounts); err != nil {
			return err
		}
		if len(view.MinorAccounts) > 0 {
			if err := writeOverviewGroup(w, i18n.T("output.account.overview.minor"), view.MinorAccounts); err != nil {
				return err
			}
		}
		if !full {
			_, err := fmt.Fprint(w, i18n.T("output.account.overview.maskHint"))
			return err
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func writeOverviewGroup(w io.Writer, title string, items []domain.AccountOverviewItem) error {
	if _, err := fmt.Fprintf(w, "\n%s\n", title); err != nil {
		return err
	}
	for _, item := range items {
		if _, err := fmt.Fprintf(w, "- %s  %s  %s  pending=%d\n",
			item.AccountName,
			item.AccountNo,
			formatFloat(float64(item.TotalAssetAmount)),
			item.PendingOrderCount,
		); err != nil {
			return err
		}
	}
	return nil
}
