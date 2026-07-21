package main

import (
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/spf13/cobra"
)

func newTaxCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tax",
		Short: i18n.T("tax.short"),
	}

	var year int
	overseasCmd := &cobra.Command{
		Use:         "overseas",
		Short:       i18n.T("tax.overseas.short"),
		Long:        i18n.T("tax.overseas.long"),
		Annotations: map[string]string{"source": "wts"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			if year == 0 {
				year = time.Now().Year()
			}
			data, err := app.client.GetOverseasTransferIncome(cmd.Context(), year)
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteOverseasTransferIncome(cmd.OutOrStdout(), app.format, data)
		},
	}
	overseasCmd.Flags().IntVar(&year, "year", 0, "Tax year (default: current year)")
	cmd.AddCommand(overseasCmd)

	return cmd
}
