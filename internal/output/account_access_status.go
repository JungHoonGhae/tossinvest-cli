package output

import (
	"fmt"
	"io"
	"strconv"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// WriteAccountAccessStatus renders verified account access and risk signals.
func WriteAccountAccessStatus(w io.Writer, format Format, status domain.AccountAccessStatus) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, status)
	case FormatCSV:
		return writeCSV(w,
			[]string{"account_scope", "last_login_channel", "last_login_os", "last_login_agent", "last_login_at", "margin_frozen", "margin_freeze_start", "margin_freeze_end", "accident_account_count"},
			[][]string{{
				status.AccountScope,
				status.LastLogin.Channel,
				status.LastLogin.OSName,
				status.LastLogin.AgentName,
				status.LastLogin.Timestamp,
				strconv.FormatBool(status.Margin.Frozen),
				status.Margin.StartDate,
				status.Margin.EndDate,
				strconv.Itoa(status.AccidentAccountCount),
			}},
		)
	case FormatTable:
		if _, err := fmt.Fprintf(w, "Account scope: %s\n", status.AccountScope); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "Last login:    %s %s %s  %s\n", status.LastLogin.Channel, status.LastLogin.OSName, status.LastLogin.AgentName, status.LastLogin.Timestamp); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "Margin frozen: %t", status.Margin.Frozen); err != nil {
			return err
		}
		if status.Margin.StartDate != "" || status.Margin.EndDate != "" {
			if _, err := fmt.Fprintf(w, "  (%s - %s)", status.Margin.StartDate, status.Margin.EndDate); err != nil {
				return err
			}
		}
		_, err := fmt.Fprintf(w, "\nAccident account count: %d\n", status.AccidentAccountCount)
		return err
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}
