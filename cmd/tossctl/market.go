package main

import (
	"fmt"
	"strconv"
	"strings"

	tossclient "github.com/JungHoonGhae/tossinvest-cli/internal/client"
	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
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
		Short: i18n.T("market.short"),
	}

	hoursCmd := &cobra.Command{
		Use:         "hours",
		Short:       i18n.T("market.hours.short"),
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
		Short:       i18n.T("market.fx.short"),
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
		Short:       i18n.T("market.index.short"),
		Long:        i18n.T("market.index.long"),
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
		Short:       i18n.T("market.ranking.short"),
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
		Short:       i18n.T("market.signals.short"),
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
		Short:       i18n.T("market.investors.short"),
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
		Short:       i18n.T("market.earnings.short"),
		Long:        i18n.T("market.earnings.long"),
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
		Use:         "sectors [id]",
		Short:       i18n.T("market.sectors.short"),
		Long:        i18n.T("market.sectors.long"),
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

	var newsScope string
	var newsLimit int
	var newsFull bool
	newsCmd := &cobra.Command{
		Use:         "news",
		Short:       i18n.T("market.news.short"),
		Long:        i18n.T("market.news.long"),
		Annotations: map[string]string{"source": "wts"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			scope, err := tossclient.NewsScope(newsScope)
			if err != nil {
				return err
			}
			if newsLimit < 0 {
				return fmt.Errorf("--limit 은 0 보다 커야 합니다")
			}
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			n, err := app.client.GetMarketNews(cmd.Context(), scope, newsLimit)
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteMarketNews(cmd.OutOrStdout(), app.format, n, newsFull)
		},
	}
	newsCmd.Flags().StringVar(&newsScope, "type", "",
		"news scope: "+strings.Join(tossclient.NewsScopeAliases(), " | ")+
			" (default "+tossclient.DefaultNewsScope+"); a raw server enum also works")
	newsCmd.Flags().IntVar(&newsLimit, "limit", 0,
		fmt.Sprintf("max items (server caps at %d); 0 = server default", tossclient.MaxNewsLimit))
	newsCmd.Flags().BoolVar(&newsFull, "full", false, "include each article's summary")

	var calendarMonth string
	calendarCmd := &cobra.Command{
		Use:         "calendar",
		Short:       i18n.T("market.calendar.short"),
		Long:        i18n.T("market.calendar.long"),
		Annotations: map[string]string{"source": "wts"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			c, err := app.client.GetMarketCalendar(cmd.Context(), calendarMonth)
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteMarketCalendar(cmd.OutOrStdout(), app.format, c)
		},
	}
	calendarCmd.Flags().StringVar(&calendarMonth, "month", "",
		"month to show as YYYY-MM (default: current month)")

	var issuesFull bool
	issuesCmd := &cobra.Command{
		Use:         "issues",
		Short:       i18n.T("market.issues.short"),
		Long:        i18n.T("market.issues.long"),
		Annotations: map[string]string{"source": "wts"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			m, err := app.client.GetMarketIssues(cmd.Context())
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteMarketIssues(cmd.OutOrStdout(), app.format, m, issuesFull)
		},
	}
	issuesCmd.Flags().BoolVar(&issuesFull, "full", false, "include the articles behind each topic")

	briefingCmd := &cobra.Command{
		Use:         "briefing",
		Short:       i18n.T("market.briefing.short"),
		Long:        i18n.T("market.briefing.long"),
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
		Use:         "screener [preset-id]",
		Short:       i18n.T("market.screener.short"),
		Long:        i18n.T("market.screener.long"),
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
		Use:         "themes",
		Short:       i18n.T("market.themes.short"),
		Long:        i18n.T("market.themes.long"),
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

	var rankingsType, rankingsMarket, rankingsDuration string
	var rankingsCount int
	var rankingsExcludeCaution bool
	rankingsCmd := &cobra.Command{
		Use:         "rankings",
		Short:       i18n.T("market.rankings.short"),
		Long:        i18n.T("market.rankings.long"),
		Annotations: map[string]string{"source": "official"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			r, err := app.client.Rankings(cmd.Context(), rankingsType, rankingsMarket, rankingsDuration, rankingsExcludeCaution, rankingsCount)
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteRanking(cmd.OutOrStdout(), app.format, r)
		},
	}
	rankingsCmd.Flags().StringVar(&rankingsType, "type", "MARKET_TRADING_AMOUNT", "ranking type (MARKET_TRADING_AMOUNT|MARKET_TRADING_VOLUME|TOP_GAINERS|TOP_LOSERS|TOSS_SECURITIES_TRADING_AMOUNT|TOSS_SECURITIES_TRADING_VOLUME)")
	rankingsCmd.Flags().StringVar(&rankingsMarket, "market", "KR", "market country (KR|US)")
	rankingsCmd.Flags().StringVar(&rankingsDuration, "duration", "1d", "duration (realtime|1d|1w|1mo|3mo|6mo|1y)")
	rankingsCmd.Flags().IntVar(&rankingsCount, "count", 0, "number of rows (max 100; 0 = API default)")
	rankingsCmd.Flags().BoolVar(&rankingsExcludeCaution, "exclude-caution", false, "exclude investment-caution stocks")

	indicatorCmd := &cobra.Command{
		Use:         "indicator [symbols]",
		Short:       i18n.T("market.indicator.short"),
		Long:        i18n.T("market.indicator.long"),
		Args:        cobra.ArbitraryArgs,
		Annotations: map[string]string{"source": "official"},
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			symbols := args
			if len(symbols) == 0 {
				symbols = []string{"KOSPI", "KOSDAQ"}
			} else if len(symbols) == 1 {
				// allow comma-separated single arg: "KOSPI,KOSDAQ"
				symbols = strings.Split(symbols[0], ",")
			}
			p, err := app.client.MarketIndicatorPrices(cmd.Context(), symbols)
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteMarketIndicatorPrices(cmd.OutOrStdout(), app.format, p)
		},
	}

	var candleInterval, candleBefore string
	var candleCount int
	indicatorCandlesCmd := &cobra.Command{
		Use:         "indicator-candles <symbol>",
		Short:       i18n.T("market.indicatorCandles.short"),
		Long:        i18n.T("market.indicatorCandles.long"),
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"source": "official"},
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			c, err := app.client.MarketIndicatorCandles(cmd.Context(), args[0], candleInterval, candleCount, candleBefore)
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteMarketIndicatorCandles(cmd.OutOrStdout(), app.format, c)
		},
	}
	indicatorCandlesCmd.Flags().StringVar(&candleInterval, "interval", "1d", "candle interval (1m|1d)")
	indicatorCandlesCmd.Flags().IntVar(&candleCount, "count", 0, "number of candles (max 200; 0 = API default)")
	indicatorCandlesCmd.Flags().StringVar(&candleBefore, "before", "", "pagination upper bound (ISO 8601; pass previous nextBefore)")

	var investorTradingInterval, investorTradingUntil string
	var investorTradingCount int
	investorTradingCmd := &cobra.Command{
		Use:         "investor-trading <symbol>",
		Short:       i18n.T("market.investorTrading.short"),
		Long:        i18n.T("market.investorTrading.long"),
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"source": "official"},
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			it, err := app.client.MarketInvestorTrading(cmd.Context(), args[0], investorTradingInterval, investorTradingCount, investorTradingUntil)
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteInvestorTrading(cmd.OutOrStdout(), app.format, it)
		},
	}
	investorTradingCmd.Flags().StringVar(&investorTradingInterval, "interval", "1d", "aggregation interval (1d|1w|1mo|1y)")
	investorTradingCmd.Flags().IntVar(&investorTradingCount, "count", 0, "number of records (max 100; 0 = API default)")
	investorTradingCmd.Flags().StringVar(&investorTradingUntil, "until", "", "inclusive upper-bound date (YYYY-MM-DD)")

	optionHoursCmd := &cobra.Command{
		Use:         "option-hours",
		Short:       i18n.T("market.optionHours.short"),
		Long:        i18n.T("market.optionHours.long"),
		Annotations: map[string]string{"source": "wts"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			hours, err := app.client.GetOptionTradingHours(cmd.Context())
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteOptionTradingHours(cmd.OutOrStdout(), app.format, hours)
		},
	}

	cmd.AddCommand(hoursCmd, fxCmd, indexCmd, rankingCmd, signalsCmd, investorsCmd, earningsCmd, briefingCmd, newsCmd, sectorsCmd, themesCmd, screenerCmd, rankingsCmd, indicatorCmd, indicatorCandlesCmd, investorTradingCmd, calendarCmd, issuesCmd, optionHoursCmd)
	return cmd
}
