package ops

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	tossclient "github.com/JungHoonGhae/tossinvest-cli/internal/client"
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
			ID: "account_detail", Method: "GET", Path: "wts:account/detail", Backend: "wts",
			Category: "account", Summary: "Account identity plus withdrawal capacity and credit-trading status — the read-only half of the web's 계좌관리 screen. Number, name, status, open date, last trade date; withdrawable cash by settlement day (D+0/1/2) with per-transaction and daily caps and today's usage; whether 미수거래 is open per market. The account number and holder name are returned in full here (an agent needs them to act) — do not echo them into shared output. WTS-only.",
			handler: func(ctx context.Context, d *Deps, _ map[string]any) (any, error) {
				return d.WTS.GetAccountDetail(ctx)
			},
		},
		{
			ID: "market_issues", Method: "GET", Path: "wts:lens/issues", Backend: "wts",
			Category: "market",
			Summary:  "Ranked board of the topics the market is talking about most, each with its rank movement (UP/DOWN), the number of articles behind it, and those articles. A different axis from market_news (flat headlines) and news_briefing (AI category grouping): here the topic ranking itself is the payload. Takes no parameters. WTS-only.",
			Probe: &ProbeSpec{Name: "market-issues", Method: "GET",
				URL: probeInfo + "/api/v1/lens/issues",
				Check: func(status int, body []byte) error {
					if err := ExpectStatus(status, 200); err != nil {
						return err
					}
					return ExpectPath(body, "result.issues", "array")
				}},
			handler: func(ctx context.Context, d *Deps, _ map[string]any) (any, error) {
				return d.WTS.GetMarketIssues(ctx)
			},
		},
		{
			ID: "auto_trades", Method: "GET", Path: "wts:trading/auto-trading/histories", Backend: "wts",
			Category: "order",
			Summary:  "Automated-trading rules armed on the account (STOP_LOSS, PROFIT_RATE, OCO, OTO) with their trigger and order prices. Read-only: arming and cancelling happen in the Toss app only. status is the server's numeric code translated to its enum name (6 = EXPIRED); status_code keeps the raw value. WTS-only.",
			Probe: &ProbeSpec{Name: "auto-trades", Method: "GET",
				URL: probeInfo + "/api/v3/trading/auto-trading/histories",
				Check: func(status int, body []byte) error {
					if err := ExpectStatus(status, 200); err != nil {
						return err
					}
					return ExpectPath(body, "result.body", "array")
				}},
			handler: func(ctx context.Context, d *Deps, _ map[string]any) (any, error) {
				return d.WTS.ListAutoTrades(ctx)
			},
		},
		{
			// 공식 API 에 이미 market_calendar 가 있다(국가별 거래일 캘린더,
			// read_operations.go). 이쪽은 지표·실적·휴장 **이벤트** 캘린더라
			// 다른 기능이므로 id 를 나눈다.
			ID: "market_events", Method: "POST", Path: "wts:calendar/monthly/{month}", Backend: "wts",
			Category: "market",
			Summary:  "One month of scheduled market events: economic releases (with the street's forecast, the actual print once published, and the prior value), Korean and US earnings announcements with their stock symbol and earnings-call time, and market holidays. month is a YYYY-MM path segment; omit it for the current month. The weekly AI summary is attached only for the present month. WTS-only.",
			Params: []Param{
				{Name: "month", Type: "string", Desc: "YYYY-MM; empty = current month"},
			},
			// Probe asserts the events array: an empty month is possible, but
			// the key vanishing means the shape changed.
			Probe: &ProbeSpec{Name: "market-calendar", Method: "POST",
				URL:  probeInfo + "/api/v4/calendar/monthly/" + time.Now().Format("2006-01"),
				Body: `{}`,
				Check: func(status int, body []byte) error {
					if err := ExpectStatus(status, 200); err != nil {
						return err
					}
					return ExpectPath(body, "result.events", "array")
				}},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				month, err := argString(args, "month")
				if err != nil {
					return nil, err
				}
				return d.WTS.GetMarketCalendar(ctx, month)
			},
		},
		{
			ID: "market_news", Method: "POST", Path: "wts:dashboard/wts/news", Backend: "wts",
			Category: "market", Summary: "Market news with each article's RELATED STOCKS and how they are moving right now — the part a plain headline list lacks. Scopes: all (widest, general market news, no stock linkage), watchlist / holdings (news about the user's own stocks, with moves), soaring (stocks spiking), recommended, latest. Server caps at 50 items; there is no pagination and no keyword search. WTS-only.",
			Params: []Param{
				{Name: "scope", Type: "string", Desc: "all (default) | recommended | watchlist | holdings | latest | soaring; a raw server enum also works"},
				{Name: "limit", Type: "integer", Desc: "max items, server caps at 50; 0 = server default"},
			},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				alias, err := argString(args, "scope")
				if err != nil {
					return nil, err
				}
				scope, err := tossclient.NewsScope(alias)
				if err != nil {
					return nil, err
				}
				limit, err := argInt(args, "limit")
				if err != nil {
					return nil, err
				}
				return d.WTS.GetMarketNews(ctx, scope, limit)
			},
		},
		{
			ID: "profit_period", Method: "POST", Path: "wts:profit/type/overview", Backend: "wts",
			Category: "portfolio", Summary: "Realized profit for ONE category over a date range — earned amount, return rate, and purchase basis in KRW and USD. Omit from/to for the whole history. The period-scoped counterpart to profit_overview (all-time, every category). WTS-only.",
			Params: []Param{
				{Name: "type", Type: "string", Desc: "category: sales | dividend | lending | account-interest (default sales)"},
				{Name: "from", Type: "string", Desc: "start date YYYY-MM-DD; omit for all time (must be paired with to)"},
				{Name: "to", Type: "string", Desc: "end date YYYY-MM-DD, not in the future; omit for all time"},
			},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				profitType, err := argString(args, "type")
				if err != nil {
					return nil, err
				}
				if profitType == "" {
					profitType = "sales"
				}
				if !slices.Contains(tossclient.ProfitTypes, profitType) {
					return nil, fmt.Errorf("type must be one of %s", strings.Join(tossclient.ProfitTypes, ", "))
				}
				from, to, err := profitRangeArgs(args)
				if err != nil {
					return nil, err
				}
				return d.WTS.GetPeriodProfit(ctx, profitType, from, to)
			},
		},
		{
			ID: "profit_daily", Method: "POST", Path: "wts:profit/wts/daily/market", Backend: "wts",
			Category: "portfolio", Summary: "Per-stock realized profit day by day — symbol, quantity, profit/loss, return rate, and the sell/buy amounts behind it, every page combined. Answers \"what did this position actually make?\" and feeds tax prep. currency selects the RATE BASIS (KRW folds in FX for foreign holdings, USD does not); it is not a filter — the same rows come back either way. WTS-only.",
			Params: []Param{
				{Name: "from", Type: "string", Desc: "start date YYYY-MM-DD; omit for all time (must be paired with to)"},
				{Name: "to", Type: "string", Desc: "end date YYYY-MM-DD, not in the future; omit for all time"},
				{Name: "currency", Type: "string", Desc: "rate basis: KRW (default) | USD — not a filter"},
			},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				currency, err := argString(args, "currency")
				if err != nil {
					return nil, err
				}
				if currency != "" {
					currency = strings.ToUpper(currency)
					if !slices.Contains(tossclient.ProfitCurrencies, currency) {
						return nil, fmt.Errorf("currency must be one of %s", strings.Join(tossclient.ProfitCurrencies, ", "))
					}
				}
				from, to, err := profitRangeArgs(args)
				if err != nil {
					return nil, err
				}
				return d.WTS.GetDailyProfit(ctx, from, to, currency)
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
			ID: "community_boards", Method: "GET", Path: "wts:community/boards", Backend: "wts",
			Category: "market", Summary: "Toss community lounges ranked by follower count, with comment counts and whether this account has joined. Server order is the ranking. WTS-only.",
			handler: func(ctx context.Context, d *Deps, _ map[string]any) (any, error) {
				return d.WTS.GetPopularBoards(ctx)
			},
		},
		{
			ID: "quote_crypto", Method: "GET", Path: "wts:quote/crypto", Backend: "wts",
			Category: "market", Summary: "KRW crypto prices (BTC/ETH/SOL/XRP) — OHLC, 52-week range, and the premium gap against the global market at the current USD/KRW rate. A volume-weighted average across aggregated exchanges, not one venue. WTS-only.",
			Params: []Param{{Name: "symbols", Type: "string", Required: true, Desc: `comma-separated, e.g. "BTC,ETH" (full codes like VWAP.KRW-BTC also work)`}},
			Probe: &ProbeSpec{Name: "quote-crypto", Method: "GET",
				URL: probeInfo + "/api/v1/crypto-prices?productCodes=VWAP.KRW-BTC",
				Check: func(status int, body []byte) error {
					if err := ExpectStatus(status, 200); err != nil {
						return err
					}
					return ExpectPath(body, "result.0.close", "number")
				}},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				symbols, err := argString(args, "symbols")
				if err != nil {
					return nil, err
				}
				return d.WTS.GetCryptoPrices(ctx, strings.Split(symbols, ","))
			},
		},
		{
			ID: "market_option_hours", Method: "GET", Path: "wts:market/option-hours", Backend: "wts",
			Category: "market", Summary: "US options session windows for the previous, current, and next business day. Equity hours are market_trading_hours; the two can diverge around holidays. WTS-only.",
			handler: func(ctx context.Context, d *Deps, _ map[string]any) (any, error) {
				return d.WTS.GetOptionTradingHours(ctx)
			},
		},
		{
			ID: "order_funding", Method: "GET", Path: "wts:order/funding", Backend: "wts",
			Category: "order", Summary: "Whether buying is possible right now and, when blocked, the deposit or exchange amount still required. Reports the gap, unlike account_summary which reports what is already orderable. WTS-only.",
			Probe: &ProbeSpec{Name: "order-funding", Method: "GET",
				URL: probeInfo + "/api/v2/trading/order/buy-control/required-deposit-amount",
				Check: func(status int, body []byte) error {
					if err := ExpectStatus(status, 200); err != nil {
						return err
					}
					return ExpectPath(body, "result.requiredDepositAmount", "number")
				}},
			handler: func(ctx context.Context, d *Deps, _ map[string]any) (any, error) {
				return d.WTS.GetOrderFunding(ctx)
			},
		},
		{
			ID: "tax_ria", Method: "GET", Path: "wts:tax/ria", Backend: "wts",
			Category: "account", Summary: "RIA account (해외주식 양도세 절세 계좌) tax-saving report: estimated capital-gains tax before/after the RIA deduction, the deduction's quarterly components, sell limit, and any further saving still reachable. Complements tax_overseas, which has no RIA concept. Mobile-app-only surface. WTS-only.",
			Probe: &ProbeSpec{Name: "ria-report", Method: "GET",
				URL: probeCert + "/api/v1/ria-calculator/report",
				Check: func(status int, body []byte) error {
					if err := ExpectStatus(status, 200); err != nil {
						return err
					}
					if err := ExpectPath(body, "result.transferIncomeTax.estimatedTaxSaving", "number"); err != nil {
						return err
					}
					return ExpectPath(body, "result.transferIncomeTaxDetail.riaDeduction", "object")
				}},
			handler: func(ctx context.Context, d *Deps, _ map[string]any) (any, error) {
				return d.WTS.GetRIAReport(ctx)
			},
		},
		{
			ID: "account_interest", Method: "GET", Path: "wts:account/interest", Backend: "wts",
			Category: "account", Summary: "Deposit-interest (예탁금 이용료) payments for a year: payment date, pre-tax amount, tax, net amount, accrual period, and whether it is still an estimate. Distinct from profit_summary type=account-interest, which is one period total. WTS-only.",
			Params: []Param{
				{Name: "year", Type: "integer", Desc: "Year to report (default: current year)"},
			},
			Probe: &ProbeSpec{Name: "account-interest-years", Method: "GET",
				URL: probeCert + "/api/v1/interest/accounts/annual/history/years",
				Check: func(status int, body []byte) error {
					if err := ExpectStatus(status, 200); err != nil {
						return err
					}
					return ExpectPath(body, "result", "array")
				}},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				// year 는 선택 — 없으면 클라이언트가 올해로 채운다.
				year := 0
				if _, ok := args["year"]; ok {
					v, err := argInt(args, "year")
					if err != nil {
						return nil, err
					}
					year = v
				}
				return d.WTS.GetAccountInterest(ctx, year)
			},
		},
		{
			ID: "account_commission", Method: "GET", Path: "wts:account/commission", Backend: "wts",
			Category: "account", Summary: "Commission schedule this account is charged, per market (KR equities, US equities, US options). Distinct from quote_commission, which is per-symbol. WTS-only.",
			Probe: &ProbeSpec{Name: "account-commission-info", Method: "GET",
				URL: probeAPI + "/api/v2/trading/commission-info",
				Check: func(status int, body []byte) error {
					if err := ExpectStatus(status, 200); err != nil {
						return err
					}
					// v2 is what makes the US-options tier non-null; if this
					// path starts behaving like v1 the tier check catches it.
					if err := ExpectPath(body, "result.commissionInfoKr.commissionRate", "number"); err != nil {
						return err
					}
					return ExpectPath(body, "result.commissionInfoUsOpt", "object")
				}},
			handler: func(ctx context.Context, d *Deps, _ map[string]any) (any, error) {
				return d.WTS.GetCommissionSchedule(ctx)
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

// profitRangeArgs reads the shared from/to pair and validates it through the
// same helper the CLI uses, so both surfaces reject the same inputs.
func profitRangeArgs(args map[string]any) (string, string, error) {
	from, err := argString(args, "from")
	if err != nil {
		return "", "", err
	}
	to, err := argString(args, "to")
	if err != nil {
		return "", "", err
	}
	return tossclient.ParseProfitRange(from, to)
}
