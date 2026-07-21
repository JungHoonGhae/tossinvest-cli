package ops

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Probe hosts — raw URLs on purpose (probes bypass the typed client so a
// server-side contract change is caught even when client code is in lockstep).
const (
	probeAPI  = "https://wts-api.tossinvest.com"
	probeCert = "https://wts-cert-api.tossinvest.com"
	probeInfo = "https://wts-info-api.tossinvest.com"
)

func statusAndPath(path, typ string) func(int, []byte) error {
	return func(status int, body []byte) error {
		if err := ExpectStatus(status, 200); err != nil {
			return err
		}
		return ExpectPath(body, path, typ)
	}
}

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
			Probe: &ProbeSpec{Name: "market-index", Method: "GET",
				URL:   probeCert + "/api/v1/dashboard/wts/overview/indicator/index",
				Check: statusAndPath("result.majorIndicatorInfos", "array")},
			handler: func(ctx context.Context, d *Deps, _ map[string]any) (any, error) {
				return d.WTS.GetMarketIndices(ctx)
			},
		},
		{
			ID: "index_detail", Method: "GET", Path: "wts:market/index", Backend: "wts",
			Category: "market", Summary: "Index detail quote (OHLC, 52w high/low) by code or name. WTS-only.",
			Params: []Param{{Name: "query", Type: "string", Required: true, Desc: `index code or name, e.g. "nasdaq" or "코스피"`}},
			Probe: &ProbeSpec{Name: "index-prices", Method: "GET",
				URL:   probeInfo + "/api/v1/index-prices/KGG01P",
				Check: statusAndPath("result.close", "number")},
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
			Params: []Param{{Name: "size", Type: "integer", Desc: "number of rows (default 20)"}},
			Probe: &ProbeSpec{Name: "stock-ranking", Method: "GET",
				URL:   probeInfo + "/api/v1/rankings/realtime/stock?size=1",
				Check: statusAndPath("result.data", "array")},
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
			Params: []Param{{Name: "size", Type: "integer", Desc: "number of rows (default 20)"}},
			Probe: &ProbeSpec{Name: "investor-rankings", Method: "GET",
				URL:   probeInfo + "/api/v1/dashboard/wts/overview/rankings/by-investors",
				Check: statusAndPath("result.rankings", "object")},
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
			Params: []Param{{Name: "size", Type: "integer", Desc: "number of rows (0 = all)"}},
			Probe: &ProbeSpec{Name: "theme-rankings", Method: "GET",
				URL:   probeInfo + "/api/v1/tics/rankings",
				Check: statusAndPath("result.data", "array")},
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
			Probe: &ProbeSpec{Name: "sectors-tics", Method: "GET",
				URL:   probeInfo + "/api/v1/tics/all",
				Check: statusAndPath("result.ticsItems", "array")},
			handler: func(ctx context.Context, d *Deps, _ map[string]any) (any, error) {
				return d.WTS.GetSectors(ctx)
			},
		},
		{
			ID: "ai_signals", Method: "GET", Path: "wts:market/signals", Backend: "wts",
			Category: "market", Summary: "Toss AI trading signals. WTS-only.",
			Probe: &ProbeSpec{Name: "ai-signals", Method: "GET",
				URL:   probeInfo + "/api/v2/reasoning-contents/interest",
				Check: statusAndPath("result.data", "array")},
			handler: func(ctx context.Context, d *Deps, _ map[string]any) (any, error) {
				return d.WTS.GetAISignals(ctx)
			},
		},
		{
			ID: "screener_presets", Method: "GET", Path: "wts:market/screener", Backend: "wts",
			Category: "market", Summary: "Screener presets (value/dividend/growth condition searches). WTS-only.",
			Probe: &ProbeSpec{Name: "screener-presets", Method: "GET",
				URL:   probeCert + "/api/v2/screener/presets/common?useCustom=true",
				Check: statusAndPath("result", "array")},
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
			Probe: &ProbeSpec{Name: "trading-flows", Method: "GET",
				URL:   probeInfo + "/api/v1/stock-infos/trade/trend/trading-trend?productCode=A005930&size=1",
				Check: statusAndPath("result.body", "array")},
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
			Probe: &ProbeSpec{Name: "earning-call", Method: "GET",
				URL:   probeInfo + "/api/v1/earning-call/upcoming",
				Check: statusAndPath("result", "array")},
			handler: func(ctx context.Context, d *Deps, _ map[string]any) (any, error) {
				return d.WTS.GetEarningCalls(ctx)
			},
		},
		{
			ID: "news_briefing", Method: "GET", Path: "wts:market/briefing", Backend: "wts",
			Category: "market", Summary: "Personalized AI news briefing (headlines grouped by theme). WTS-only.",
			Probe: &ProbeSpec{Name: "news-briefing", Method: "GET",
				URL:   probeInfo + "/api/v1/dashboard/wts/overview/ai-signals/personalized",
				Check: statusAndPath("result.items", "array")},
			handler: func(ctx context.Context, d *Deps, _ map[string]any) (any, error) {
				return d.WTS.GetNewsBriefing(ctx)
			},
		},
		{
			ID: "community_rankings", Method: "GET", Path: "wts:community/rankings", Backend: "wts",
			Category: "market", Summary: "Toss community rankings (influencer / profit / followers). WTS-only.",
			Params: []Param{{Name: "type", Type: "string", Required: true, Desc: `"influencer", "profit", or "followers"`}},
			Probe: &ProbeSpec{Name: "community-rankings", Method: "GET",
				URL:   probeInfo + "/api/v1/community/top-rankings/INFLUENCER",
				Check: statusAndPath("result.items", "array")},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				rankType, err := argString(args, "type")
				if err != nil {
					return nil, err
				}
				return d.WTS.GetCommunityRankings(ctx, rankType)
			},
		},
		{
			ID: "lending_expected", Method: "GET", Path: "wts:lending/revenue/expected", Backend: "wts",
			Category: "account", Summary: "Projected share-lending (대주) income for the account — monthly/yearly USD totals plus per-stock breakdown. Works even without an active lending agreement (zeros). WTS-only.",
			Probe: &ProbeSpec{Name: "lending-expected", Method: "GET",
				URL: probeCert + "/api/v1/lending/revenue/account/expected",
				Check: func(status int, _ []byte) error {
					return ExpectStatus(status, 200)
				}},
			handler: func(ctx context.Context, d *Deps, _ map[string]any) (any, error) {
				return d.WTS.GetLendingExpected(ctx)
			},
		},
		{
			ID: "accumulation_plans", Method: "GET", Path: "wts:autotrade/plan/find", Backend: "wts",
			Category: "portfolio", Summary: "All stock-accumulation (주식모으기) recurring-buy plans on the account — which stocks, Active vs Paused, amount/quantity, frequency, rounds completed. WTS-only.",
			Probe: &ProbeSpec{Name: "accumulation-plans", Method: "GET",
				URL:   probeAPI + "/api/v2/autotrade/plan/find",
				Check: statusAndPath("result", "array")},
			handler: func(ctx context.Context, d *Deps, _ map[string]any) (any, error) {
				return d.WTS.ListAccumulationPlans(ctx)
			},
		},
		{
			ID: "accumulation_status", Method: "GET", Path: "wts:autotrade/plan/stock", Backend: "wts",
			Category: "portfolio", Summary: "Stock-accumulation (주식모으기) plan(s) for one stock — Active vs Paused, amount/quantity, frequency. WTS-only.",
			Params: []Param{{Name: "symbol", Type: "string", Required: true, Desc: "ticker (e.g. 005930, AAPL) or Toss product code"}},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				symbol, err := argString(args, "symbol")
				if err != nil {
					return nil, err
				}
				return d.WTS.GetAccumulationPlansByStock(ctx, symbol)
			},
		},
		{
			ID: "profit_overview", Method: "POST", Path: "wts:profit/overview", Backend: "wts",
			Category: "portfolio", Summary: "Cumulative realized profit across every category — trading gains, dividends, share-lending, maturity, deposit interest — each in KRW and USD. A cumulative view distinct from account summary (current valuation). WTS-only.",
			Probe: &ProbeSpec{Name: "profit-overview", Method: "POST",
				URL:  probeCert + "/api/v1/profit/overview",
				Body: `{}`,
				Check: func(status int, body []byte) error {
					if err := ExpectStatus(status, 200); err != nil {
						return err
					}
					return ExpectPath(body, "result.totalAssetAmount", "object")
				}},
			handler: func(ctx context.Context, d *Deps, _ map[string]any) (any, error) {
				return d.WTS.GetProfitOverview(ctx)
			},
		},
		{
			ID: "tax_overseas", Method: "GET", Path: "wts:tax/transfer-income/overseas", Backend: "wts",
			Category: "portfolio", Summary: "Overseas-stock transfer income (해외주식 양도소득) for a tax year — tax summary (rate, deduction, tax due) plus per-stock profit/loss. For capital-gains tax filing (KRW). WTS-only.",
			Params: []Param{{Name: "year", Type: "integer", Desc: "tax year (0 = current)"}},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				year, err := argInt(args, "year")
				if err != nil {
					return nil, err
				}
				if year == 0 {
					year = time.Now().Year()
				}
				return d.WTS.GetOverseasTransferIncome(ctx, year)
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
			Probe: &ProbeSpec{Name: "account-summary-overview", Method: "GET",
				URL: probeCert + "/api/v3/my-assets/summaries/markets/all/overview",
				Check: func(status int, body []byte) error {
					if err := ExpectStatus(status, 200); err != nil {
						return err
					}
					if err := ExpectPath(body, "result.overviewByMarket", "object"); err != nil {
						return err
					}
					return ExpectPath(body, "result.totalAssetAmount", "number")
				}},
			handler: func(ctx context.Context, d *Deps, _ map[string]any) (any, error) {
				return d.WTS.GetAccountSummary(ctx)
			},
		},
		{
			ID: "completed_orders", Method: "GET", Path: "wts:trading/my-orders/completed", Backend: "wts",
			Category: "order", Summary: "Completed (filled) orders with average execution price + executed quantity — the data needed for realized P&L. Supports a date range and paging. WTS-only.",
			Params: []Param{
				{Name: "market", Type: "string", Desc: `"kr", "us", or "all" (default all)`},
				{Name: "from", Type: "string", Desc: "start date YYYY-MM-DD (default: current month start)"},
				{Name: "to", Type: "string", Desc: "end date YYYY-MM-DD (default: today)"},
				{Name: "size", Type: "integer", Desc: "page size (default 50)"},
				{Name: "page", Type: "integer", Desc: "page number, 1-based (default 1)"},
			},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				market, err := argString(args, "market")
				if err != nil {
					return nil, err
				}
				if market == "" {
					market = "all"
				}
				fromStr, err := argString(args, "from")
				if err != nil {
					return nil, err
				}
				toStr, err := argString(args, "to")
				if err != nil {
					return nil, err
				}
				// No range given → default helper (current month).
				if fromStr == "" && toStr == "" {
					return d.WTS.ListCompletedOrders(ctx, market)
				}
				size, err := argInt(args, "size")
				if err != nil {
					return nil, err
				}
				if size <= 0 {
					size = 50
				}
				page, err := argInt(args, "page")
				if err != nil {
					return nil, err
				}
				if page <= 0 {
					page = 1
				}
				now := time.Now()
				from, to := now, now
				if fromStr != "" {
					if from, err = time.ParseInLocation("2006-01-02", fromStr, now.Location()); err != nil {
						return nil, fmt.Errorf("invalid `from` date (want YYYY-MM-DD): %v", err)
					}
				} else {
					from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
				}
				if toStr != "" {
					if to, err = time.ParseInLocation("2006-01-02", toStr, now.Location()); err != nil {
						return nil, fmt.Errorf("invalid `to` date (want YYYY-MM-DD): %v", err)
					}
				}
				return d.WTS.ListCompletedOrdersRange(ctx, market, from, to, size, page)
			},
		},
		{
			ID: "transactions_overview", Method: "GET", Path: "wts:transactions/overview", Backend: "wts",
			Category: "account", Summary: "Transaction history overview (deposits/withdrawals/trades summary). WTS-only.",
			Params: []Param{{Name: "market", Type: "string", Desc: `"kr" or "us" (optional)`}},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				market, err := argString(args, "market")
				if err != nil {
					return nil, err
				}
				return d.WTS.GetTransactionsOverview(ctx, market)
			},
		},
		{
			ID: "positions", Method: "GET", Path: "wts:portfolio/positions", Backend: "wts",
			Category: "account", Summary: "Current holdings with valuation and unrealized P&L (works without an official key). WTS-only.",
			// #29 재발 방지: 빈 `{}` body 는 빈 sections 를 돌려준다 — 진짜 sections
			// 배열에 SORTED_OVERVIEW 항목과 products[] 가 있어야 정상.
			Probe: &ProbeSpec{Name: "portfolio-positions", Method: "POST",
				URL:  probeCert + "/api/v2/dashboard/asset/sections/all",
				Body: `{"types":["SORTED_OVERVIEW"]}`,
				Check: func(status int, body []byte) error {
					if err := ExpectStatus(status, 200); err != nil {
						return err
					}
					if err := ExpectPath(body, "result.sections", "array"); err != nil {
						return err
					}
					var env struct {
						Result struct {
							Sections []struct {
								Type string `json:"type"`
								Data struct {
									Products json.RawMessage `json:"products"`
								} `json:"data"`
							} `json:"sections"`
						} `json:"result"`
					}
					if err := json.Unmarshal(body, &env); err != nil {
						return fmt.Errorf("decode sections: %v", err)
					}
					if len(env.Result.Sections) == 0 {
						return fmt.Errorf("result.sections is empty — likely body-contract regression (#29-class)")
					}
					if env.Result.Sections[0].Type != "SORTED_OVERVIEW" {
						return fmt.Errorf("expected section[0].type=SORTED_OVERVIEW, got %q", env.Result.Sections[0].Type)
					}
					if !bytes.HasPrefix(bytes.TrimSpace(env.Result.Sections[0].Data.Products), []byte("[")) {
						return fmt.Errorf("section[0].data.products is not an array")
					}
					return nil
				}},
			handler: func(ctx context.Context, d *Deps, _ map[string]any) (any, error) {
				return d.WTS.ListPositions(ctx)
			},
		},
		{
			ID: "pending_orders", Method: "GET", Path: "wts:trading/orders/pending", Backend: "wts",
			Category: "order", Summary: "Open (unfilled) pending orders (works without an official key). WTS-only.",
			Probe: &ProbeSpec{Name: "pending-orders", Method: "GET",
				URL:   probeCert + "/api/v1/trading/orders/histories/all/pending",
				Check: statusAndPath("result", "array")},
			handler: func(ctx context.Context, d *Deps, _ map[string]any) (any, error) {
				return d.WTS.ListPendingOrders(ctx)
			},
		},
		{
			ID: "transactions", Method: "GET", Path: "wts:transactions/list", Backend: "wts",
			Category: "account", Summary: "Detailed transaction list (deposits/withdrawals/trades) aggregated across pages over a date range. WTS-only.",
			Params: []Param{
				{Name: "market", Type: "string", Desc: `"kr", "us", or "all" (default all)`},
				{Name: "from", Type: "string", Desc: "start date YYYY-MM-DD (default: 1 year ago)"},
				{Name: "to", Type: "string", Desc: "end date YYYY-MM-DD (default: today)"},
				{Name: "filter", Type: "string", Desc: "transaction type filter (optional)"},
				{Name: "size", Type: "integer", Desc: "page size (default 50)"},
				{Name: "page_limit", Type: "integer", Desc: "max pages to aggregate (default 20)"},
			},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				market, err := argString(args, "market")
				if err != nil {
					return nil, err
				}
				if market == "" {
					market = "all"
				}
				filter, err := argString(args, "filter")
				if err != nil {
					return nil, err
				}
				size, err := argInt(args, "size")
				if err != nil {
					return nil, err
				}
				if size <= 0 {
					size = 50
				}
				pageLimit, err := argInt(args, "page_limit")
				if err != nil {
					return nil, err
				}
				if pageLimit <= 0 {
					pageLimit = 20
				}
				now := time.Now()
				from, to := now.AddDate(-1, 0, 0), now
				fromStr, err := argString(args, "from")
				if err != nil {
					return nil, err
				}
				if fromStr != "" {
					if from, err = time.ParseInLocation("2006-01-02", fromStr, now.Location()); err != nil {
						return nil, fmt.Errorf("invalid `from` date (want YYYY-MM-DD): %v", err)
					}
				}
				toStr, err := argString(args, "to")
				if err != nil {
					return nil, err
				}
				if toStr != "" {
					if to, err = time.ParseInLocation("2006-01-02", toStr, now.Location()); err != nil {
						return nil, fmt.Errorf("invalid `to` date (want YYYY-MM-DD): %v", err)
					}
				}
				return d.WTS.ListAllTransactions(ctx, market, from, to, filter, size, pageLimit)
			},
		},
		{
			ID: "watchlist", Method: "GET", Path: "wts:watchlist", Backend: "wts",
			Category: "watchlist", Summary: "Watchlist items (with change/change-rate). WTS-only.",
			Probe: &ProbeSpec{Name: "watchlist", Method: "POST",
				URL:  probeCert + "/api/v2/dashboard/asset/sections/all",
				Body: `{"types":["WATCHLIST"]}`,
				Check: func(status int, body []byte) error {
					if err := ExpectStatus(status, 200); err != nil {
						return err
					}
					var env struct {
						Result struct {
							Sections []struct {
								Type string `json:"type"`
							} `json:"sections"`
						} `json:"result"`
					}
					if err := json.Unmarshal(body, &env); err != nil {
						return fmt.Errorf("decode sections: %v", err)
					}
					if len(env.Result.Sections) == 0 {
						return fmt.Errorf("result.sections is empty — likely body-contract regression")
					}
					if env.Result.Sections[0].Type != "WATCHLIST" {
						return fmt.Errorf("expected section[0].type=WATCHLIST, got %q", env.Result.Sections[0].Type)
					}
					return nil
				}},
			handler: func(ctx context.Context, d *Deps, _ map[string]any) (any, error) {
				return d.WTS.ListWatchlist(ctx)
			},
		},
		{
			ID: "watchlist_groups", Method: "GET", Path: "wts:watchlist/groups", Backend: "wts",
			Category: "watchlist", Summary: "Watchlist folders/groups. WTS-only.",
			Probe: &ProbeSpec{Name: "watchlist-groups", Method: "GET",
				URL:   probeCert + "/api/v1/new-watchlists?includePrice=false&lazyLoad=true",
				Check: statusAndPath("result.watchlists", "array")},
			handler: func(ctx context.Context, d *Deps, _ map[string]any) (any, error) {
				return d.WTS.ListWatchlistGroups(ctx)
			},
		},
		{
			ID: "earnings_major", Method: "GET", Path: "wts:market/earnings/major", Backend: "wts",
			Category: "market", Summary: "Curated major-company earnings calls. WTS-only.",
			Probe: &ProbeSpec{Name: "earning-call-home", Method: "GET",
				URL:   probeInfo + "/api/v1/earning-call/home",
				Check: statusAndPath("result.majorCompanies", "object")},
			handler: func(ctx context.Context, d *Deps, _ map[string]any) (any, error) {
				return d.WTS.GetEarningCallHome(ctx)
			},
		},
	}
}
