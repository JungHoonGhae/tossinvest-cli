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
