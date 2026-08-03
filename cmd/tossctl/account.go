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
	)

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
