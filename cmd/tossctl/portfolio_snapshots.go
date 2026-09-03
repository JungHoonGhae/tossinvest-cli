package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/spf13/cobra"
)

func newPortfolioPerformanceCmd(opts *rootOptions) *cobra.Command {
	var account string
	cmd := &cobra.Command{
		Use:         "performance",
		Short:       i18n.T("portfolio.performance.short"),
		Long:        i18n.T("portfolio.performance.long"),
		Annotations: map[string]string{"source": "wts", "domain": "securities"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			performance, err := app.client.GetAssetPerformance(cmd.Context(), account)
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteAssetPerformance(cmd.OutOrStdout(), app.format, performance)
		},
	}
	cmd.Flags().StringVar(&account, "account", "", "specific Securities account key (default: all accounts)")
	return cmd
}

func newPortfolioSnapshotsCmd(opts *rootOptions) *cobra.Command {
	var account, cursor string
	var limit int
	cmd := &cobra.Command{
		Use:         "snapshots",
		Short:       i18n.T("portfolio.snapshots.short"),
		Long:        i18n.T("portfolio.snapshots.long"),
		Annotations: map[string]string{"source": "wts", "domain": "securities"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if limit < 0 {
				return fmt.Errorf("--limit must be zero or greater")
			}
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			page, err := app.client.ListAssetSnapshots(cmd.Context(), account, cursor, limit)
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteAssetSnapshots(cmd.OutOrStdout(), app.format, page)
		},
	}
	cmd.Flags().StringVar(&account, "account", "", "specific Securities account key (default: all accounts)")
	cmd.Flags().StringVar(&cursor, "cursor", "", "pagination cursor from the previous next_cursor")
	cmd.Flags().IntVar(&limit, "limit", 20, "history rows per page (the current realtime point can be additional)")
	return cmd
}

func newPortfolioSnapshotCmd(opts *rootOptions) *cobra.Command {
	var account string
	cmd := &cobra.Command{
		Use:         "snapshot <date>",
		Short:       i18n.T("portfolio.snapshot.short"),
		Long:        i18n.T("portfolio.snapshot.long"),
		Annotations: map[string]string{"source": "wts", "domain": "securities"},
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			date := strings.TrimSpace(args[0])
			if _, err := time.Parse("2006-01-02", date); err != nil {
				return fmt.Errorf("date must be YYYY-MM-DD: %w", err)
			}
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			detail, err := app.client.GetAssetSnapshot(cmd.Context(), account, date)
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteAssetSnapshot(cmd.OutOrStdout(), app.format, detail)
		},
	}
	cmd.Flags().StringVar(&account, "account", "", "specific Securities account key (default: all accounts)")
	return cmd
}
