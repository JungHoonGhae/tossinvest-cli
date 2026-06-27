// Package hybrid routes read operations between the official Toss Open API and
// the unofficial web-session (WTS) client. The Client EMBEDS *client.Client, so
// every WTS method is promoted unchanged; only the reads that the official API
// can cleanly serve are overridden below. Each override has the EXACT WTS
// signature so it shadows the embedded method, and delegates the routing
// decision to route().
//
// Routing policy (route):
//   - off==nil OR Prefer=="wts"  -> pure WTS passthrough (identical to today).
//   - Prefer auto/official       -> try official; on success return it silently;
//     on a fallback-eligible failure (official.ShouldFallback && Fallback) emit a
//     one-line stderr notice and retry via WTS; on a domain error (e.g. 404)
//     return it as-is with NO fallback.
package hybrid

import (
	"context"
	"fmt"
	"io"

	"github.com/JungHoonGhae/tossinvest-cli/internal/client"
	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/official"
)

// Policy controls how the hybrid router chooses between official and WTS.
//   - Prefer: "auto" (try official, fall back), "wts" (always WTS), "official".
//   - Fallback: whether a fallback-eligible official failure retries via WTS.
type Policy struct {
	Prefer   string
	Fallback bool
}

// Client embeds the WTS client and overrides the reads the official API serves.
type Client struct {
	*client.Client
	off    *official.Client
	pol    Policy
	stderr io.Writer
}

// New builds a hybrid client. If off is nil (no official credentials) every
// override degrades to a pure WTS passthrough. A nil stderr is replaced with
// io.Discard so overrides never write to a nil writer.
func New(wts *client.Client, off *official.Client, pol Policy, stderr io.Writer) *Client {
	if stderr == nil {
		stderr = io.Discard
	}
	return &Client{Client: wts, off: off, pol: pol, stderr: stderr}
}

// route is the single decision point. It is intentionally backend-agnostic
// (takes two closures, never touches c.off itself beyond the nil check) so the
// routing logic is unit-testable without any real client.
func route[T any](c *Client, official func() (T, error), wts func() (T, error)) (T, error) {
	if c.off == nil || c.pol.Prefer == "wts" {
		return wts()
	}
	v, err := official()
	if err == nil {
		// Happy path is silent on purpose: a "via official" line on every call
		// would spam stderr.
		return v, nil
	}
	if c.pol.Fallback && officialShouldFallback(err) {
		fmt.Fprintf(c.stderr, "tossctl: official path unavailable, falling back to web session (%v)\n", err)
		return wts()
	}
	return v, err // official's domain error — no fallback.
}

// officialShouldFallback is a thin alias so route reads cleanly; it reports
// whether err indicates the official API is unavailable (transient/auth/etc.)
// rather than a domain error like 404.
func officialShouldFallback(err error) bool { return official.ShouldFallback(err) }

// --- Overrides: official -> (fallback) WTS ---------------------------------

// ListAccounts routes to off.Accounts. The official path has no cursor, so the
// returned cursor is "" when official answers; WTS retains its own cursor.
func (c *Client) ListAccounts(ctx context.Context) ([]domain.Account, string, error) {
	type res struct {
		accts  []domain.Account
		cursor string
	}
	r, err := route(c,
		func() (res, error) {
			a, e := c.off.Accounts(ctx)
			return res{accts: a, cursor: ""}, e
		},
		func() (res, error) {
			a, cur, e := c.Client.ListAccounts(ctx)
			return res{accts: a, cursor: cur}, e
		})
	return r.accts, r.cursor, err
}

// ListPositions routes to off.Holdings(ctx, "") (all symbols).
func (c *Client) ListPositions(ctx context.Context) ([]domain.Position, error) {
	return route(c,
		func() ([]domain.Position, error) { return c.off.Holdings(ctx, "") },
		func() ([]domain.Position, error) { return c.Client.ListPositions(ctx) })
}

// GetQuote routes to off.Prices for a single symbol. An empty official result
// is a definitive "not found" — official answered, so we do NOT fall back.
func (c *Client) GetQuote(ctx context.Context, symbol string) (domain.Quote, error) {
	return route(c,
		func() (domain.Quote, error) {
			qs, err := c.off.Prices(ctx, []string{symbol})
			if err != nil {
				return domain.Quote{}, err
			}
			if len(qs) == 0 {
				return domain.Quote{}, fmt.Errorf("hybrid: no quote found for %s", symbol)
			}
			return qs[0], nil
		},
		func() (domain.Quote, error) { return c.Client.GetQuote(ctx, symbol) })
}

// GetOrderBook routes to off.Orderbook.
func (c *Client) GetOrderBook(ctx context.Context, symbol string) (domain.OrderBook, error) {
	return route(c,
		func() (domain.OrderBook, error) { return c.off.Orderbook(ctx, symbol) },
		func() (domain.OrderBook, error) { return c.Client.GetOrderBook(ctx, symbol) })
}

// GetTrades routes to off.Trades.
func (c *Client) GetTrades(ctx context.Context, symbol string, count int) (domain.TradeList, error) {
	return route(c,
		func() (domain.TradeList, error) { return c.off.Trades(ctx, symbol, count) },
		func() (domain.TradeList, error) { return c.Client.GetTrades(ctx, symbol, count) })
}

// GetChart routes to off.Candles (no before-cursor, unadjusted).
func (c *Client) GetChart(ctx context.Context, symbol, interval string, count int) (domain.Chart, error) {
	return route(c,
		func() (domain.Chart, error) { return c.off.Candles(ctx, symbol, interval, count, "", false) },
		func() (domain.Chart, error) { return c.Client.GetChart(ctx, symbol, interval, count) })
}

// GetPriceLimits routes to off.PriceLimits.
func (c *Client) GetPriceLimits(ctx context.Context, symbol string) (domain.PriceLimits, error) {
	return route(c,
		func() (domain.PriceLimits, error) { return c.off.PriceLimits(ctx, symbol) },
		func() (domain.PriceLimits, error) { return c.Client.GetPriceLimits(ctx, symbol) })
}

// GetStockWarnings routes to off.Warnings.
func (c *Client) GetStockWarnings(ctx context.Context, symbol string) (domain.StockWarnings, error) {
	return route(c,
		func() (domain.StockWarnings, error) { return c.off.Warnings(ctx, symbol) },
		func() (domain.StockWarnings, error) { return c.Client.GetStockWarnings(ctx, symbol) })
}

// GetSellableQuantity routes to off.SellableQuantity.
func (c *Client) GetSellableQuantity(ctx context.Context, symbol string) (domain.SellableQuantity, error) {
	return route(c,
		func() (domain.SellableQuantity, error) { return c.off.SellableQuantity(ctx, symbol) },
		func() (domain.SellableQuantity, error) { return c.Client.GetSellableQuantity(ctx, symbol) })
}

// GetCommission routes to off.Commissions.
func (c *Client) GetCommission(ctx context.Context, symbol string) (domain.Commission, error) {
	return route(c,
		func() (domain.Commission, error) { return c.off.Commissions(ctx, symbol) },
		func() (domain.Commission, error) { return c.Client.GetCommission(ctx, symbol) })
}

// --- Intentionally NOT overridden (served by embedded WTS) ------------------
//
// These are left to the embedded *client.Client because the official API does
// not map cleanly onto the WTS contract:
//
//   - GetAccountSummary: official exposes BuyingPower, a different type/concept
//     than the WTS account summary — not a drop-in substitute.
//   - GetExchangeRates: WTS returns the full set (plural); official ExchangeRate
//     is a single base/quote pair, so it cannot satisfy the plural signature.
//   - Order reads (ListPendingOrders / ListCompletedOrders / FindOrder): the
//     official status-enum mapping is still uncertain, so routing them risks
//     silently wrong statuses.
//   - Order writes (PlacePendingOrder / CancelPendingOrder / AmendPendingOrder):
//     these are the Broker's concern and are handled in Task 10 — not here.
