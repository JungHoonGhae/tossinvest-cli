package ops

import (
	"context"
	"fmt"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/official"
	"strings"
)

// ---------------------------------------------------------------------------
// Argument coercion helpers.
//
// Arguments arrive as a JSON-decoded map, so numbers are float64, strings are
// string, and arrays are []any. These helpers coerce leniently and report
// clear errors for wrong types on required fields.
// ---------------------------------------------------------------------------

func argString(args map[string]any, name string) (string, error) {
	v, ok := args[name]
	if !ok {
		return "", nil
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("parameter %q must be a string", name)
	}
	return s, nil
}

func argInt(args map[string]any, name string) (int, error) {
	v, ok := args[name]
	if !ok {
		return 0, nil
	}
	switch n := v.(type) {
	case float64:
		return int(n), nil
	case int:
		return n, nil
	default:
		return 0, fmt.Errorf("parameter %q must be an integer", name)
	}
}

func argBool(args map[string]any, name string) (bool, error) {
	v, ok := args[name]
	if !ok {
		return false, nil
	}
	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("parameter %q must be a boolean", name)
	}
	return b, nil
}

func argStringSlice(args map[string]any, name string) ([]string, error) {
	v, ok := args[name]
	if !ok {
		return nil, nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("parameter %q must be an array of strings", name)
	}
	out := make([]string, 0, len(arr))
	for i, e := range arr {
		s, ok := e.(string)
		if !ok {
			return nil, fmt.Errorf("parameter %q[%d] must be a string", name, i)
		}
		out = append(out, s)
	}
	return out, nil
}

// readOperations returns the catalog of read-only official API operations.
func readOperations() []Operation {
	return []Operation{
		{
			ID: "auth_status", Method: "GET", Path: "local:auth-status", Backend: "none",
			Category: "system", Summary: "Which backends are connected and when they expire — WTS web session and official Open API key. No secrets returned. Call this to diagnose auth before other operations; a disconnected/expired backend means run `tossctl auth login` (WTS) or `tossctl openapi login` (official).",
			handler: func(_ context.Context, d *Deps, _ map[string]any) (any, error) {
				return d.Auth, nil
			},
		},
		{
			ID: "accounts", Method: "GET", Path: "/api/v1/accounts",
			Category: "account", Summary: "List brokerage accounts.",
			handler: func(ctx context.Context, d *Deps, _ map[string]any) (any, error) {
				return d.Client.Accounts(ctx)
			},
		},
		{
			ID: "buying_power", Method: "GET", Path: "/api/v1/buying-power",
			Category: "account", Summary: "Cash buying power for a currency.",
			Params: []Param{{Name: "currency", Type: "string", Required: true, Desc: `"KRW" or "USD"`}},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				currency, err := argString(args, "currency")
				if err != nil {
					return nil, err
				}
				return d.Client.BuyingPower(ctx, currency)
			},
		},
		{
			ID: "holdings", Method: "GET", Path: "/api/v1/holdings",
			Category: "account", Summary: "Current stock holdings; optionally filter by symbol.",
			Params: []Param{{Name: "symbol", Type: "string", Desc: "optional ticker filter"}},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				symbol, err := argString(args, "symbol")
				if err != nil {
					return nil, err
				}
				return d.Client.Holdings(ctx, symbol)
			},
		},
		{
			ID: "orders", Method: "GET", Path: "/api/v1/orders",
			Category: "order",
			Summary: "List orders with optional filters. Returns one PAGE: check has_next, " +
				"and pass next_cursor back as cursor to get the rest — the first call is not " +
				"necessarily the whole history.",
			Params: []Param{
				{Name: "status", Type: "string", Desc: `"OPEN" or "CLOSED"`},
				{Name: "symbol", Type: "string"},
				{Name: "from", Type: "string", Desc: "start date YYYY-MM-DD"},
				{Name: "to", Type: "string", Desc: "end date YYYY-MM-DD"},
				{Name: "cursor", Type: "string", Desc: "next_cursor from a previous call's response"},
				{Name: "limit", Type: "integer", Desc: "orders per page (0 = API default)"},
			},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				var f official.OrdersFilter
				var err error
				if f.Status, err = argString(args, "status"); err != nil {
					return nil, err
				}
				if f.Symbol, err = argString(args, "symbol"); err != nil {
					return nil, err
				}
				if f.From, err = argString(args, "from"); err != nil {
					return nil, err
				}
				if f.To, err = argString(args, "to"); err != nil {
					return nil, err
				}
				if f.Cursor, err = argString(args, "cursor"); err != nil {
					return nil, err
				}
				if f.Limit, err = argInt(args, "limit"); err != nil {
					return nil, err
				}
				return d.Client.Orders(ctx, f)
			},
		},
		{
			ID: "order", Method: "GET", Path: "/api/v1/orders/{orderId}",
			Category: "order", Summary: "Fetch a single order by id.",
			Params: []Param{{Name: "order_id", Type: "string", Required: true}},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				orderID, err := argString(args, "order_id")
				if err != nil {
					return nil, err
				}
				return d.Client.OrderByID(ctx, orderID)
			},
		},
		{
			ID: "sellable_quantity", Method: "GET", Path: "/api/v1/sellable-quantity", Backend: "auto",
			Category: "order", Summary: "Sellable quantity for a symbol. Served by either backend (official first, web-session fallback).",
			Params: []Param{{Name: "symbol", Type: "string", Required: true}},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				symbol, err := argString(args, "symbol")
				if err != nil {
					return nil, err
				}
				return d.WTS.GetSellableQuantity(ctx, symbol)
			},
		},
		{
			ID: "prices", Method: "GET", Path: "/api/v1/prices",
			Category: "market", Summary: "Latest prices for one or more symbols.",
			Params: []Param{{Name: "symbols", Type: "string[]", Required: true, Desc: "list of tickers"}},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				symbols, err := argStringSlice(args, "symbols")
				if err != nil {
					return nil, err
				}
				return d.Client.Prices(ctx, symbols)
			},
		},
		{
			ID: "stocks", Method: "GET", Path: "/api/v1/stocks",
			Category: "market", Summary: "Stock metadata/quotes for one or more symbols.",
			Params: []Param{{Name: "symbols", Type: "string[]", Required: true, Desc: "list of tickers"}},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				symbols, err := argStringSlice(args, "symbols")
				if err != nil {
					return nil, err
				}
				return d.Client.Stocks(ctx, symbols)
			},
		},
		{
			ID: "orderbook", Method: "GET", Path: "/api/v1/orderbook", Backend: "auto",
			Category: "market", Summary: "Order book (bids/asks) for a symbol. Served by either backend (official first, web-session fallback).",
			Params: []Param{{Name: "symbol", Type: "string", Required: true}},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				symbol, err := argString(args, "symbol")
				if err != nil {
					return nil, err
				}
				return d.WTS.GetOrderBook(ctx, symbol)
			},
		},
		{
			ID: "trades", Method: "GET", Path: "/api/v1/trades", Backend: "auto",
			Category: "market", Summary: "Recent trades for a symbol. Served by either backend (official first, web-session fallback).",
			Params: []Param{
				{Name: "symbol", Type: "string", Required: true},
				{Name: "count", Type: "integer", Desc: "number of trades (0 = API default)"},
			},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				symbol, err := argString(args, "symbol")
				if err != nil {
					return nil, err
				}
				count, err := argInt(args, "count")
				if err != nil {
					return nil, err
				}
				return d.WTS.GetTrades(ctx, symbol, count)
			},
		},
		{
			ID: "candles", Method: "GET", Path: "/api/v1/candles",
			Category: "market", Summary: "OHLC candles for a symbol.",
			Params: []Param{
				{Name: "symbol", Type: "string", Required: true},
				{Name: "interval", Type: "string", Required: true, Desc: "e.g. 1d, 1w, 1m"},
				{Name: "count", Type: "integer", Desc: "number of candles (0 = API default)"},
				{Name: "before", Type: "string", Desc: "cursor: return candles before this timestamp"},
				{Name: "adjusted", Type: "boolean", Desc: "price adjusted for splits/dividends"},
			},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				symbol, err := argString(args, "symbol")
				if err != nil {
					return nil, err
				}
				interval, err := argString(args, "interval")
				if err != nil {
					return nil, err
				}
				count, err := argInt(args, "count")
				if err != nil {
					return nil, err
				}
				before, err := argString(args, "before")
				if err != nil {
					return nil, err
				}
				adjusted, err := argBool(args, "adjusted")
				if err != nil {
					return nil, err
				}
				return d.Client.Candles(ctx, symbol, interval, count, before, adjusted)
			},
		},
		{
			ID: "price_limits", Method: "GET", Path: "/api/v1/price-limits", Backend: "auto",
			Category: "market", Summary: "Upper/lower price limits for a symbol. Served by either backend (official first, web-session fallback).",
			Params: []Param{{Name: "symbol", Type: "string", Required: true}},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				symbol, err := argString(args, "symbol")
				if err != nil {
					return nil, err
				}
				return d.WTS.GetPriceLimits(ctx, symbol)
			},
		},
		{
			ID: "warnings", Method: "GET", Path: "/api/v1/stocks/{symbol}/warnings", Backend: "auto",
			Category: "market", Summary: "Trading warnings/designations for a symbol. Served by either backend (official first, web-session fallback).",
			Params: []Param{{Name: "symbol", Type: "string", Required: true}},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				symbol, err := argString(args, "symbol")
				if err != nil {
					return nil, err
				}
				return d.WTS.GetStockWarnings(ctx, symbol)
			},
		},
		{
			ID: "exchange_rate", Method: "GET", Path: "/api/v1/exchange-rate",
			Category: "market", Summary: "Exchange rate between two currencies.",
			Params: []Param{
				{Name: "base", Type: "string", Required: true, Desc: `e.g. "USD"`},
				{Name: "quote", Type: "string", Required: true, Desc: `e.g. "KRW"`},
			},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				base, err := argString(args, "base")
				if err != nil {
					return nil, err
				}
				quote, err := argString(args, "quote")
				if err != nil {
					return nil, err
				}
				return d.Client.ExchangeRate(ctx, base, quote)
			},
		},
		{
			ID: "commissions", Method: "GET", Path: "/api/v1/commissions", Backend: "auto",
			Category: "market", Summary: "Commission/fee schedule for a symbol. Served by either backend (official first, web-session fallback).",
			Params: []Param{{Name: "symbol", Type: "string", Required: true}},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				symbol, err := argString(args, "symbol")
				if err != nil {
					return nil, err
				}
				return d.WTS.GetCommission(ctx, symbol)
			},
		},
		{
			ID: "market_stocks", Method: "GET", Path: "/api/v1/stocks/all",
			Category: "market", Summary: "Every tradable stock on one market (the universe), sorted by symbol. Thousands of rows in a single response with no pagination — low-churn batch data refreshed daily, so cache rather than re-request. Use the returned symbols with the per-symbol operations.",
			Params: []Param{
				{Name: "market", Type: "string", Required: true, Desc: `"KOSPI", "KOSDAQ", "NYSE", "NASDAQ", "AMEX", "KR_ETC", or "US_ETC"`},
				{Name: "status", Type: "string", Desc: `"ACTIVE" (default), "SCHEDULED", or "DELISTED"`},
				{Name: "security_type", Type: "string", Desc: `"STOCK", "ETF", "REIT", "ETN", … (omit for all)`},
				{Name: "common_share", Type: "boolean", Desc: "common shares only"},
			},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				market, err := argString(args, "market")
				if err != nil {
					return nil, err
				}
				status, _ := args["status"].(string)
				secType, _ := args["security_type"].(string)
				common, _ := args["common_share"].(bool)
				return d.Client.ListStocks(ctx, market, status, secType, common)
			},
		},
		{
			// CLI 에만 있고 ops 에 빠져 있었다 — 에이전트는 시장 수급을 볼 수 없었다.
			// 종목별은 stock_supply 다. 이쪽은 지수(KOSPI/KOSDAQ) 단위다.
			ID: "market_investor_trading", Method: "GET", Path: "/api/v1/market-indicators/{symbol}/investor-trading",
			Category: "market", Summary: "Market-wide investor trading for an index (KOSPI/KOSDAQ) over time — individual, foreigner, institution, other. For a single stock's supply use stock_supply instead.",
			Params: []Param{
				{Name: "symbol", Type: "string", Required: true, Desc: "index symbol, e.g. KOSPI or KOSDAQ"},
				{Name: "interval", Type: "string", Desc: `"1d" (default), "1w", "1mo", or "1y"`},
				{Name: "count", Type: "integer", Desc: "rows, up to 100"},
				{Name: "until", Type: "string", Desc: "cursor from a previous page"},
			},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				symbol, err := argString(args, "symbol")
				if err != nil {
					return nil, err
				}
				count, err := argInt(args, "count")
				if err != nil {
					return nil, err
				}
				interval, _ := args["interval"].(string)
				until, _ := args["until"].(string)
				return d.Client.MarketInvestorTrading(ctx, symbol, interval, count, until)
			},
		},
		{
			ID: "stock_supply", Method: "GET", Path: "/api/v1/stocks/{symbol}/supply",
			Category: "market", Summary: "KR stock supply series — investor-type trading (with the 7-way institution breakdown, foreign holding, CFD balance), short selling, credit trades, securities lending, or program trades. Daily time series with a cursor. Fields not yet tallied for a date are null, which is distinct from zero.",
			Params: []Param{
				{Name: "symbol", Type: "string", Required: true, Desc: "KR ticker, e.g. 005930"},
				{Name: "type", Type: "string", Desc: `"investor" (default), "short", "credit", "lending", or "program"`},
				{Name: "count", Type: "integer", Desc: "rows per page (server default 10)"},
				{Name: "until", Type: "string", Desc: "cursor from a previous page's next_until"},
			},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				symbol, err := argString(args, "symbol")
				if err != nil {
					return nil, err
				}
				kindArg, _ := args["type"].(string)
				if kindArg == "" {
					kindArg = "investor"
				}
				var kind domain.SupplyKind
				for _, k := range official.SupplyKinds() {
					if string(k) == strings.ToLower(strings.TrimSpace(kindArg)) {
						kind = k
					}
				}
				if kind == "" {
					return nil, fmt.Errorf("unknown type %q", kindArg)
				}
				count, err := argInt(args, "count")
				if err != nil {
					return nil, err
				}
				until, _ := args["until"].(string)
				return d.Client.Supply(ctx, symbol, kind, count, until)
			},
		},
		{
			ID: "market_calendar", Method: "GET", Path: "/api/v1/market-calendar/{country}",
			Category: "market", Summary: "Trading-hours calendar (previous/today/next business day) for KR or US, normalized to one shape across both markets: each day carries a holiday flag and a session list (pre_market, day_market for US, regular_market, after_market) with KR single-price auction windows where they apply. Also available as `tossctl market business-days`.",
			Params: []Param{
				{Name: "country", Type: "string", Required: true, Desc: `"KR" or "US"`},
				{Name: "date", Type: "string", Desc: "reference date YYYY-MM-DD (default: today)"},
			},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				country, err := argString(args, "country")
				if err != nil {
					return nil, err
				}
				date, err := argString(args, "date")
				if err != nil {
					return nil, err
				}
				return d.Client.MarketCalendar(ctx, country, date)
			},
		},
	}
}
