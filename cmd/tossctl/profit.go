package main

import (
	"fmt"
	"strings"

	tossclient "github.com/JungHoonGhae/tossinvest-cli/internal/client"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/spf13/cobra"
)

func newProfitCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "profit",
		Short:       i18n.T("profit.short"),
		Long:        i18n.T("profit.long"),
		Annotations: map[string]string{"source": "wts"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			data, err := app.client.GetProfitOverview(cmd.Context())
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteProfitOverview(cmd.OutOrStdout(), app.format, data)
		},
	}

	cmd.AddCommand(newProfitSummaryCmd(opts), newProfitDailyCmd(opts))
	return cmd
}

func newProfitSummaryCmd(opts *rootOptions) *cobra.Command {
	var profitType, from, to string

	cmd := &cobra.Command{
		Use:         "summary",
		Short:       i18n.T("profit.summary.short"),
		Long:        i18n.T("profit.summary.long"),
		Annotations: map[string]string{"source": "wts"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Validated locally: the server returns 400 for an unknown type, and
			// the set is small and stable enough to name in the error.
			if !containsString(tossclient.ProfitTypes, profitType) {
				return fmt.Errorf("--type: %q 는 지원되지 않습니다 — %s 중 하나여야 합니다",
					profitType, strings.Join(tossclient.ProfitTypes, ", "))
			}
			f, t, err := tossclient.ParseProfitRange(from, to)
			if err != nil {
				return err
			}
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			data, err := app.client.GetPeriodProfit(cmd.Context(), profitType, f, t)
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WritePeriodProfit(cmd.OutOrStdout(), app.format, data)
		},
	}

	cmd.Flags().StringVar(&profitType, "type", "sales",
		"profit category: "+strings.Join(tossclient.ProfitTypes, "|"))
	cmd.Flags().StringVar(&from, "from", "", "start date (YYYY-MM-DD); omit for all time")
	cmd.Flags().StringVar(&to, "to", "", "end date (YYYY-MM-DD); omit for all time")
	return cmd
}

func newProfitDailyCmd(opts *rootOptions) *cobra.Command {
	var currency, from, to string

	cmd := &cobra.Command{
		Use:         "daily",
		Short:       i18n.T("profit.daily.short"),
		Long:        i18n.T("profit.daily.long"),
		Annotations: map[string]string{"source": "wts"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// --currency picks the basis the rates are computed against, not a
			// filter: both values return the same rows.
			basis := tossclient.DefaultProfitCurrency
			if currency != "" {
				basis = strings.ToUpper(currency)
				if !containsString(tossclient.ProfitCurrencies, basis) {
					return fmt.Errorf("--currency: %q 는 지원되지 않습니다 — %s 중 하나여야 합니다",
						currency, strings.Join(tossclient.ProfitCurrencies, ", "))
				}
			}
			f, t, err := tossclient.ParseProfitRange(from, to)
			if err != nil {
				return err
			}
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			data, err := app.client.GetDailyProfit(cmd.Context(), f, t, basis)
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteDailyProfit(cmd.OutOrStdout(), app.format, data)
		},
	}

	cmd.Flags().StringVar(&currency, "currency", "",
		"basis for the return rate ("+strings.Join(tossclient.ProfitCurrencies, "|")+
			"), not a filter; default "+tossclient.DefaultProfitCurrency)
	cmd.Flags().StringVar(&from, "from", "", "start date (YYYY-MM-DD); omit for all time")
	cmd.Flags().StringVar(&to, "to", "", "end date (YYYY-MM-DD); omit for all time")
	return cmd
}

func containsString(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}
