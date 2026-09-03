package output

import (
	"fmt"
	"io"
	"sort"
	"strconv"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/privacy"
)

func WriteOpenBankingStatus(w io.Writer, format Format, status domain.OpenBankingStatus, full bool) error {
	view := status
	if !full {
		view = privacy.RedactOpenBankingStatus(status)
	}
	accountNo, bankCode := "", ""
	if view.ConnectedAccount != nil {
		accountNo = view.ConnectedAccount.AccountNo
		bankCode = view.ConnectedAccount.BankCode
	}
	switch format {
	case FormatJSON:
		return writeJSON(w, view)
	case FormatCSV:
		return writeCSV(w,
			[]string{"holder_name", "connected_account_no", "bank_code", "linked_account_count", "registrable_account_count", "saving_count", "connection_creatable", "registration_required"},
			[][]string{{view.HolderName, accountNo, bankCode, strconv.Itoa(view.LinkedAccountCount), strconv.Itoa(view.RegistrableAccountCount), strconv.Itoa(view.SavingCount), strconv.FormatBool(view.ConnectionCreatable), strconv.FormatBool(view.RegistrationRequired)}},
		)
	case FormatTable:
		state := "not connected"
		if view.ConnectedAccount != nil {
			state = "connected"
		}
		if _, err := fmt.Fprintf(w, "Open banking: %s\n", state); err != nil {
			return err
		}
		if view.HolderName != "" {
			if _, err := fmt.Fprintf(w, "Holder:       %s\n", view.HolderName); err != nil {
				return err
			}
		}
		if view.ConnectedAccount != nil {
			if _, err := fmt.Fprintf(w, "Account:      %s  (bank %s)\n", accountNo, bankCode); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "Linked:       %d\nRegistrable:  %d\nSavings:      %d\n", view.LinkedAccountCount, view.RegistrableAccountCount, view.SavingCount); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "Connection creatable:  %t\nRegistration required: %t\n", view.ConnectionCreatable, view.RegistrationRequired); err != nil {
			return err
		}
		if !full && view.ConnectedAccount != nil {
			_, err := fmt.Fprintln(w, "(use --full to reveal the account holder and number)")
			return err
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func WriteMarketKeyEvents(w io.Writer, format Format, events domain.MarketKeyEvents) error {
	switch format {
	case FormatJSON:
		return writeJSON(w, events)
	case FormatCSV:
		rows := make([][]string, 0, len(events.Earnings)+len(events.Indicators))
		for _, item := range events.Earnings {
			rows = append(rows, []string{
				"earnings", item.AnnounceAt, item.CompanyCode, item.CompanyName,
				optionalFloat(item.EPS), optionalFloat(item.EPSEstimate), "", "EPS", item.MarketStatus,
				optionalString(item.EPSDisplay), optionalString(item.EPSEstimateDisplay), optionalFloat(item.EPSSurprise), optionalString(item.EPSSurpriseDisplay),
				optionalFloat(item.Sales), optionalString(item.SalesDisplay), optionalFloat(item.SalesEstimate), optionalString(item.SalesEstimateDisplay), optionalFloat(item.SalesSurprise), optionalString(item.SalesSurpriseDisplay),
				optionalFloat(item.OperatingProfit), optionalString(item.OperatingProfitDisplay), optionalFloat(item.OperatingEstimate), optionalString(item.OperatingEstimateDisplay), optionalFloat(item.OperatingSurprise), optionalString(item.OperatingSurpriseDisplay),
				item.CountryIcon, item.LogoImageURL, optionalString(item.LegacyReportID), item.ReportID, item.ReportItem, item.LandingURL, "", "",
			})
		}
		for _, item := range events.Indicators {
			rows = append(rows, []string{
				"economic", item.AnnounceAt, item.RIC, item.Title,
				optionalFloat(item.Actual), optionalFloat(item.Forecast), optionalFloat(item.Historical), item.Unit, "",
				"", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "",
				item.UnitPrefix, item.DisplayUnit,
			})
		}
		return writeCSV(w, []string{
			"kind", "announce_at", "code", "title", "actual", "forecast", "historical", "unit", "status",
			"actual_display", "forecast_display", "surprise", "surprise_display",
			"sales", "sales_display", "sales_estimate", "sales_estimate_display", "sales_surprise", "sales_surprise_display",
			"operating_profit", "operating_profit_display", "operating_profit_estimate", "operating_profit_estimate_display", "operating_profit_surprise", "operating_profit_surprise_display",
			"country_icon", "logo_image_url", "legacy_report_id", "report_id", "report_item", "landing_url", "unit_prefix", "display_unit",
		}, rows)
	case FormatTable:
		if _, err := fmt.Fprintln(w, "Earnings"); err != nil {
			return err
		}
		earningRows := make([][]string, 0, len(events.Earnings))
		for _, item := range events.Earnings {
			earningRows = append(earningRows, []string{item.AnnounceAt, item.CompanyName, optionalFloat(item.EPS), optionalFloat(item.EPSEstimate), item.MarketStatusText})
		}
		if err := renderTable(w, []string{"TIME", "COMPANY", "EPS", "EST", "SESSION"}, earningRows); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "\nEconomic indicators"); err != nil {
			return err
		}
		indicatorRows := make([][]string, 0, len(events.Indicators))
		for _, item := range events.Indicators {
			indicatorRows = append(indicatorRows, []string{item.AnnounceAt, item.Title, optionalFloatWithUnit(item.Actual, item.Unit), optionalFloatWithUnit(item.Forecast, item.Unit), optionalFloatWithUnit(item.Historical, item.Unit)})
		}
		return renderTable(w, []string{"TIME", "INDICATOR", "ACTUAL", "FORECAST", "PREVIOUS"}, indicatorRows)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func optionalFloat(value *float64) string {
	if value == nil {
		return "-"
	}
	return formatFloat(*value)
}

func optionalFloatWithUnit(value *float64, unit string) string {
	if value == nil {
		return "-"
	}
	return formatFloat(*value) + unit
}

func WriteNotificationSettings(w io.Writer, format Format, settings domain.NotificationSettings) error {
	view := settings
	view.Settings = append([]domain.NotificationSetting(nil), settings.Settings...)
	sort.SliceStable(view.Settings, func(i, j int) bool { return view.Settings[i].Type < view.Settings[j].Type })
	switch format {
	case FormatJSON:
		return writeJSON(w, view)
	case FormatCSV:
		rows := make([][]string, 0, len(view.Settings))
		for _, item := range view.Settings {
			rows = append(rows, []string{strconv.FormatInt(item.ID, 10), item.Type, strconv.FormatBool(item.Enabled), item.UpdatedAt})
		}
		return writeCSV(w, []string{"id", "type", "enabled", "updated_at"}, rows)
	case FormatTable:
		rows := make([][]string, 0, len(view.Settings))
		for _, item := range view.Settings {
			name := item.Type
			if name == "" {
				name = "(untyped)"
			}
			rows = append(rows, []string{name, strconv.FormatBool(item.Enabled), item.UpdatedAt})
		}
		return renderTable(w, []string{"TYPE", "ENABLED", "UPDATED"}, rows)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}
