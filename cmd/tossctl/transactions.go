package main

import (
	"fmt"
	"strings"
	"time"

	tossclient "github.com/JungHoonGhae/tossinvest-cli/internal/client"
	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/spf13/cobra"
)

func newTransactionsCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transactions",
		Short: "Read transaction ledger (trades, cash flow, stock in/out)",
		Long: "Read-only access to Toss Securities' transaction ledger. " +
			"Covers trades, deposits/withdrawals, dividends, and stock in/out per market. " +
			"Toss caps a single query at 200 days and paginates.",
	}

	cmd.AddCommand(newTransactionsListCmd(opts))
	cmd.AddCommand(newTransactionsOverviewCmd(opts))
	return cmd
}

type transactionsListFlags struct {
	market    string
	fromStr   string
	toStr     string
	filter    string
	size      int
	number    int
	all       bool
	pageLimit int
}

func newTransactionsListCmd(opts *rootOptions) *cobra.Command {
	flags := &transactionsListFlags{}
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List transactions (trades + cash flow) for a market",
		Annotations: map[string]string{"source": "wts"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}

			to, err := parseTransactionDate(flags.toStr, time.Now().In(tossclient.KoreaLocation))
			if err != nil {
				return fmt.Errorf("invalid --to: %w", err)
			}
			from, err := parseTransactionDate(flags.fromStr, to.AddDate(0, 0, -30))
			if err != nil {
				return fmt.Errorf("invalid --from: %w", err)
			}

			ctx := cmd.Context()
			var items []domain.Transaction
			if flags.all {
				items, err = app.client.ListAllTransactions(ctx, flags.market, from, to, flags.filter, flags.size, flags.pageLimit)
				if err != nil {
					return userFacingCommandError(err)
				}
			} else {
				page, err := app.client.ListTransactions(ctx, flags.market, from, to, flags.filter, flags.size, flags.number)
				if err != nil {
					return userFacingCommandError(err)
				}
				items = page.Items
				if !page.LastPage && page.Next != nil && app.format == output.FormatTable {
					fmt.Fprintf(cmd.ErrOrStderr(),
						"(more pages available; pass --all or --number=%d to fetch next)\n",
						page.Next.Number,
					)
				}
			}

			return output.WriteTransactions(cmd.OutOrStdout(), app.format, items)
		},
	}

	cmd.Flags().StringVar(&flags.market, "market", "kr", "Market: kr or us")
	cmd.Flags().StringVar(&flags.fromStr, "from", "", "Start date YYYY-MM-DD (default: to - 30 days)")
	cmd.Flags().StringVar(&flags.toStr, "to", "", "End date YYYY-MM-DD (default: today)")
	cmd.Flags().StringVar(&flags.filter, "filter", "all", "Filter: all|trade|cash|inout|cash-alt or 0|1|2|3|6")
	cmd.Flags().IntVar(&flags.size, "size", tossclient.TransactionsDefaultPageSize, "Page size")
	cmd.Flags().IntVar(&flags.number, "number", 0, "Page index (0-based); ignored when --all")
	cmd.Flags().BoolVar(&flags.all, "all", false, "Page through automatically until last page")
	cmd.Flags().IntVar(&flags.pageLimit, "page-limit", tossclient.TransactionsDefaultPageLimit, "Max pages when --all is set")
	return cmd
}

func newTransactionsOverviewCmd(opts *rootOptions) *cobra.Command {
	var market string
	cmd := &cobra.Command{
		Use:         "overview",
		Short:       "Show cash overview for a market (orderable, withdrawable, upcoming deposits)",
		Annotations: map[string]string{"source": "wts"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			overview, err := app.client.GetTransactionsOverview(cmd.Context(), market)
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteTransactionsOverview(cmd.OutOrStdout(), app.format, overview)
		},
	}
	cmd.Flags().StringVar(&market, "market", "kr", "Market: kr or us")
	return cmd
}

func parseTransactionDate(value string, fallback time.Time) (time.Time, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		return fallback, nil
	}
	return time.ParseInLocation("2006-01-02", v, tossclient.KoreaLocation)
}
