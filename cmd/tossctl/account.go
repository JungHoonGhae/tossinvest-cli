package main

import (
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/spf13/cobra"
)

func newAccountCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "account",
		Short: i18n.T("account.short"),
	}

	cmd.AddCommand(
		newAccountDetailCmd(opts),
		newAccountInterestCmd(opts),
		newAccountBuyingPowerCmd(opts),
		newAccountOverviewCmd(opts),
		newAccountTradingSettingsCmd(opts),
		newAccountTransferAccountsCmd(opts),
		newAccountAccessStatusCmd(opts),
		&cobra.Command{
			Use:         "list",
			Short:       i18n.T("account.list.short"),
			Annotations: map[string]string{"source": "both"},
			RunE: func(cmd *cobra.Command, _ []string) error {
				app, err := newAppContext(opts)
				if err != nil {
					return err
				}

				accounts, primaryKey, err := app.client.ListAccounts(cmd.Context())
				if err != nil {
					return userFacingCommandError(err)
				}

				return output.WriteAccounts(cmd.OutOrStdout(), app.format, accounts, primaryKey)
			},
		},
		&cobra.Command{
			Use:         "summary",
			Short:       i18n.T("account.summary.short"),
			Annotations: map[string]string{"source": "wts"},
			RunE: func(cmd *cobra.Command, _ []string) error {
				app, err := newAppContext(opts)
				if err != nil {
					return err
				}

				summary, err := app.client.GetAccountSummary(cmd.Context())
				if err != nil {
					return userFacingCommandError(err)
				}

				return output.WriteAccountSummary(cmd.OutOrStdout(), app.format, summary)
			},
		},
		&cobra.Command{
			Use:         "commission",
			Short:       i18n.T("account.commission.short"),
			Long:        i18n.T("account.commission.long"),
			Annotations: map[string]string{"source": "wts"},
			RunE: func(cmd *cobra.Command, _ []string) error {
				app, err := newAppContext(opts)
				if err != nil {
					return err
				}

				schedule, err := app.client.GetCommissionSchedule(cmd.Context())
				if err != nil {
					return userFacingCommandError(err)
				}

				return output.WriteAccountCommission(cmd.OutOrStdout(), app.format, schedule)
			},
		},
		&cobra.Command{
			Use:         "prime",
			Short:       i18n.T("account.prime.short"),
			Long:        i18n.T("account.prime.long"),
			Annotations: map[string]string{"source": "wts"},
			RunE: func(cmd *cobra.Command, _ []string) error {
				app, err := newAppContext(opts)
				if err != nil {
					return err
				}

				status, err := app.client.GetPrimeStatus(cmd.Context())
				if err != nil {
					return userFacingCommandError(err)
				}

				return output.WriteAccountPrime(cmd.OutOrStdout(), app.format, status)
			},
		},
		newAccountReceivableCmd(opts),
	)

	return cmd
}

func newAccountAccessStatusCmd(opts *rootOptions) *cobra.Command {
	var account string
	cmd := &cobra.Command{
		Use:         "access-status",
		Short:       i18n.T("account.accessStatus.short"),
		Long:        i18n.T("account.accessStatus.long"),
		Args:        cobra.NoArgs,
		Annotations: map[string]string{"source": "wts", "domain": "securities"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			status, err := app.client.GetAccountAccessStatus(cmd.Context(), account)
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteAccountAccessStatus(cmd.OutOrStdout(), app.format, status)
		},
	}
	cmd.Flags().StringVar(&account, "account", "", "Securities account key (default: primary account)")
	return cmd
}

func newAccountTransferAccountsCmd(opts *rootOptions) *cobra.Command {
	var full bool
	var account string
	cmd := &cobra.Command{
		Use:         "transfer-accounts",
		Short:       i18n.T("account.transferAccounts.short"),
		Long:        i18n.T("account.transferAccounts.long"),
		Annotations: map[string]string{"source": "wts", "domain": "securities"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			accounts, err := app.client.GetSecuritiesTransferAccounts(cmd.Context(), account)
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteSecuritiesTransferAccounts(cmd.OutOrStdout(), app.format, accounts, full)
		},
	}
	cmd.Flags().StringVar(&account, "account", "", "Securities account key (default: primary account)")
	cmd.Flags().BoolVar(&full, "full", false, "show complete account numbers (default masks them)")
	return cmd
}

func newAccountTradingSettingsCmd(opts *rootOptions) *cobra.Command {
	var account string
	cmd := &cobra.Command{
		Use:         "trading-settings",
		Short:       i18n.T("account.tradingSettings.short"),
		Long:        i18n.T("account.tradingSettings.long"),
		Annotations: map[string]string{"source": "wts", "domain": "securities"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			settings, err := app.client.GetTradingSettings(cmd.Context(), account)
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteTradingSettings(cmd.OutOrStdout(), app.format, settings)
		},
	}
	cmd.Flags().StringVar(&account, "account", "", "Securities account key (default: primary account)")
	return cmd
}

func newAccountOverviewCmd(opts *rootOptions) *cobra.Command {
	var full bool
	cmd := &cobra.Command{
		Use:         "overview",
		Short:       i18n.T("account.overview.short"),
		Long:        i18n.T("account.overview.long"),
		Annotations: map[string]string{"source": "wts"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			overview, err := app.client.GetAccountOverview(cmd.Context())
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteAccountOverview(cmd.OutOrStdout(), app.format, overview, full)
		},
	}
	cmd.Flags().BoolVar(&full, "full", false, "show account numbers in full (default masks them)")
	return cmd
}

func newAccountReceivableCmd(opts *rootOptions) *cobra.Command {
	var currency string
	cmd := &cobra.Command{
		Use:         "receivable",
		Short:       i18n.T("account.receivable.short"),
		Annotations: map[string]string{"source": "wts"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			n, err := app.client.GetMarginNotice(cmd.Context(), currency)
			if err != nil {
				return err
			}
			return output.WriteMarginNotice(cmd.OutOrStdout(), app.format, n)
		},
	}
	cmd.Flags().StringVar(&currency, "currency", "KRW", "KRW or USD")
	return cmd
}

func newAccountDetailCmd(opts *rootOptions) *cobra.Command {
	var full bool
	cmd := &cobra.Command{
		Use:         "detail",
		Short:       i18n.T("account.detail.short"),
		Long:        i18n.T("account.detail.long"),
		Annotations: map[string]string{"source": "wts"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			d, err := app.client.GetAccountDetail(cmd.Context())
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteAccountDetail(cmd.OutOrStdout(), app.format, d, full)
		},
	}
	cmd.Flags().BoolVar(&full, "full", false, "show the account number in full (default masks it)")
	return cmd
}

func newAccountInterestCmd(opts *rootOptions) *cobra.Command {
	var year int

	cmd := &cobra.Command{
		Use:         "interest",
		Short:       i18n.T("account.interest.short"),
		Long:        i18n.T("account.interest.long"),
		Annotations: map[string]string{"source": "wts"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}

			interest, err := app.client.GetAccountInterest(cmd.Context(), year)
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteAccountInterest(cmd.OutOrStdout(), app.format, interest)
		},
	}

	cmd.Flags().IntVar(&year, "year", 0, i18n.T("account.interest.yearFlag"))
	return cmd
}

// newAccountBuyingPowerCmd exposes the official API's cash buying power.
//
// Its own command rather than a field on `account summary`: the official
// 매수여력 and the WTS summary's orderable amounts are different concepts, and
// merging them would let one be read as the other. Keeping them apart is also
// what makes this usable with an official key and no web session — the summary
// is WTS-only. See issue #136.
func newAccountBuyingPowerCmd(opts *rootOptions) *cobra.Command {
	var currency string
	cmd := &cobra.Command{
		Use:         "buying-power",
		Short:       i18n.T("account.buyingPower.short"),
		Long:        i18n.T("account.buyingPower.long"),
		Annotations: map[string]string{"source": "official"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			bp, err := app.client.BuyingPower(cmd.Context(), currency)
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteBuyingPower(cmd.OutOrStdout(), app.format, bp)
		},
	}
	cmd.Flags().StringVar(&currency, "currency", "KRW", "KRW or USD")
	return cmd
}
