package main

import (
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/spf13/cobra"
)

func newProfitCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
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
}
