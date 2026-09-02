package main

import (
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/spf13/cobra"
)

func newMarketKeyEventsCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:         "key-events",
		Short:       i18n.T("market.keyEvents.short"),
		Long:        i18n.T("market.keyEvents.long"),
		Args:        cobra.NoArgs,
		Annotations: map[string]string{"source": "wts"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			events, err := app.client.GetMarketKeyEvents(cmd.Context())
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteMarketKeyEvents(cmd.OutOrStdout(), app.format, events)
		},
	}
}
