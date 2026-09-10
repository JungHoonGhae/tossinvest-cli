package main

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/JungHoonGhae/tossinvest-cli/internal/briefing"
	"github.com/JungHoonGhae/tossinvest-cli/internal/config"
	"github.com/JungHoonGhae/tossinvest-cli/internal/history"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/spf13/cobra"
)

func historyPath(opts *rootOptions) (string, error) {
	dir := opts.configDir
	if dir == "" {
		paths, err := config.DefaultPaths()
		if err != nil {
			return "", err
		}
		dir = paths.ConfigDir
	}
	return filepath.Join(dir, "history.sqlite"), nil
}

func historyStore(opts *rootOptions) (history.Store, error) {
	path, err := historyPath(opts)
	return history.Store{Path: path}, err
}

func newHistoryCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "history", Short: "Collect and explore local portfolio and transaction history", Long: "Explicit WTS observations stored in a private SQLite database. Sync previews a local append; execute it with the preview token. List, positions, transactions, search, and compare work offline without credentials. Each collection retains its date range, freshness, and completeness. Use a separate --config-dir for a different account scope."}
	cmd.AddCommand(newHistorySyncCmd(opts), newHistoryListCmd(opts), newHistoryPositionsCmd(opts), newHistoryTransactionsCmd(opts, false), newHistoryTransactionsCmd(opts, true), newHistoryCompareCmd(opts))
	return cmd
}

func newHistorySyncCmd(opts *rootOptions) *cobra.Command {
	var intent history.SyncOptions
	var execute bool
	var confirm string
	cmd := &cobra.Command{Use: "sync", Short: "Preview or confirm a local WTS history collection", Args: cobra.NoArgs, Annotations: map[string]string{"source": "wts", "writes_state": "true", "mutation_risk": "preference", "reversibility": "reversible"}, RunE: func(cmd *cobra.Command, _ []string) error {
		app, err := newAppContext(opts)
		if err != nil {
			return err
		}
		path, err := historyPath(opts)
		if err != nil {
			return err
		}
		result, err := history.New(path, app.client.Client).Sync(cmd.Context(), intent, execute, confirm)
		if err != nil {
			return userFacingCommandError(err)
		}
		return output.WriteHistorySync(cmd.OutOrStdout(), app.format, result)
	}}
	cmd.Flags().StringVar(&intent.From, "from", "", "Transaction start date YYYY-MM-DD (default: to minus 30 days)")
	cmd.Flags().StringVar(&intent.To, "to", "", "Transaction end date YYYY-MM-DD (default: today in Korea)")
	cmd.Flags().StringVar(&intent.Market, "market", "all", "Transaction markets: all|kr|us; holdings always include all session positions")
	cmd.Flags().IntVar(&intent.PageLimit, "page-limit", 20, "Maximum transaction pages per market (1..200); incomplete collections are labeled")
	cmd.Flags().BoolVar(&execute, "execute", false, "Append to the local history database using the preview confirmation")
	cmd.Flags().StringVar(&confirm, "confirm", "", "Fresh confirm_token returned by this exact sync preview")
	return cmd
}

func newHistoryListCmd(opts *rootOptions) *cobra.Command {
	var limit int
	cmd := &cobra.Command{Use: "list", Short: "List local snapshots and their collection coverage", Args: cobra.NoArgs, Annotations: map[string]string{"source": "local"}, RunE: func(cmd *cobra.Command, _ []string) error {
		store, err := historyStore(opts)
		if err != nil {
			return err
		}
		items, err := store.List(cmd.Context(), limit)
		if err != nil {
			return err
		}
		return output.WriteHistoryList(cmd.OutOrStdout(), output.Format(opts.outputFormat), items)
	}}
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum snapshots, newest first (1..1000)")
	return cmd
}

func newHistoryPositionsCmd(opts *rootOptions) *cobra.Command {
	var id int64
	cmd := &cobra.Command{Use: "positions", Short: "Read holdings from a local snapshot", Args: cobra.NoArgs, Annotations: map[string]string{"source": "local"}, RunE: func(cmd *cobra.Command, _ []string) error {
		if id < 0 {
			return fmt.Errorf("snapshot must be non-negative")
		}
		store, err := historyStore(opts)
		if err != nil {
			return err
		}
		data, err := store.Positions(cmd.Context(), id)
		if err != nil {
			return err
		}
		return output.WriteHistoryPositions(cmd.OutOrStdout(), output.Format(opts.outputFormat), data)
	}}
	cmd.Flags().Int64Var(&id, "snapshot", 0, "Snapshot ID from history list (default: latest)")
	return cmd
}

func newHistoryTransactionsCmd(opts *rootOptions, search bool) *cobra.Command {
	var q history.Query
	cmd := &cobra.Command{Use: "transactions", Short: "Read transactions from a local snapshot", Args: cobra.NoArgs, Annotations: map[string]string{"source": "local"}, RunE: func(cmd *cobra.Command, args []string) error {
		if search {
			q.Search = args[0]
		}
		store, err := historyStore(opts)
		if err != nil {
			return err
		}
		data, err := store.Transactions(cmd.Context(), q)
		if err != nil {
			return err
		}
		return output.WriteHistoryTransactions(cmd.OutOrStdout(), output.Format(opts.outputFormat), data)
	}}
	if search {
		cmd.Use = "search <query>"
		cmd.Short = "Search locally stored transaction names, symbols, and summaries"
		cmd.Args = cobra.ExactArgs(1)
	} else {
		cmd.Flags().StringVar(&q.Search, "query", "", "Literal substring in symbol, name, category, or summary")
	}
	cmd.Flags().Int64Var(&q.SnapshotID, "snapshot", 0, "Snapshot ID from history list (default: latest; collections are not merged)")
	cmd.Flags().StringVar(&q.From, "from", "", "Start date YYYY-MM-DD within the selected collection")
	cmd.Flags().StringVar(&q.To, "to", "", "End date YYYY-MM-DD within the selected collection")
	cmd.Flags().StringVar(&q.Market, "market", "", "Filter market: kr|us")
	cmd.Flags().IntVar(&q.Limit, "limit", 50, "Maximum rows (1..1000)")
	cmd.Flags().IntVar(&q.Offset, "offset", 0, "Row offset; use next_offset from the previous result")
	return cmd
}

func newHistoryCompareCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "compare <before-id> <after-id>", Short: "Compare holdings in two local snapshots", Args: cobra.ExactArgs(2), Annotations: map[string]string{"source": "local"}, RunE: func(cmd *cobra.Command, args []string) error {
		before, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("before-id must be a snapshot ID")
		}
		after, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return fmt.Errorf("after-id must be a snapshot ID")
		}
		store, err := historyStore(opts)
		if err != nil {
			return err
		}
		data, err := store.Compare(cmd.Context(), before, after)
		if err != nil {
			return err
		}
		return output.WriteHistoryComparison(cmd.OutOrStdout(), output.Format(opts.outputFormat), data)
	}}
}

func newPortfolioBriefingCmd(opts *rootOptions) *cobra.Command {
	var limit int
	cmd := &cobra.Command{Use: "briefing", Short: "Combine holdings, related news, earnings, and pending orders", Args: cobra.NoArgs, Annotations: map[string]string{"source": "wts"}, RunE: func(cmd *cobra.Command, _ []string) error {
		app, err := newAppContext(opts)
		if err != nil {
			return err
		}
		data, err := briefing.Collect(cmd.Context(), app.client.Client, limit)
		if err != nil {
			return userFacingCommandError(err)
		}
		return output.WritePortfolioBriefing(cmd.OutOrStdout(), app.format, data)
	}}
	cmd.Flags().IntVar(&limit, "news-limit", 10, "Newest holdings-news articles (1..50); failures are reported per section")
	return cmd
}
