package official

import (
	"context"
	"net/url"
	"strconv"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// apiCandle mirrors the Candle schema returned inside CandlePageResponse.
// Endpoint: GET /api/v1/candles
// Schema (openapi.latest.json component "CandlePageResponse"):
//
//	candles[].timestamp  string (datetime, RFC3339) — bar open time
//	candles[].openPrice  string (decimal)
//	candles[].highPrice  string (decimal)
//	candles[].lowPrice   string (decimal)
//	candles[].closePrice string (decimal)
//	candles[].volume     string (decimal)
//	candles[].currency   string — "KRW" | "USD"
type apiCandle struct {
	Timestamp  string `json:"timestamp"`
	OpenPrice  string `json:"openPrice"`
	HighPrice  string `json:"highPrice"`
	LowPrice   string `json:"lowPrice"`
	ClosePrice string `json:"closePrice"`
	Volume     string `json:"volume"`
	Currency   string `json:"currency"`
}

// apiCandlePage mirrors the CandlePageResponse schema.
// nextBefore is the cursor for the previous page (older candles).
type apiCandlePage struct {
	Candles    []apiCandle `json:"candles"`
	NextBefore string      `json:"nextBefore"`
}

// Candles fetches OHLCV bars for symbol at the given interval.
//
// interval must be "1m" (minute) or "1d" (daily).
// count controls how many candles to return (default 100, max 200); pass 0 to
// use the API default.
// before is an optional datetime string cursor for pagination; pass "" to start
// from the most recent candle.
// adjusted controls split/dividend adjustment; pass true to use the API default.
func (c *Client) Candles(ctx context.Context, symbol, interval string, count int, before string, adjusted bool) (domain.Chart, error) {
	q := url.Values{}
	q.Set("symbol", symbol)
	q.Set("interval", interval)
	if count > 0 {
		q.Set("count", strconv.Itoa(count))
	}
	if before != "" {
		q.Set("before", before)
	}
	q.Set("adjusted", strconv.FormatBool(adjusted))

	var raw apiCandlePage
	if err := c.get(ctx, "/api/v1/candles", q, &raw); err != nil {
		return domain.Chart{}, err
	}
	return adaptCandles(symbol, interval, raw), nil
}

// adaptCandles converts the official CandlePageResponse to domain.Chart.
//
// Mapping rationale:
//
//   - symbol (caller-supplied) → Symbol:
//     Not present in response; injected from the call parameter.
//
//   - interval (caller-supplied) → Interval:
//     Not present in response; injected from the call parameter.
//
//   - candles[].timestamp (RFC3339) → Candle.Time:
//     Parsed via time.Parse(time.RFC3339, ...). Zero time on parse failure.
//
//   - candles[].openPrice/highPrice/lowPrice/closePrice (decimal strings)
//     → Candle.Open/High/Low/Close: parsed via parseDecimal.
//
//   - candles[].volume (decimal string) → Candle.Volume: parseDecimal.
//
//   - ProductCode, Name, Base: not available from /candles; left zero/empty.
func adaptCandles(symbol, interval string, raw apiCandlePage) domain.Chart {
	candles := make([]domain.Candle, 0, len(raw.Candles))
	for _, c := range raw.Candles {
		var t time.Time
		if c.Timestamp != "" {
			if parsed, err := time.Parse(time.RFC3339, c.Timestamp); err == nil {
				t = parsed
			}
		}
		candles = append(candles, domain.Candle{
			Time:   t,
			Open:   parseDecimal(c.OpenPrice),
			High:   parseDecimal(c.HighPrice),
			Low:    parseDecimal(c.LowPrice),
			Close:  parseDecimal(c.ClosePrice),
			Volume: parseDecimal(c.Volume),
		})
	}
	return domain.Chart{
		Symbol:    symbol,
		Interval:  interval,
		Candles:   candles,
		FetchedAt: time.Now().UTC(),
		// ProductCode, Name, Base — not available from /candles response
	}
}
