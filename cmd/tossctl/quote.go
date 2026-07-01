package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/spf13/cobra"
)

func newQuoteCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "quote",
		Short: "Read quote data",
	}

	getCmd := &cobra.Command{
		Use:         "get <symbol or name>",
		Short:       "Fetch quote data for a symbol or stock name",
		Args:        cobra.MinimumNArgs(1),
		Annotations: map[string]string{"source": "both"},
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}

			symbol := strings.Join(args, " ")
			quote, err := app.client.GetQuote(cmd.Context(), symbol)
			if err != nil {
				return err
			}

			return output.WriteQuote(cmd.OutOrStdout(), app.format, quote)
		},
	}

	var (
		batchChart    bool
		batchLive     bool
		batchInterval int
	)
	batchCmd := &cobra.Command{
		Use:         "batch <symbol>[,symbol,...] [...]",
		Short:       "Fetch quotes for multiple symbols at once (comma-separated supported)",
		Args:        cobra.MinimumNArgs(1),
		Annotations: map[string]string{"source": "both"},
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}

			symbols := parseBatchSymbols(args)

			fetchAndRender := func(ctx context.Context, w io.Writer) error {
				quotes, err := fetchQuotesConcurrently(ctx, app.client, symbols)
				if err != nil {
					return err
				}

				if !batchChart {
					return output.WriteQuotes(w, app.format, quotes)
				}

				warnW := io.Writer(cmd.ErrOrStderr())
				if batchLive {
					warnW = w
				}
				var charts []domain.Chart
				for _, q := range quotes {
					chart, err := app.client.GetChart(ctx, q.ProductCode, "3m", 30)
					if err != nil {
						fmt.Fprintf(warnW, "warning: chart unavailable for %s: %v\n", q.Symbol, err)
						charts = append(charts, domain.Chart{})
						continue
					}
					charts = append(charts, chart)
				}
				return output.WriteQuotesWithCharts(w, app.format, quotes, charts)
			}

			if !batchLive {
				return fetchAndRender(cmd.Context(), cmd.OutOrStdout())
			}

			if !isTerminal(cmd.OutOrStdout()) {
				return fmt.Errorf("--live requires an interactive terminal")
			}

			if batchInterval < 1 {
				batchInterval = 1
			}

			ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer cancel()

			w := cmd.OutOrStdout()
			fmt.Fprint(w, "\033[?1049h\033[?25l")
			defer fmt.Fprint(w, "\033[?25h\033[?1049l")

			fmt.Fprintf(w, "\033[H\033[JEvery %ds | Fetching...\n", batchInterval)

			var lastGood string
			interval := time.Duration(batchInterval) * time.Second
			for {
				var buf strings.Builder
				fmt.Fprintf(&buf, "Every %ds | %s\n\n",
					batchInterval, time.Now().Local().Format("2006-01-02 15:04:05"))

				if err := fetchAndRender(ctx, &buf); err != nil {
					if ctx.Err() != nil {
						return nil
					}
					fmt.Fprintf(&buf, "%s\n\033[31merror: %v\033[0m\n", lastGood, err)
				} else {
					lastGood = buf.String()
				}

				fmt.Fprint(w, "\033[H"+buf.String()+"\033[J")

				select {
				case <-ctx.Done():
					return nil
				case <-time.After(interval):
				}
			}
		},
	}
	batchCmd.Flags().BoolVar(&batchChart, "chart", false, "show sparkline chart for each symbol")
	batchCmd.Flags().BoolVar(&batchLive, "live", false, "continuously refresh (like watch/viddy)")
	batchCmd.Flags().IntVar(&batchInterval, "interval", 2, "refresh interval in seconds (used with --live)")

	var (
		chartInterval string
		chartCount    int
	)
	chartCmd := &cobra.Command{
		Use:         "chart <symbol or name>",
		Short:       "Fetch candle chart for a symbol or stock name",
		Args:        cobra.MinimumNArgs(1),
		Annotations: map[string]string{"source": "both"},
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}

			symbol := strings.Join(args, " ")
			chart, err := app.client.GetChart(cmd.Context(), symbol, chartInterval, chartCount)
			if err != nil {
				return err
			}

			return output.WriteChart(cmd.OutOrStdout(), app.format, chart)
		},
	}
	chartCmd.Flags().StringVar(&chartInterval, "interval", "3m", "candle interval: 1m, 3m, 5m, 10m, 15m, 30m, 60m")
	chartCmd.Flags().IntVar(&chartCount, "count", 30, "number of candles to fetch")

	var tradesCount int
	tradesCmd := &cobra.Command{
		Use:         "trades <symbol or name>",
		Short:       "Fetch recent executed ticks for a symbol",
		Args:        cobra.MinimumNArgs(1),
		Annotations: map[string]string{"source": "both"},
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			list, err := app.client.GetTrades(cmd.Context(), strings.Join(args, " "), tradesCount)
			if err != nil {
				return err
			}
			return output.WriteTrades(cmd.OutOrStdout(), app.format, list)
		},
	}
	tradesCmd.Flags().IntVar(&tradesCount, "count", 30, "number of recent ticks to fetch")

	limitsCmd := &cobra.Command{
		Use:         "limits <symbol or name>",
		Short:       "Fetch daily upper and lower price band",
		Args:        cobra.MinimumNArgs(1),
		Annotations: map[string]string{"source": "both"},
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			pl, err := app.client.GetPriceLimits(cmd.Context(), strings.Join(args, " "))
			if err != nil {
				return err
			}
			return output.WritePriceLimits(cmd.OutOrStdout(), app.format, pl)
		},
	}

	warningsCmd := &cobra.Command{
		Use:         "warnings <symbol or name>",
		Short:       "Fetch buy-caution badges",
		Args:        cobra.MinimumNArgs(1),
		Annotations: map[string]string{"source": "wts"},
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			sw, err := app.client.GetStockWarnings(cmd.Context(), strings.Join(args, " "))
			if err != nil {
				return err
			}
			return output.WriteStockWarnings(cmd.OutOrStdout(), app.format, sw)
		},
	}

	var flowsSize int
	flowsCmd := &cobra.Command{
		Use:         "flows <symbol or name>",
		Short:       "Investor-type net flows (KR: retail, foreign, institutional)",
		Args:        cobra.MinimumNArgs(1),
		Annotations: map[string]string{"source": "wts"},
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			tf, err := app.client.GetTradingFlows(cmd.Context(), strings.Join(args, " "), flowsSize)
			if err != nil {
				return err
			}
			return output.WriteTradingFlows(cmd.OutOrStdout(), app.format, tf)
		},
	}
	flowsCmd.Flags().IntVar(&flowsSize, "size", 20, "number of recent days")

	orderbookCmd := &cobra.Command{
		Use:         "orderbook <symbol or name>",
		Short:       "Bid/ask depth ladder",
		Args:        cobra.MinimumNArgs(1),
		Annotations: map[string]string{"source": "both"},
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			ob, err := app.client.GetOrderBook(cmd.Context(), strings.Join(args, " "))
			if err != nil {
				return err
			}
			return output.WriteOrderBook(cmd.OutOrStdout(), app.format, ob)
		},
	}

	sellableCmd := &cobra.Command{
		Use:         "sellable <symbol or name>",
		Short:       "Sellable quantity for a held symbol",
		Args:        cobra.MinimumNArgs(1),
		Annotations: map[string]string{"source": "both"},
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			sq, err := app.client.GetSellableQuantity(cmd.Context(), strings.Join(args, " "))
			if err != nil {
				return err
			}
			return output.WriteSellableQuantity(cmd.OutOrStdout(), app.format, sq)
		},
	}

	commissionCmd := &cobra.Command{
		Use:         "commission <symbol or name>",
		Short:       "Commission and tax rate",
		Args:        cobra.MinimumNArgs(1),
		Annotations: map[string]string{"source": "both"},
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			c, err := app.client.GetCommission(cmd.Context(), strings.Join(args, " "))
			if err != nil {
				return err
			}
			return output.WriteCommission(cmd.OutOrStdout(), app.format, c)
		},
	}

	cmd.AddCommand(getCmd, batchCmd, chartCmd, tradesCmd, limitsCmd, warningsCmd, flowsCmd, orderbookCmd, sellableCmd, commissionCmd)

	return cmd
}

func parseBatchSymbols(args []string) []string {
	var symbols []string
	for _, arg := range args {
		for _, s := range strings.Split(arg, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				symbols = append(symbols, s)
			}
		}
	}
	return symbols
}

type quoteFetcher interface {
	GetQuote(ctx context.Context, symbol string) (domain.Quote, error)
}

// maxConcurrentQuotes bounds how many symbols are fetched at once in batch mode.
// Each GetQuote fans out into several Toss HTTP calls, so a sequential loop over
// many symbols is slow — especially under `--live` where it re-runs on a timer.
// Modest fan-out keeps it polite to the upstream.
const maxConcurrentQuotes = 6

// fetchQuotesConcurrently fetches every symbol in parallel (bounded) and returns
// the quotes in the original symbol order. It is fail-fast: if any symbol
// errors, the first error by symbol order is returned (wrapped with the symbol).
func fetchQuotesConcurrently(ctx context.Context, c quoteFetcher, symbols []string) ([]domain.Quote, error) {
	quotes := make([]domain.Quote, len(symbols))
	errs := make([]error, len(symbols))

	sem := make(chan struct{}, maxConcurrentQuotes)
	var wg sync.WaitGroup
	for i, symbol := range symbols {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, symbol string) {
			defer wg.Done()
			defer func() { <-sem }()
			quotes[i], errs[i] = c.GetQuote(ctx, symbol)
		}(i, symbol)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			return nil, fmt.Errorf("%s: %w", symbols[i], err)
		}
	}
	return quotes, nil
}
