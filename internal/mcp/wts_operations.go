package mcp

import "context"

// wtsOperations returns the catalog of WTS-only read operations — features the
// official Open API does not expose (rankings, flows, indices, AI signals,
// screener, sectors, earnings, briefing, community, dividends, Prime,
// transactions). They dispatch to the embedded web-session client (d.WTS) and
// are marked Backend "wts" so Catalog.Call verifies a session before running
// them (a missing session yields a "run tossctl auth login" error).
//
// These are read-only. Order execution stays on the official path (writes.go).
func wtsOperations() []Operation {
	return []Operation{
		{
			ID: "market_indices", Method: "GET", Path: "wts:market/indices", Backend: "wts",
			Category: "market", Summary: "Major market indices (KOSPI/KOSDAQ/NASDAQ/S&P500/VIX etc). WTS-only.",
			handler: func(ctx context.Context, d *Deps, _ map[string]any) (any, error) {
				return d.WTS.GetMarketIndices(ctx)
			},
		},
		{
			ID: "index_detail", Method: "GET", Path: "wts:market/index", Backend: "wts",
			Category: "market", Summary: "Index detail quote (OHLC, 52w high/low) by code or name. WTS-only.",
			Params:   []Param{{Name: "query", Type: "string", Required: true, Desc: `index code or name, e.g. "nasdaq" or "코스피"`}},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				query, err := argString(args, "query")
				if err != nil {
					return nil, err
				}
				return d.WTS.GetIndexDetail(ctx, query)
			},
		},
		{
			ID: "stock_ranking", Method: "GET", Path: "wts:rankings/realtime/stock", Backend: "wts",
			Category: "market", Summary: "Realtime popularity ranking (most-viewed stocks). WTS-only.",
			Params:   []Param{{Name: "size", Type: "integer", Desc: "number of rows (default 20)"}},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				size, err := argInt(args, "size")
				if err != nil {
					return nil, err
				}
				return d.WTS.GetStockRanking(ctx, size)
			},
		},
		{
			ID: "investor_rankings", Method: "GET", Path: "wts:rankings/investor", Backend: "wts",
			Category: "market", Summary: "Top net-buy stocks by investor type (foreign/institution/individual). WTS-only.",
			Params:   []Param{{Name: "size", Type: "integer", Desc: "number of rows (default 20)"}},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				size, err := argInt(args, "size")
				if err != nil {
					return nil, err
				}
				return d.WTS.GetInvestorRankings(ctx, size)
			},
		},
		{
			ID: "theme_rankings", Method: "GET", Path: "wts:rankings/theme", Backend: "wts",
			Category: "market", Summary: "Theme movement ranking (top-moving Toss themes today). WTS-only.",
			Params:   []Param{{Name: "size", Type: "integer", Desc: "number of rows (0 = all)"}},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				size, err := argInt(args, "size")
				if err != nil {
					return nil, err
				}
				return d.WTS.GetThemeRankings(ctx, size)
			},
		},
		{
			ID: "sectors", Method: "GET", Path: "wts:market/sectors", Backend: "wts",
			Category: "market", Summary: "Sector movement (39 top-level sectors, 1d/1mo/1y returns). WTS-only.",
			handler: func(ctx context.Context, d *Deps, _ map[string]any) (any, error) {
				return d.WTS.GetSectors(ctx)
			},
		},
		{
			ID: "ai_signals", Method: "GET", Path: "wts:market/signals", Backend: "wts",
			Category: "market", Summary: "Toss AI trading signals. WTS-only.",
			handler: func(ctx context.Context, d *Deps, _ map[string]any) (any, error) {
				return d.WTS.GetAISignals(ctx)
			},
		},
		{
			ID: "screener_presets", Method: "GET", Path: "wts:market/screener", Backend: "wts",
			Category: "market", Summary: "Screener presets (value/dividend/growth condition searches). WTS-only.",
			handler: func(ctx context.Context, d *Deps, _ map[string]any) (any, error) {
				return d.WTS.GetScreenerPresets(ctx)
			},
		},
		{
			ID: "trading_flows", Method: "GET", Path: "wts:stock/trading-trend", Backend: "wts",
			Category: "market", Summary: "Per-stock investor net-buy flows (individual/foreign/institution, KRX only). WTS-only.",
			Params: []Param{
				{Name: "symbol", Type: "string", Required: true, Desc: "KR ticker, e.g. 005930"},
				{Name: "size", Type: "integer", Desc: "number of days (default 20)"},
			},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				symbol, err := argString(args, "symbol")
				if err != nil {
					return nil, err
				}
				size, err := argInt(args, "size")
				if err != nil {
					return nil, err
				}
				return d.WTS.GetTradingFlows(ctx, symbol, size)
			},
		},
		{
			ID: "earning_calls", Method: "GET", Path: "wts:market/earnings", Backend: "wts",
			Category: "market", Summary: "Upcoming earnings-call schedule. WTS-only.",
			handler: func(ctx context.Context, d *Deps, _ map[string]any) (any, error) {
				return d.WTS.GetEarningCalls(ctx)
			},
		},
		{
			ID: "news_briefing", Method: "GET", Path: "wts:market/briefing", Backend: "wts",
			Category: "market", Summary: "Personalized AI news briefing (headlines grouped by theme). WTS-only.",
			handler: func(ctx context.Context, d *Deps, _ map[string]any) (any, error) {
				return d.WTS.GetNewsBriefing(ctx)
			},
		},
		{
			ID: "community_rankings", Method: "GET", Path: "wts:community/rankings", Backend: "wts",
			Category: "market", Summary: "Toss community rankings (influencer / profit / followers). WTS-only.",
			Params:   []Param{{Name: "type", Type: "string", Required: true, Desc: `"influencer", "profit", or "followers"`}},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				rankType, err := argString(args, "type")
				if err != nil {
					return nil, err
				}
				return d.WTS.GetCommunityRankings(ctx, rankType)
			},
		},
		{
			ID: "dividends", Method: "GET", Path: "wts:portfolio/dividends", Backend: "wts",
			Category: "portfolio", Summary: "Annual dividend history (received/scheduled, by region, monthly). WTS-only.",
			Params: []Param{
				{Name: "year", Type: "integer", Desc: "year (0 = current)"},
				{Name: "by_payment_date", Type: "boolean", Desc: "use payment date (incl. tax/fees) instead of ex-date"},
			},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				year, err := argInt(args, "year")
				if err != nil {
					return nil, err
				}
				byPay, err := argBool(args, "by_payment_date")
				if err != nil {
					return nil, err
				}
				return d.WTS.GetDividends(ctx, year, byPay)
			},
		},
		{
			ID: "prime_status", Method: "GET", Path: "wts:account/prime", Backend: "wts",
			Category: "account", Summary: "Toss Prime subscription status and this month's fee/interest benefits. WTS-only.",
			handler: func(ctx context.Context, d *Deps, _ map[string]any) (any, error) {
				return d.WTS.GetPrimeStatus(ctx)
			},
		},
		{
			ID: "account_summary", Method: "GET", Path: "wts:account/summary", Backend: "wts",
			Category: "account", Summary: "Account summary (balance, holdings valuation, P&L). WTS-only.",
			handler: func(ctx context.Context, d *Deps, _ map[string]any) (any, error) {
				return d.WTS.GetAccountSummary(ctx)
			},
		},
		{
			ID: "transactions_overview", Method: "GET", Path: "wts:transactions/overview", Backend: "wts",
			Category: "account", Summary: "Transaction history overview (deposits/withdrawals/trades). WTS-only.",
			Params:   []Param{{Name: "market", Type: "string", Desc: `"kr" or "us" (optional)`}},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				market, err := argString(args, "market")
				if err != nil {
					return nil, err
				}
				return d.WTS.GetTransactionsOverview(ctx, market)
			},
		},
	}
}
