package main

import (
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/spf13/cobra"
)

func newBankingCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "banking", Short: i18n.T("banking.short")}
	statusCmd := newAccumulationFundingStatusCmd(opts, "status")
	statusCmd.Deprecated = "use `tossctl accumulate funding-status` instead; this is a Securities funding status, not general Toss Banking"
	cmd.AddCommand(statusCmd)
	return cmd
}

func newAccumulationFundingStatusCmd(opts *rootOptions, use string) *cobra.Command {
	var full bool
	statusCmd := &cobra.Command{
		Use:         use,
		Short:       i18n.T("accumulate.funding-status.short"),
		Long:        i18n.T("accumulate.funding-status.long"),
		Args:        cobra.NoArgs,
		Annotations: map[string]string{"source": "wts", "domain": "securities"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			status, err := app.client.GetOpenBankingStatus(cmd.Context())
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteOpenBankingStatus(cmd.OutOrStdout(), app.format, status, full)
		},
	}
	statusCmd.Flags().BoolVar(&full, "full", false, "show the full account holder name and account number")
	return statusCmd
}
