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
		rows = append(rows, []string{view.AccountScope, "own", item.BankCode, item.AccountNo})
	}
	for _, item := range view.RecentAccounts {
		rows = append(rows, []string{view.AccountScope, "recent", item.BankCode, item.AccountNo})
	}

	switch format {
	case FormatJSON:
		return writeJSON(w, view)
	case FormatCSV:
		return writeCSV(w, []string{"account_scope", "kind", "bank_code", "account_no"}, rows)
	case FormatTable:
		if len(rows) == 0 {
			_, err := fmt.Fprintf(w, "Account scope: %s\n(no transfer accounts)\n", view.AccountScope)
			return err
		}
		if err := renderTable(w, []string{"ACCOUNT SCOPE", "KIND", "BANK", "ACCOUNT"}, rows); err != nil {
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
