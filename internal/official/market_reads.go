package official

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// parseDecimal parses a decimal string to float64. Returns 0 on empty/error.
// Used by all official-API adapters that receive numeric fields as strings.
func parseDecimal(s string) float64 {
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}

// ---------------------------------------------------------------------------
// ExchangeRate
// ---------------------------------------------------------------------------

// apiExchangeRate mirrors the ExchangeRateResponse schema.
// Endpoint: GET /api/v1/exchange-rate
// Schema (openapi.latest.json component "ExchangeRateResponse"):
//
//	baseCurrency   string — "USD" | "KRW"
//	quoteCurrency  string — "KRW" | "USD"
//	rate           string (decimal) — buying rate (1 base = ? quote)
//	midRate        string (decimal) — interbank mid rate (매매기준율)
//	basisPoint     string (decimal) — (rate - midRate) / midRate × 10000
//	rateChangeType string enum — "UP" | "EQUAL" | "DOWN"
//	validFrom       string (datetime)
//	validUntil      string (datetime)
type apiExchangeRate struct {
	BaseCurrency   string `json:"baseCurrency"`
	QuoteCurrency  string `json:"quoteCurrency"`
	Rate           string `json:"rate"`
	MidRate        string `json:"midRate"`
	BasisPoint     string `json:"basisPoint"`
	RateChangeType string `json:"rateChangeType"`
	ValidFrom      string `json:"validFrom"`
	ValidUntil     string `json:"validUntil"`
}

// ExchangeRate fetches the FX rate between base and quote currencies.
// Typical call: ExchangeRate(ctx, "USD", "KRW").
func (c *Client) ExchangeRate(ctx context.Context, base, quote string) (domain.ExchangeRate, error) {
	q := url.Values{}
	q.Set("baseCurrency", base)
	q.Set("quoteCurrency", quote)
	var raw apiExchangeRate
	if err := c.get(ctx, "/api/v1/exchange-rate", q, &raw); err != nil {
		return domain.ExchangeRate{}, err
	}
	return adaptExchangeRate(raw), nil
}

// adaptExchangeRate converts official ExchangeRateResponse to domain.ExchangeRate.
//
// Mapping rationale (cross-referenced with WTS exchange-rate feed):
//
//   - baseCurrency + "/" + quoteCurrency → Code:
//     WTS uses a currency code like "USD" in ExchangeRate.Code; the official
//     endpoint provides a pair. We build a conventional "USD/KRW" pair string.
//
//   - midRate (매매기준율, interbank mid) → Base:
//     WTS domain.ExchangeRate.Base is the "previous/reference rate" concept.
//     midRate is the closest analogue — it is the interbank reference.
//
//   - rate (매수 환율, buying rate) → Close:
//     WTS domain.ExchangeRate.Close is the last/current rate. The official
//     rate field (buying exchange rate) serves the same purpose.
//
//   - Name: not provided by /exchange-rate; left empty.
//   - basisPoint, rateChangeType, validFrom, validUntil: informational only;
//     not mapped to domain (no corresponding domain fields).
func adaptExchangeRate(raw apiExchangeRate) domain.ExchangeRate {
	return domain.ExchangeRate{
		Code:  raw.BaseCurrency + "/" + raw.QuoteCurrency,
		Base:  parseDecimal(raw.MidRate), // midRate → reference / previous base
		Close: parseDecimal(raw.Rate),    // rate (buying) → current close
		// Name — not available from /exchange-rate endpoint
	}
}

// ---------------------------------------------------------------------------
// Prices
// ---------------------------------------------------------------------------

// apiPrice mirrors the PriceResponse schema.
// Endpoint: GET /api/v1/prices
// Schema (openapi.latest.json component "PriceResponse"):
//
//	symbol    string — ticker (e.g. "005930", "AAPL")
//	lastPrice string (decimal) — current price
//	currency  string — "KRW" | "USD"
//	timestamp string (datetime, nullable) — data timestamp
type apiPrice struct {
	Symbol    string `json:"symbol"`
	LastPrice string `json:"lastPrice"`
	Currency  string `json:"currency"`
	Timestamp string `json:"timestamp"`
}

// Prices fetches the latest prices for a batch of symbols.
// symbols is joined as a comma-separated query parameter (max 200 per API spec).
func (c *Client) Prices(ctx context.Context, symbols []string) ([]domain.Quote, error) {
	q := url.Values{}
	q.Set("symbols", strings.Join(symbols, ","))
	var raw []apiPrice
	if err := c.get(ctx, "/api/v1/prices", q, &raw); err != nil {
		return nil, err
	}
	return adaptPrices(raw), nil
}

// adaptPrices converts a slice of official PriceResponse items to []domain.Quote.
//
// Mapping rationale (cross-referenced with internal/client/quote.go WTS adapter):
//
//   - symbol → Symbol:
//     WTS uses stockPriceResult.ProductCode; the official API provides the human
//     symbol directly (e.g. "005930", "AAPL").
//
//   - lastPrice (string decimal) → Last:
//     WTS stockPriceResult.Close → domain.Quote.Last. Same semantics.
//
//   - currency → Currency:
//     Direct pass-through, same as WTS.
//
//   - Name, Market, Change, Volume etc.: not available from /prices endpoint.
//     Use /stocks for metadata, /prices only for last price.
func adaptPrices(raw []apiPrice) []domain.Quote {
	out := make([]domain.Quote, 0, len(raw))
	for _, p := range raw {
		out = append(out, domain.Quote{
			Symbol:    p.Symbol,
			Last:      parseDecimal(p.LastPrice),
			Currency:  p.Currency,
			FetchedAt: time.Now().UTC(),
			// Name, MarketCode, Market, Change, Volume etc. — not in /prices
		})
	}
	return out
}

// ---------------------------------------------------------------------------
// Orderbook
// ---------------------------------------------------------------------------

// apiOrderbookEntry mirrors the OrderbookEntry schema.
// Schema: {price string (decimal), volume string (decimal)}
type apiOrderbookEntry struct {
	Price  string `json:"price"`
	Volume string `json:"volume"`
}

// apiOrderbook mirrors the OrderbookResponse schema.
// Endpoint: GET /api/v1/orderbook
// Schema (openapi.latest.json component "OrderbookResponse"):
//
//	asks      []OrderbookEntry — ask (매도) levels, ascending price order
//	bids      []OrderbookEntry — bid (매수) levels, descending price order
//	currency  string — "KRW" | "USD"
//	timestamp string (datetime, nullable)
type apiOrderbook struct {
	Asks      []apiOrderbookEntry `json:"asks"`
	Bids      []apiOrderbookEntry `json:"bids"`
	Currency  string              `json:"currency"`
	Timestamp string              `json:"timestamp"`
}

// Orderbook fetches the bid/ask depth ladder for a symbol.
func (c *Client) Orderbook(ctx context.Context, symbol string) (domain.OrderBook, error) {
	q := url.Values{}
	q.Set("symbol", symbol)
	var raw apiOrderbook
	if err := c.get(ctx, "/api/v1/orderbook", q, &raw); err != nil {
		return domain.OrderBook{}, err
	}
	return adaptOrderbook(symbol, raw), nil
}

// adaptOrderbook converts the official OrderbookResponse to domain.OrderBook.
//
// Mapping rationale (cross-referenced with internal/client quote-related WTS
// adapter fields):
//
//   - asks (매도호가) → Offers: WTS uses "offers" for the sell side.
//   - bids (매수호가) → Bids: direct pass-through.
//   - price/volume strings → parsed to float64 via parseDecimal.
//   - TotalOffer / TotalBid: summed from levels (not directly provided by API).
//   - symbol (caller-supplied) → Symbol; ProductCode/Name/Close not in response.
func adaptOrderbook(symbol string, raw apiOrderbook) domain.OrderBook {
	offers := make([]domain.OrderBookLevel, 0, len(raw.Asks))
	var totalOffer float64
	for _, a := range raw.Asks {
		vol := parseDecimal(a.Volume)
		totalOffer += vol
		offers = append(offers, domain.OrderBookLevel{
			Price:  parseDecimal(a.Price),
			Volume: vol,
		})
	}

	bids := make([]domain.OrderBookLevel, 0, len(raw.Bids))
	var totalBid float64
	for _, b := range raw.Bids {
		vol := parseDecimal(b.Volume)
		totalBid += vol
		bids = append(bids, domain.OrderBookLevel{
			Price:  parseDecimal(b.Price),
			Volume: vol,
		})
	}

	return domain.OrderBook{
		Symbol:     symbol,
		Offers:     offers,
		Bids:       bids,
		TotalOffer: totalOffer,
		TotalBid:   totalBid,
		FetchedAt:  time.Now().UTC(),
		// ProductCode, Name, Close — not provided by /orderbook response
	}
}

// ---------------------------------------------------------------------------
// Trades
// ---------------------------------------------------------------------------

// apiTrade mirrors the Trade schema.
// Endpoint: GET /api/v1/trades
// Schema (openapi.latest.json component "Trade"):
//
//	price     string (decimal) — execution price
//	volume    string (decimal) — execution volume
//	timestamp string (datetime) — execution timestamp
//	currency  string — "KRW" | "USD"
type apiTrade struct {
	Price     string `json:"price"`
	Volume    string `json:"volume"`
	Timestamp string `json:"timestamp"`
	Currency  string `json:"currency"`
}

// Trades fetches recent executed ticks (체결 내역) for a symbol.
// count is optional; pass 0 to use the API default (50). Max is 50.
func (c *Client) Trades(ctx context.Context, symbol string, count int) (domain.TradeList, error) {
	q := url.Values{}
	q.Set("symbol", symbol)
	if count > 0 {
		q.Set("count", strconv.Itoa(count))
	}
	var raw []apiTrade
	if err := c.get(ctx, "/api/v1/trades", q, &raw); err != nil {
		return domain.TradeList{}, err
	}
	return adaptTrades(symbol, raw), nil
}

// adaptTrades converts a slice of official Trade items to domain.TradeList.
//
// Mapping rationale (cross-referenced with WTS domain.Trade):
//
//   - timestamp → Time: WTS domain.Trade.Time is a string; we keep it as-is.
//   - price (string decimal) → Price: parseDecimal to float64, matches WTS.
//   - volume (string decimal) → Volume: same treatment.
//   - TradeType (BUY/SELL): not in official Trade schema; left empty.
//   - Base, CumulativeVolume: not in official Trade schema; left zero.
//   - ProductCode, Name: not in /trades response; left empty.
func adaptTrades(symbol string, raw []apiTrade) domain.TradeList {
	trades := make([]domain.Trade, 0, len(raw))
	for _, t := range raw {
		trades = append(trades, domain.Trade{
			Time:   t.Timestamp,
			Price:  parseDecimal(t.Price),
			Volume: parseDecimal(t.Volume),
			// TradeType, Base, CumulativeVolume — not in official Trade schema
		})
	}
	return domain.TradeList{
		Symbol:    symbol,
		Trades:    trades,
		FetchedAt: time.Now().UTC(),
		// ProductCode, Name — not in /trades response
	}
}

// ---------------------------------------------------------------------------
// Rankings
// ---------------------------------------------------------------------------

// apiRankingPrice mirrors RankingPrice: {lastPrice, basePrice, changeRate?}.
type apiRankingPrice struct {
	LastPrice  string `json:"lastPrice"`
	BasePrice  string `json:"basePrice"`
	ChangeRate string `json:"changeRate"` // nullable → "" when null
}

// apiRankingItem mirrors RankingItem in the official /rankings response.
type apiRankingItem struct {
	Rank          int             `json:"rank"`
	Symbol        string          `json:"symbol"`
	Currency      string          `json:"currency"`
	Price         apiRankingPrice `json:"price"`
	TradingVolume string          `json:"tradingVolume"`
	TradingAmount string          `json:"tradingAmount"`
}

// apiRankingResult mirrors RankingResponse (the unwrapped `result`).
type apiRankingResult struct {
	RankedAt string           `json:"rankedAt"` // nullable → "" when null
	Rankings []apiRankingItem `json:"rankings"`
}

// Rankings fetches the official stock ranking.
// typ: MARKET_TRADING_AMOUNT | MARKET_TRADING_VOLUME | TOP_GAINERS | TOP_LOSERS |
//
//	TOSS_SECURITIES_TRADING_AMOUNT | TOSS_SECURITIES_TRADING_VOLUME
//
// marketCountry: KR | US. duration: realtime|1d|1w|1mo|3mo|6mo|1y.
// count is optional (0 = API default); max 100 per spec.
func (c *Client) Rankings(ctx context.Context, typ, marketCountry, duration string, excludeCaution bool, count int) (domain.Ranking, error) {
	q := url.Values{}
	q.Set("type", typ)
	q.Set("marketCountry", marketCountry)
	q.Set("duration", duration)
	if excludeCaution {
		q.Set("excludeInvestmentCaution", "true")
	}
	if count > 0 {
		q.Set("count", strconv.Itoa(count))
	}
	var raw apiRankingResult
	if err := c.get(ctx, "/api/v1/rankings", q, &raw); err != nil {
		return domain.Ranking{}, err
	}
	return adaptRanking(typ, marketCountry, duration, raw), nil
}

// adaptRanking converts the official RankingResponse to domain.Ranking.
// The type/market/duration are echoed from the request (the response body does
// not repeat them). Decimal strings → float64 via parseDecimal; a null
// changeRate/rankedAt arrives as "" and maps to 0 / "".
func adaptRanking(typ, marketCountry, duration string, raw apiRankingResult) domain.Ranking {
	items := make([]domain.RankingItem, 0, len(raw.Rankings))
	for _, r := range raw.Rankings {
		items = append(items, domain.RankingItem{
			Rank:          r.Rank,
			Symbol:        r.Symbol,
			Currency:      r.Currency,
			LastPrice:     parseDecimal(r.Price.LastPrice),
			BasePrice:     parseDecimal(r.Price.BasePrice),
			ChangeRate:    parseDecimal(r.Price.ChangeRate),
			TradingVolume: parseDecimal(r.TradingVolume),
			TradingAmount: parseDecimal(r.TradingAmount),
		})
	}
	return domain.Ranking{
		Type:          typ,
		MarketCountry: marketCountry,
		Duration:      duration,
		RankedAt:      raw.RankedAt,
		Items:         items,
		FetchedAt:     time.Now().UTC(),
	}
}

// ---------------------------------------------------------------------------
// Market indicator prices
// ---------------------------------------------------------------------------

// apiMarketIndicatorPrice mirrors MarketIndicatorPriceResponse.
type apiMarketIndicatorPrice struct {
	Symbol    string `json:"symbol"`
	Timestamp string `json:"timestamp"` // nullable → "" when null
	LastPrice string `json:"lastPrice"`
}

// MarketIndicatorPrices fetches current prices for market indicators (지수 등).
// symbols is comma-joined; max 200 per spec. Only catalog symbols (e.g. KOSPI,
// KOSDAQ) are supported by the API.
func (c *Client) MarketIndicatorPrices(ctx context.Context, symbols []string) (domain.MarketIndicatorPrices, error) {
	q := url.Values{}
	q.Set("symbols", strings.Join(symbols, ","))
	var raw []apiMarketIndicatorPrice
	if err := c.get(ctx, "/api/v1/market-indicators/prices", q, &raw); err != nil {
		return domain.MarketIndicatorPrices{}, err
	}
	return adaptMarketIndicatorPrices(raw), nil
}

// adaptMarketIndicatorPrices converts the official response array to domain.
func adaptMarketIndicatorPrices(raw []apiMarketIndicatorPrice) domain.MarketIndicatorPrices {
	out := make([]domain.MarketIndicatorPrice, 0, len(raw))
	for _, p := range raw {
		out = append(out, domain.MarketIndicatorPrice{
			Symbol:    p.Symbol,
			LastPrice: parseDecimal(p.LastPrice),
			Timestamp: p.Timestamp,
		})
	}
	return domain.MarketIndicatorPrices{Indicators: out, FetchedAt: time.Now().UTC()}
}
