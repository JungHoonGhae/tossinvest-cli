package main

import (
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/spf13/cobra"
)

func newAccountCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "account",
		Short: "Read account-level data",
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:         "list",
			Short:       "List available accounts",
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
			Short:       "Show a summary for the selected account",
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
	)

	return cmd
}
