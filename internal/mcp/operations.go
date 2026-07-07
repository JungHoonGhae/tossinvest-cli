package mcp

import (
	"context"
	"fmt"

	"github.com/JungHoonGhae/tossinvest-cli/internal/official"
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
			ID: "accounts", Method: "GET", Path: "/api/v1/accounts",
			Category: "account", Summary: "List brokerage accounts.",
			handler: func(ctx context.Context, c *official.Client, _ map[string]any) (any, error) {
				return c.Accounts(ctx)
			},
		},
		{
			ID: "buying_power", Method: "GET", Path: "/api/v1/buying-power",
			Category: "account", Summary: "Cash buying power for a currency.",
			Params: []Param{{Name: "currency", Type: "string", Required: true, Desc: `"KRW" or "USD"`}},
			handler: func(ctx context.Context, c *official.Client, args map[string]any) (any, error) {
				currency, err := argString(args, "currency")
				if err != nil {
					return nil, err
				}
				return c.BuyingPower(ctx, currency)
			},
		},
		{
			ID: "holdings", Method: "GET", Path: "/api/v1/holdings",
			Category: "account", Summary: "Current stock holdings; optionally filter by symbol.",
			Params: []Param{{Name: "symbol", Type: "string", Desc: "optional ticker filter"}},
			handler: func(ctx context.Context, c *official.Client, args map[string]any) (any, error) {
				symbol, err := argString(args, "symbol")
				if err != nil {
					return nil, err
				}
				return c.Holdings(ctx, symbol)
			},
		},
		{
			ID: "orders", Method: "GET", Path: "/api/v1/orders",
			Category: "order", Summary: "List orders with optional filters.",
			Params: []Param{
				{Name: "status", Type: "string", Desc: `"OPEN" or "CLOSED"`},
				{Name: "symbol", Type: "string"},
				{Name: "from", Type: "string", Desc: "start date YYYY-MM-DD"},
				{Name: "to", Type: "string", Desc: "end date YYYY-MM-DD"},
				{Name: "cursor", Type: "string", Desc: "pagination cursor"},
				{Name: "limit", Type: "integer"},
			},
			handler: func(ctx context.Context, c *official.Client, args map[string]any) (any, error) {
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
				return c.Orders(ctx, f)
			},
		},
		{
			ID: "order", Method: "GET", Path: "/api/v1/orders/{orderId}",
			Category: "order", Summary: "Fetch a single order by id.",
			Params: []Param{{Name: "order_id", Type: "string", Required: true}},
			handler: func(ctx context.Context, c *official.Client, args map[string]any) (any, error) {
				orderID, err := argString(args, "order_id")
				if err != nil {
					return nil, err
				}
				return c.OrderByID(ctx, orderID)
			},
		},
		{
			ID: "sellable_quantity", Method: "GET", Path: "/api/v1/sellable-quantity",
			Category: "order", Summary: "Sellable quantity for a symbol.",
			Params: []Param{{Name: "symbol", Type: "string", Required: true}},
			handler: func(ctx context.Context, c *official.Client, args map[string]any) (any, error) {
				symbol, err := argString(args, "symbol")
				if err != nil {
					return nil, err
				}
				return c.SellableQuantity(ctx, symbol)
			},
		},
		{
			ID: "prices", Method: "GET", Path: "/api/v1/prices",
			Category: "market", Summary: "Latest prices for one or more symbols.",
			Params: []Param{{Name: "symbols", Type: "string[]", Required: true, Desc: "list of tickers"}},
			handler: func(ctx context.Context, c *official.Client, args map[string]any) (any, error) {
				symbols, err := argStringSlice(args, "symbols")
				if err != nil {
					return nil, err
				}
				return c.Prices(ctx, symbols)
			},
		},
		{
			ID: "stocks", Method: "GET", Path: "/api/v1/stocks",
			Category: "market", Summary: "Stock metadata/quotes for one or more symbols.",
			Params: []Param{{Name: "symbols", Type: "string[]", Required: true, Desc: "list of tickers"}},
			handler: func(ctx context.Context, c *official.Client, args map[string]any) (any, error) {
				symbols, err := argStringSlice(args, "symbols")
				if err != nil {
					return nil, err
				}
				return c.Stocks(ctx, symbols)
			},
		},
		{
			ID: "orderbook", Method: "GET", Path: "/api/v1/orderbook",
			Category: "market", Summary: "Order book (bids/asks) for a symbol.",
			Params: []Param{{Name: "symbol", Type: "string", Required: true}},
			handler: func(ctx context.Context, c *official.Client, args map[string]any) (any, error) {
				symbol, err := argString(args, "symbol")
				if err != nil {
					return nil, err
				}
				return c.Orderbook(ctx, symbol)
			},
		},
		{
			ID: "trades", Method: "GET", Path: "/api/v1/trades",
			Category: "market", Summary: "Recent trades for a symbol.",
			Params: []Param{
				{Name: "symbol", Type: "string", Required: true},
				{Name: "count", Type: "integer", Desc: "number of trades (0 = API default)"},
			},
			handler: func(ctx context.Context, c *official.Client, args map[string]any) (any, error) {
				symbol, err := argString(args, "symbol")
				if err != nil {
					return nil, err
				}
				count, err := argInt(args, "count")
				if err != nil {
					return nil, err
				}
				return c.Trades(ctx, symbol, count)
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
			handler: func(ctx context.Context, c *official.Client, args map[string]any) (any, error) {
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
				return c.Candles(ctx, symbol, interval, count, before, adjusted)
			},
		},
		{
			ID: "price_limits", Method: "GET", Path: "/api/v1/price-limits",
			Category: "market", Summary: "Upper/lower price limits for a symbol.",
			Params: []Param{{Name: "symbol", Type: "string", Required: true}},
			handler: func(ctx context.Context, c *official.Client, args map[string]any) (any, error) {
				symbol, err := argString(args, "symbol")
				if err != nil {
					return nil, err
				}
				return c.PriceLimits(ctx, symbol)
			},
		},
		{
			ID: "warnings", Method: "GET", Path: "/api/v1/stocks/{symbol}/warnings",
			Category: "market", Summary: "Trading warnings/designations for a symbol.",
			Params: []Param{{Name: "symbol", Type: "string", Required: true}},
			handler: func(ctx context.Context, c *official.Client, args map[string]any) (any, error) {
				symbol, err := argString(args, "symbol")
				if err != nil {
					return nil, err
				}
				return c.Warnings(ctx, symbol)
			},
		},
		{
			ID: "exchange_rate", Method: "GET", Path: "/api/v1/exchange-rate",
			Category: "market", Summary: "Exchange rate between two currencies.",
			Params: []Param{
				{Name: "base", Type: "string", Required: true, Desc: `e.g. "USD"`},
				{Name: "quote", Type: "string", Required: true, Desc: `e.g. "KRW"`},
			},
			handler: func(ctx context.Context, c *official.Client, args map[string]any) (any, error) {
				base, err := argString(args, "base")
				if err != nil {
					return nil, err
				}
				quote, err := argString(args, "quote")
				if err != nil {
					return nil, err
				}
				return c.ExchangeRate(ctx, base, quote)
			},
		},
		{
			ID: "commissions", Method: "GET", Path: "/api/v1/commissions",
			Category: "market", Summary: "Commission/fee schedule for a symbol.",
			Params: []Param{{Name: "symbol", Type: "string", Required: true}},
			handler: func(ctx context.Context, c *official.Client, args map[string]any) (any, error) {
				symbol, err := argString(args, "symbol")
				if err != nil {
					return nil, err
				}
				return c.Commissions(ctx, symbol)
			},
		},
	}
}
