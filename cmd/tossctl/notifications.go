package main

import (
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/spf13/cobra"
)

func newNotificationsCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "notifications", Short: i18n.T("notifications.short")}
	cmd.AddCommand(&cobra.Command{
		Use:         "list",
		Short:       i18n.T("notifications.list.short"),
		Long:        i18n.T("notifications.list.long"),
		Args:        cobra.NoArgs,
		Annotations: map[string]string{"source": "wts", "domain": "securities"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			settings, err := app.client.GetNotificationSettings(cmd.Context())
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteNotificationSettings(cmd.OutOrStdout(), app.format, settings)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:         "status",
		Short:       i18n.T("notifications.status.short"),
		Long:        i18n.T("notifications.status.long"),
		Args:        cobra.NoArgs,
		Annotations: map[string]string{"source": "wts", "domain": "securities"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			status, err := app.client.GetNotificationStatus(cmd.Context())
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteNotificationStatus(cmd.OutOrStdout(), app.format, status)
		},
	})
	return cmd
}
