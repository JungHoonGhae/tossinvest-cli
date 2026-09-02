// Package hybrid routes read operations between the official Toss Open API and
// the unofficial web-session (WTS) client. The Client EMBEDS *client.Client, so
// every WTS method is promoted unchanged; only the reads that the official API
// can cleanly serve are overridden below. Each override has the EXACT WTS
// signature so it shadows the embedded method, and delegates the routing
// decision to route().
//
// Routing policy (route):
//   - off==nil OR Prefer==routing.WTS -> pure WTS passthrough (identical to today).
//   - Prefer auto/openapi        -> try official; on success return it silently;
//     on a fallback-eligible failure (official.ShouldFallback && Fallback) emit a
//     one-line stderr notice and retry via WTS; on a domain error (e.g. 404)
//     return it as-is with NO fallback.
package hybrid

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/client"
	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/official"
	"github.com/JungHoonGhae/tossinvest-cli/internal/orderintent"
	"github.com/JungHoonGhae/tossinvest-cli/internal/routing"
)

// Policy controls how the hybrid router chooses between official and WTS.
//   - Prefer: auto/openapi try official first; wts disables the official path.
//   - Fallback: whether a fallback-eligible official failure retries via WTS.
type Policy struct {
	Prefer   routing.Preference
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

// Official exposes the routed official client (nil when no credentials are
// configured, or when policy pins this run to WTS). The operation registry
// needs it directly: official-backend operations call typed official methods
// the hybrid router does not front. Nil is meaningful — Catalog.Call turns it
// into "run `tossctl openapi login`".
func (c *Client) Official() *official.Client {
	if c.pol.Prefer == routing.WTS {
		return nil
	}
	return c.off
}

// route is the single decision point. It is intentionally backend-agnostic
// (takes two closures, never touches c.off itself beyond the nil check) so the
// routing logic is unit-testable without any real client.
func route[T any](c *Client, official func() (T, error), wts func() (T, error)) (T, error) {
	if c.off == nil || c.pol.Prefer == routing.WTS {
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

// officialSymbolPattern mirrors the official API's own symbol validation —
// `^[A-Za-z0-9.,\-]+$`, taken verbatim from the `rule: Pattern` it returns on a
// 400.
//
// Option contract guids carry underscores (OPT_AAPL260805C00230000_20260722),
// so sending one to the official path earns a validation 400. That is a domain
// error, not an availability failure, so ShouldFallback says no and the request
// dies there — even though WTS serves these fine. Deciding before the call
// keeps a request we know will fail from being made at all.
var officialSymbolPattern = regexp.MustCompile(`^[A-Za-z0-9.,\-]+$`)

// routeSymbol is route plus the knowledge that some symbols the official API
// simply cannot express. Those go straight to WTS.
func routeSymbol[T any](c *Client, symbol string, official func() (T, error), wts func() (T, error)) (T, error) {
	if !officialSymbolPattern.MatchString(symbol) {
		return wts()
	}
	return route(c, official, wts)
}

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

// GetExchangeRates is the one override that prefers WTS.
//
// Every other route tries official first. Here that would be a downgrade: WTS
// returns the whole feed (USD/KRW, DXY, …) while the official endpoint answers
// one base/quote pair at a time. Routing official-first would silently drop
// rows for users who hold both credentials.
//
// So WTS leads, and official serves as the floor — a user with only an official
// key used to get nothing at all from `market fx`, which made the command's
// "both" annotation untrue. One pair is less than the full feed, but the table
// shows exactly what came back, so the reduction is visible rather than silent.
func (c *Client) GetExchangeRates(ctx context.Context) (domain.ExchangeRates, error) {
	rates, err := c.Client.GetExchangeRates(ctx)
	if err == nil {
		return rates, nil
	}
	off := c.Official()
	if off == nil {
		return domain.ExchangeRates{}, err
	}
	one, offErr := off.ExchangeRate(ctx, "USD", "KRW")
	if offErr != nil {
		// 원래(WTS) 에러를 살린다 — 세션이 없다는 사실이 사용자가 고칠 수 있는
		// 정보이고, 폴백이 실패했다는 사실은 그 다음이다.
		return domain.ExchangeRates{}, err
	}
	fmt.Fprintf(c.stderr, "tossctl: web session unavailable, showing USD/KRW from the official API only (%v)\n", err)
	return domain.ExchangeRates{Rates: []domain.ExchangeRate{one}, FetchedAt: time.Now().UTC()}, nil
}

// GetQuote routes to off.Prices for a single symbol. An empty official result
// is a definitive "not found" — official answered, so we do NOT fall back.
func (c *Client) GetQuote(ctx context.Context, symbol string) (domain.Quote, error) {
	return routeSymbol(c, symbol,
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
	return routeSymbol(c, symbol,
		func() (domain.OrderBook, error) { return c.off.Orderbook(ctx, symbol) },
		func() (domain.OrderBook, error) { return c.Client.GetOrderBook(ctx, symbol) })
}

// GetTrades routes to off.Trades.
func (c *Client) GetTrades(ctx context.Context, symbol string, count int) (domain.TradeList, error) {
	return routeSymbol(c, symbol,
		func() (domain.TradeList, error) { return c.off.Trades(ctx, symbol, count) },
		func() (domain.TradeList, error) { return c.Client.GetTrades(ctx, symbol, count) })
}

// GetChart routes to off.Candles (no before-cursor, unadjusted).
func (c *Client) GetChart(ctx context.Context, symbol, interval string, count int) (domain.Chart, error) {
	return routeSymbol(c, symbol,
		func() (domain.Chart, error) { return c.off.Candles(ctx, symbol, interval, count, "", false) },
		func() (domain.Chart, error) { return c.Client.GetChart(ctx, symbol, interval, count) })
}

// GetPriceLimits routes to off.PriceLimits.
func (c *Client) GetPriceLimits(ctx context.Context, symbol string) (domain.PriceLimits, error) {
	return routeSymbol(c, symbol,
		func() (domain.PriceLimits, error) { return c.off.PriceLimits(ctx, symbol) },
		func() (domain.PriceLimits, error) { return c.Client.GetPriceLimits(ctx, symbol) })
}

// GetStockWarnings routes to off.Warnings.
func (c *Client) GetStockWarnings(ctx context.Context, symbol string) (domain.StockWarnings, error) {
	return routeSymbol(c, symbol,
		func() (domain.StockWarnings, error) { return c.off.Warnings(ctx, symbol) },
		func() (domain.StockWarnings, error) { return c.Client.GetStockWarnings(ctx, symbol) })
}

// GetSellableQuantity routes to off.SellableQuantity.
func (c *Client) GetSellableQuantity(ctx context.Context, symbol string) (domain.SellableQuantity, error) {
	return routeSymbol(c, symbol,
		func() (domain.SellableQuantity, error) { return c.off.SellableQuantity(ctx, symbol) },
		func() (domain.SellableQuantity, error) { return c.Client.GetSellableQuantity(ctx, symbol) })
}

// GetCommission routes to off.Commissions.
func (c *Client) GetCommission(ctx context.Context, symbol string) (domain.Commission, error) {
	return routeSymbol(c, symbol,
		func() (domain.Commission, error) { return c.off.Commissions(ctx, symbol) },
		func() (domain.Commission, error) { return c.Client.GetCommission(ctx, symbol) })
}

// ErrOfficialAccessRequired signals a feature that only the routed official
// Open API client can serve. Credentials may be missing, or policy may have
// deliberately disabled official access. cmd/tossctl converts it into a hint.
var ErrOfficialAccessRequired = errors.New("official Open API access required")

// officialOnly centralizes the policy gate and zero-value error contract for
// reads that have no WTS equivalent. Callers only describe the typed official
// operation; they cannot accidentally bypass a pinned-WTS policy.
func officialOnly[T any](c *Client, call func(*official.Client) (T, error)) (T, error) {
	off := c.Official()
	if off == nil {
		var zero T
		return zero, ErrOfficialAccessRequired
	}
	return call(off)
}

func officialOnlyDo(c *Client, call func(*official.Client) error) error {
	off := c.Official()
	if off == nil {
		return ErrOfficialAccessRequired
	}
	return call(off)
}

// BuyingPower serves the official cash buying-power read. The WTS account
// summary exposes different orderable-amount concepts, so this read must not
// fall back or silently substitute one for the other.
func (c *Client) BuyingPower(ctx context.Context, currency string) (domain.BuyingPower, error) {
	return officialOnly(c, func(off *official.Client) (domain.BuyingPower, error) {
		return off.BuyingPower(ctx, currency)
	})
}

// MarketCalendar serves the official trading-day calendar. WTS exposes a
// different monthly market-events calendar, so this read has no valid fallback.
func (c *Client) MarketCalendar(ctx context.Context, country, date string) (domain.TradingCalendar, error) {
	return officialOnly(c, func(off *official.Client) (domain.TradingCalendar, error) {
		return off.MarketCalendar(ctx, country, date)
	})
}

// Stocks serves the official batch stock-metadata read. WTS has quote and
// universe endpoints, but neither is an equivalent metadata contract, so this
// read must not fall back or silently substitute another dataset.
func (c *Client) Stocks(ctx context.Context, symbols []string) ([]domain.StockMetadata, error) {
	return officialOnly(c, func(off *official.Client) ([]domain.StockMetadata, error) {
		return off.Stocks(ctx, symbols)
	})
}

// Rankings serves the official /rankings ranking. official-only: no WTS
// fallback (WTS "popularity" ranking is a different dataset), so a missing key
// returns ErrOfficialAccessRequired rather than degrading.
func (c *Client) Rankings(ctx context.Context, typ, marketCountry, duration string, excludeCaution bool, count int) (domain.Ranking, error) {
	return officialOnly(c, func(off *official.Client) (domain.Ranking, error) {
		return off.Rankings(ctx, typ, marketCountry, duration, excludeCaution, count)
	})
}

// MarketIndicatorPrices serves official market-indicator current prices.
// official-only (see Rankings).
func (c *Client) MarketIndicatorPrices(ctx context.Context, symbols []string) (domain.MarketIndicatorPrices, error) {
	return officialOnly(c, func(off *official.Client) (domain.MarketIndicatorPrices, error) {
		return off.MarketIndicatorPrices(ctx, symbols)
	})
}

// MarketIndicatorCandles serves official market-indicator candles.
// official-only (see Rankings).
func (c *Client) MarketIndicatorCandles(ctx context.Context, symbol, interval string, count int, before string) (domain.MarketIndicatorCandles, error) {
	return officialOnly(c, func(off *official.Client) (domain.MarketIndicatorCandles, error) {
		return off.MarketIndicatorCandles(ctx, symbol, interval, count, before)
	})
}

// MarketInvestorTrading serves official market-wide investor trading.
// official-only (see Rankings).
func (c *Client) MarketInvestorTrading(ctx context.Context, symbol, interval string, count int, until string) (domain.InvestorTrading, error) {
	return officialOnly(c, func(off *official.Client) (domain.InvestorTrading, error) {
		return off.MarketInvestorTrading(ctx, symbol, interval, count, until)
	})
}

// ConditionalOrders serves the official conditional-order list. official-only.
func (c *Client) ConditionalOrders(ctx context.Context, status, symbol, cursor string, limit int) (domain.ConditionalOrderList, error) {
	return officialOnly(c, func(off *official.Client) (domain.ConditionalOrderList, error) {
		return off.ConditionalOrders(ctx, status, symbol, cursor, limit)
	})
}

// ConditionalOrder serves one official conditional order by id. official-only.
func (c *Client) ConditionalOrder(ctx context.Context, id string) (domain.ConditionalOrder, error) {
	return officialOnly(c, func(off *official.Client) (domain.ConditionalOrder, error) {
		return off.ConditionalOrder(ctx, id)
	})
}

// CancelConditionalOrder cancels a conditional order. official-only.
func (c *Client) CancelConditionalOrder(ctx context.Context, intent orderintent.ConditionalCancelIntent) error {
	return officialOnlyDo(c, func(off *official.Client) error {
		return off.CancelConditionalOrder(ctx, intent.ID)
	})
}

// CreateConditionalOrder creates a conditional order. official-only.
func (c *Client) CreateConditionalOrder(ctx context.Context, intent orderintent.ConditionalPlaceIntent) (domain.ConditionalOrderRef, error) {
	body := official.ConditionalCreateBody{
		Symbol: intent.Symbol, Type: string(intent.Type), Quantity: fmtDec(intent.Quantity),
		OrderType: string(intent.OrderType), ClientOrderID: intent.ClientOrderID, ExpireDate: intent.ExpireDate,
		First: officialLeg(intent.First), ConfirmHighValueOrder: intent.ConfirmHighValue,
	}
	if intent.Second != nil {
		s := officialLeg(*intent.Second)
		body.Second = &s
	}
	return officialOnly(c, func(off *official.Client) (domain.ConditionalOrderRef, error) {
		return off.CreateConditionalOrder(ctx, body)
	})
}

// ModifyConditionalOrder modifies a conditional order. official-only.
func (c *Client) ModifyConditionalOrder(ctx context.Context, intent orderintent.ConditionalModifyIntent) error {
	body := official.ConditionalModifyBody{
		Type: string(intent.Type), Quantity: fmtDec(intent.Quantity), OrderType: string(intent.OrderType),
		ExpireDate: intent.ExpireDate, First: officialLeg(intent.First), ConfirmHighValueOrder: intent.ConfirmHighValue,
	}
	if intent.Second != nil {
		s := officialLeg(*intent.Second)
		body.Second = &s
	}
	return officialOnlyDo(c, func(off *official.Client) error {
		return off.ModifyConditionalOrder(ctx, intent.ID, body)
	})
}

func fmtDec(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func officialLeg(l orderintent.ConditionLeg) official.ConditionLegBody {
	b := official.ConditionLegBody{OrderSide: l.OrderSide, TriggerPrice: fmtDec(l.TriggerPrice)}
	if l.OrderPrice > 0 {
		b.OrderPrice = fmtDec(l.OrderPrice)
	}
	return b
}

// --- Intentionally NOT overridden (served by embedded WTS) ------------------
//
// These are left to the embedded *client.Client because the official API does
// not map cleanly onto the WTS contract:
//
//   - GetAccountSummary: official exposes BuyingPower, a different type/concept
//     than the WTS account summary — not a drop-in substitute.
//   - (GetExchangeRates moved out of this list in 2026-08 — see the override
//     below. The blocker was the plural/singular signature, not semantics.)
//   - Order reads (ListPendingOrders / ListCompletedOrders / FindOrder): the
//     official status-enum mapping is still uncertain, so routing them risks
//     silently wrong statuses. Measured 2026-08-03: the official API answers
//     status=CLOSED with FILLED and CANCELED. The WTS side could not be
//     compared — the account had no completed orders in the current month, and
//     WTS passes its own server string through unmapped. Until both sides are
//     observed together, routing here could report a filled order as canceled,
//     which is worse than making the user pick a backend. See issue tracker.
//   - Order writes (PlacePendingOrder / CancelPendingOrder / AmendPendingOrder):
//     these are the Broker's concern and are handled in Task 10 — not here.

// Supply exposes the official supply series. There is no WTS equivalent for
// short-selling/credit/lending/program, and the one overlapping series
// (investor-trading) is routed by GetTradingFlows instead — so this is
// official-only and reports the shared access error when unavailable.
func (c *Client) Supply(ctx context.Context, symbol string, kind domain.SupplyKind, count int, until string) (domain.SupplySeries, error) {
	return officialOnly(c, func(off *official.Client) (domain.SupplySeries, error) {
		return off.Supply(ctx, symbol, kind, count, until)
	})
}

// GetTradingFlows prefers the official investor-trading series and falls back
// to WTS. The official payload is strictly richer (4 investor classes plus the
// institution breakdown, foreign holding, and CFD balance) where WTS carries
// three net figures — so when a key is present it is the better answer, and the
// WTS path stays as the no-key default it has always been.
func (c *Client) GetTradingFlows(ctx context.Context, symbol string, size int) (domain.TradingFlows, error) {
	return routeSymbol(c, symbol,
		func() (domain.TradingFlows, error) {
			s, err := c.off.Supply(ctx, symbol, domain.SupplyInvestor, size, "")
			if err != nil {
				return domain.TradingFlows{}, err
			}
			return supplyToFlows(symbol, s), nil
		},
		func() (domain.TradingFlows, error) { return c.Client.GetTradingFlows(ctx, symbol, size) })
}

// supplyToFlows narrows the official series to the shape `quote flows` has
// always printed. Nil (not yet tallied) becomes 0 here because domain.TradingFlow
// carries plain floats — callers who need the distinction should use
// `quote supply --type investor`, which keeps it.
func supplyToFlows(symbol string, s domain.SupplySeries) domain.TradingFlows {
	out := domain.TradingFlows{Symbol: symbol, FetchedAt: s.FetchedAt}
	for _, r := range s.Records {
		f := domain.TradingFlow{Date: r.Date}
		if r.Individual != nil {
			f.NetIndividuals = r.Individual.NetBuy
		}
		if r.Foreigner != nil {
			f.NetForeigner = r.Foreigner.NetBuy
		}
		if r.Institution != nil {
			f.NetInstitution = r.Institution.NetBuy
		}
		out.Flows = append(out.Flows, f)
	}
	return out
}

// ListStocks exposes the official universe endpoint. WTS has no equivalent —
// its catalogue surfaces are search-shaped, not enumerable.
func (c *Client) ListStocks(ctx context.Context, market, status, securityType string, commonShareOnly bool) (domain.StockUniverse, error) {
	return officialOnly(c, func(off *official.Client) (domain.StockUniverse, error) {
		return off.ListStocks(ctx, market, status, securityType, commonShareOnly)
	})
}
