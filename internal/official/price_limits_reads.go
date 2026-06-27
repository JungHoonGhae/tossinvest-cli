package official

import (
	"context"
	"net/url"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// apiPriceLimits mirrors the PriceLimitResponse schema.
// Endpoint: GET /api/v1/price-limits
// Schema (openapi.latest.json component "PriceLimitResponse"):
//
//	upperLimitPrice string (decimal, nullable) — daily upper price band (상한가)
//	lowerLimitPrice string (decimal, nullable) — daily lower price band (하한가)
//	currency        string — "KRW" | "USD"
//	timestamp       string (datetime, nullable)
//
// Note: US stocks have no daily price limits; these fields are null for them.
// encoding/json leaves a string field at its zero value ("") on JSON null,
// so parseDecimal correctly returns 0 for null/empty.
type apiPriceLimits struct {
	UpperLimitPrice string `json:"upperLimitPrice"`
	LowerLimitPrice string `json:"lowerLimitPrice"`
	Currency        string `json:"currency"`
	Timestamp       string `json:"timestamp"`
}

// PriceLimits fetches the daily upper/lower price band (상하한가) for symbol.
// US stocks have no limits; UpperLimit and LowerLimit will be 0 in that case.
func (c *Client) PriceLimits(ctx context.Context, symbol string) (domain.PriceLimits, error) {
	q := url.Values{}
	q.Set("symbol", symbol)
	var raw apiPriceLimits
	if err := c.get(ctx, "/api/v1/price-limits", q, &raw); err != nil {
		return domain.PriceLimits{}, err
	}
	return adaptPriceLimits(symbol, raw), nil
}

// adaptPriceLimits converts the official PriceLimitResponse to domain.PriceLimits.
//
// Mapping rationale:
//
//   - symbol (caller-supplied) → Symbol:
//     Not present in response; injected from the call parameter.
//
//   - upperLimitPrice (nullable decimal string) → UpperLimit:
//     parseDecimal returns 0 for null (unmarshaled as "") or empty.
//
//   - lowerLimitPrice (nullable decimal string) → LowerLimit:
//     Same treatment as upper.
//
//   - ProductCode, Name, Date: not available from /price-limits; left empty.
func adaptPriceLimits(symbol string, raw apiPriceLimits) domain.PriceLimits {
	return domain.PriceLimits{
		Symbol:     symbol,
		UpperLimit: parseDecimal(raw.UpperLimitPrice),
		LowerLimit: parseDecimal(raw.LowerLimitPrice),
		// ProductCode, Name, Date — not available from /price-limits response
	}
}
