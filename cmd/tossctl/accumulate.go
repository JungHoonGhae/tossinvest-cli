package main

import (
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/spf13/cobra"
)

func newAccumulateCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "accumulate",
		Short: i18n.T("accumulate.short"),
	}
	cmd.AddCommand(newAccumulationFundingStatusCmd(opts, "funding-status"))

	cmd.AddCommand(&cobra.Command{
		Use:         "list",
		Short:       i18n.T("accumulate.list.short"),
		Long:        i18n.T("accumulate.list.long"),
		Annotations: map[string]string{"source": "wts"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			plans, err := app.client.ListAccumulationPlans(cmd.Context())
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteAccumulationPlans(cmd.OutOrStdout(), app.format, plans)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:         "status <symbol>",
		Short:       i18n.T("accumulate.status.short"),
		Long:        i18n.T("accumulate.status.long"),
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"source": "wts"},
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			plans, err := app.client.GetAccumulationPlansByStock(cmd.Context(), args[0])
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteAccumulationPlans(cmd.OutOrStdout(), app.format, plans)
		},
	})

	return cmd
}
