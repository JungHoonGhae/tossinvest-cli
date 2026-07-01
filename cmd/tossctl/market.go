package main

import (
	"fmt"
	"strconv"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/spf13/cobra"
)

// findSector returns the sub-sectors of the sector with the given id, searching
// the full tree. The returned slice is the matched sector's children.
func findSector(items []domain.Sector, id int) ([]domain.Sector, bool) {
	for _, s := range items {
		if s.ID == id {
			return s.SubSectors, true
		}
		if sub, found := findSector(s.SubSectors, id); found {
			return sub, true
		}
	}
	return nil, false
}

func newMarketCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "market",
		Short: "Market-wide information (trading hours, etc.)",
	}

	hoursCmd := &cobra.Command{
		Use:         "hours",
		Short:       "Today's KR and US trading session windows",
		Annotations: map[string]string{"source": "wts"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			th, err := app.client.GetTradingHours(cmd.Context())
			if err != nil {
				return err
			}
			return output.WriteTradingHours(cmd.OutOrStdout(), app.format, th)
		},
	}

	fxCmd := &cobra.Command{
		Use:         "fx",
		Short:       "USD/KRW FX and dollar index quotes",
		Annotations: map[string]string{"source": "both"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			er, err := app.client.GetExchangeRates(cmd.Context())
			if err != nil {
				return err
			}
			return output.WriteExchangeRates(cmd.OutOrStdout(), app.format, er)
		},
	}

	indexCmd := &cobra.Command{
		Use:         "index [code|name]",
		Short:       "Major market indices (KOSPI, KOSDAQ, Nasdaq, S&P 500, VIX)",
		Long:        "Major market indices. Pass a symbol to get index detail (OHLC, 52-week range).",
		Args:        cobra.MaximumNArgs(1),
		Annotations: map[string]string{"source": "wts"},
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			if len(args) == 1 {
				q, err := app.client.GetIndexDetail(cmd.Context(), args[0])
				if err != nil {
					return err
				}
				return output.WriteIndexQuote(cmd.OutOrStdout(), app.format, q)
			}
			mi, err := app.client.GetMarketIndices(cmd.Context())
			if err != nil {
				return err
			}
			return output.WriteMarketIndices(cmd.OutOrStdout(), app.format, mi)
		},
	}

	var rankingSize int
	rankingCmd := &cobra.Command{
		Use:         "ranking",
		Short:       "Realtime popularity ranking",
		Annotations: map[string]string{"source": "wts"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			sr, err := app.client.GetStockRanking(cmd.Context(), rankingSize)
			if err != nil {
				return err
			}
			return output.WriteStockRanking(cmd.OutOrStdout(), app.format, sr)
		},
	}
	rankingCmd.Flags().IntVar(&rankingSize, "size", 20, "number of ranked stocks")

	signalsCmd := &cobra.Command{
		Use:         "signals",
		Short:       "Toss AI market signals",
		Annotations: map[string]string{"source": "wts"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			sg, err := app.client.GetAISignals(cmd.Context())
			if err != nil {
				return err
			}
			return output.WriteAISignals(cmd.OutOrStdout(), app.format, sg)
		},
	}

	var investorsSize int
	investorsCmd := &cobra.Command{
		Use:         "investors",
		Short:       "Net-buy ranking by investor type (foreign, institutional, retail)",
		Annotations: map[string]string{"source": "wts"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			ir, err := app.client.GetInvestorRankings(cmd.Context(), investorsSize)
			if err != nil {
				return err
			}
			return output.WriteInvestorRankings(cmd.OutOrStdout(), app.format, ir)
		},
	}
	investorsCmd.Flags().IntVar(&investorsSize, "size", 10, "top stocks per investor type")

	var earningsMajor bool
	earningsCmd := &cobra.Command{
		Use:         "earnings",
		Short:       "Upcoming earnings-call calendar",
		Long:        "Upcoming earnings-call calendar. Use --major to show only major companies.",
		Annotations: map[string]string{"source": "wts"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			get := app.client.GetEarningCalls
			if earningsMajor {
				get = app.client.GetEarningCallHome
			}
			ec, err := get(cmd.Context())
			if err != nil {
				return err
			}
			return output.WriteEarningCalls(cmd.OutOrStdout(), app.format, ec)
		},
	}
	earningsCmd.Flags().BoolVar(&earningsMajor, "major", false, "show only major companies' earnings calls (curated)")

	sectorsCmd := &cobra.Command{
		Use:   "sectors [id]",
		Short: "Industry (TICS) sector fluctuations",
		Long: "Industry (TICS) sector fluctuations. With no argument, shows top-level sectors; " +
			"pass an id to drill into sub-industries.\n\n" +
			"Note: uses a WTS internal endpoint; not available via the official Open API and may change without notice.",
		Args:        cobra.MaximumNArgs(1),
		Annotations: map[string]string{"source": "wts"},
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			sectors, err := app.client.GetSectors(cmd.Context())
			if err != nil {
				return err
			}
			if len(args) == 1 {
				id, perr := strconv.Atoi(args[0])
				if perr != nil {
					return fmt.Errorf("invalid sector id %q", args[0])
				}
				sub, found := findSector(sectors.Items, id)
				if !found {
					return fmt.Errorf("sector id %d not found (run `market sectors` to list)", id)
				}
				sectors.Items = sub
			}
			return output.WriteSectors(cmd.OutOrStdout(), app.format, sectors)
		},
	}

	briefingCmd := &cobra.Command{
		Use:   "briefing",
		Short: "Personalized AI news briefing",
		Long: "Personalized AI news briefing.\n\n" +
			"Note: uses a WTS internal endpoint; not available via the official Open API and may change without notice.",
		Annotations: map[string]string{"source": "wts"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			b, err := app.client.GetNewsBriefing(cmd.Context())
			if err != nil {
				return err
			}
			return output.WriteNewsBriefing(cmd.OutOrStdout(), app.format, b)
		},
	}

	var (
		screenerNation string
		screenerSize   int
		screenerFilter string
	)
	screenerCmd := &cobra.Command{
		Use:   "screener [preset-id]",
		Short: "Stock screeners (value, dividend, growth, and more)",
		Long: `Stock screeners. With no argument, lists available presets; pass a preset id to run it.

For custom conditions, pass a raw JSON array via --filter (Toss web's filter schema):
  tossctl market screener --filter '[{"id":"marketCap","conditions":[...]}]' --nation kr

To shape the filter ID/condition structure, reference the preset output (--output json).`,
		Args:        cobra.MaximumNArgs(1),
		Annotations: map[string]string{"source": "wts"},
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			// --filter (custom raw) takes priority
			if screenerFilter != "" {
				res, err := app.client.RunScreenerRaw(cmd.Context(), screenerFilter, screenerNation, screenerSize)
				if err != nil {
					return err
				}
				return output.WriteScreenerResult(cmd.OutOrStdout(), app.format, res)
			}
			if len(args) == 0 {
				presets, err := app.client.GetScreenerPresets(cmd.Context())
				if err != nil {
					return err
				}
				return output.WriteScreenerPresets(cmd.OutOrStdout(), app.format, presets)
			}
			res, err := app.client.RunScreener(cmd.Context(), args[0], screenerNation, screenerSize)
			if err != nil {
				return err
			}
			return output.WriteScreenerResult(cmd.OutOrStdout(), app.format, res)
		},
	}
	screenerCmd.Flags().StringVar(&screenerNation, "nation", "kr", "market: kr | us")
	screenerCmd.Flags().IntVar(&screenerSize, "size", 30, "max stocks to return")
	screenerCmd.Flags().StringVar(&screenerFilter, "filter", "", "custom raw filter JSON array (instead of a preset)")

	var themesSize int
	themesCmd := &cobra.Command{
		Use:   "themes",
		Short: "Theme fluctuation ranking",
		Long: "Theme fluctuation ranking (today's top-moving themes).\n\n" +
			"Note: uses a WTS internal endpoint; not available via the official Open API and may change without notice.",
		Annotations: map[string]string{"source": "wts"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			tr, err := app.client.GetThemeRankings(cmd.Context(), themesSize)
			if err != nil {
				return err
			}
			return output.WriteThemeRankings(cmd.OutOrStdout(), app.format, tr)
		},
	}
	themesCmd.Flags().IntVar(&themesSize, "size", 20, "number of ranked themes (0 = all)")

	cmd.AddCommand(hoursCmd, fxCmd, indexCmd, rankingCmd, signalsCmd, investorsCmd, earningsCmd, briefingCmd, sectorsCmd, themesCmd, screenerCmd)
	return cmd
}
