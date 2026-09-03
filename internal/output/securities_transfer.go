package output

import (
	"fmt"
	"io"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/privacy"
)

func WriteSecuritiesTransferAccounts(w io.Writer, format Format, accounts domain.SecuritiesTransferAccounts, full bool) error {
	view := accounts
	if !full {
		view = privacy.RedactSecuritiesTransferAccounts(accounts)
	}

	rows := make([][]string, 0, len(view.OwnAccounts)+len(view.RecentAccounts))
	for _, item := range view.OwnAccounts {
		rows = append(rows, []string{"own", item.BankCode, item.AccountNo, item.AccountID})
	}
	for _, item := range view.RecentAccounts {
		rows = append(rows, []string{"recent", item.BankCode, item.AccountNo, item.AccountID})
	}

	switch format {
	case FormatJSON:
		return writeJSON(w, view)
	case FormatCSV:
		return writeCSV(w, []string{"kind", "bank_code", "account_no", "account_id"}, rows)
	case FormatTable:
		if err := renderTable(w, []string{"KIND", "BANK", "ACCOUNT", "ACCOUNT ID"}, rows); err != nil {
			return err
		}
		if !full && len(rows) > 0 {
			_, err := fmt.Fprintln(w, "(use --full to reveal complete account numbers)")
			return err
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}
