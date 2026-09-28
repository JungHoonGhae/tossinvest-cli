package main

import (
	"fmt"
	"strconv"

	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/spf13/cobra"
)

func positiveResearchID(raw, kind string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("%s id must be a positive integer", kind)
	}
	return id, nil
}

func newEarningContentCmd(opts *rootOptions, report bool) *cobra.Command {
	use, short := "transcript <event-id>", i18n.T("market.earnings.transcript.short")
	if report {
		use, short = "report <event-id>", i18n.T("market.earnings.report.short")
	}
	return &cobra.Command{Use: use, Short: short, Args: cobra.ExactArgs(1), Annotations: map[string]string{"source": "wts", "domain": "securities"}, RunE: func(cmd *cobra.Command, args []string) error {
		id, err := positiveResearchID(args[0], "event")
		if err != nil {
			return err
		}
		if cmd.Flags().Changed("major") {
			return fmt.Errorf("--major cannot be used with earnings content")
		}
		app, err := newAppContext(opts)
		if err != nil {
			return err
		}
		if report {
			data, err := app.client.GetEarningReport(cmd.Context(), id)
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteEarningReport(cmd.OutOrStdout(), app.format, data)
		}
		data, err := app.client.GetEarningTranscript(cmd.Context(), id)
		if err != nil {
			return userFacingCommandError(err)
		}
		return output.WriteEarningTranscript(cmd.OutOrStdout(), app.format, data)
	}}
}

func newWatchlistNewsCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "news <folder-id>", Short: i18n.T("watchlist.news.short"), Args: cobra.ExactArgs(1), Annotations: map[string]string{"source": "wts", "domain": "securities"}, RunE: func(cmd *cobra.Command, args []string) error {
		id, err := positiveResearchID(args[0], "folder")
		if err != nil {
			return err
		}
		app, err := newAppContext(opts)
		if err != nil {
			return err
		}
		data, err := app.client.GetWatchlistNews(cmd.Context(), id)
		if err != nil {
			return userFacingCommandError(err)
		}
		return output.WriteWatchlistNews(cmd.OutOrStdout(), app.format, data)
	}}
}
