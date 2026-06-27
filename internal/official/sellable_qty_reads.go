package official

import (
	"context"
	"net/url"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// apiSellableQuantity mirrors the SellableQuantityResponse schema.
// Endpoint: GET /api/v1/sellable-quantity (account-scoped)
// Schema (openapi.latest.json component "SellableQuantityResponse"):
//
//	sellableQuantity string (decimal) — shares available to sell now
type apiSellableQuantity struct {
	SellableQuantity string `json:"sellableQuantity"`
}

// SellableQuantity fetches how many shares of symbol can be sold immediately.
// Requires the X-Tossinvest-Account header; configure via WithAccountSeq.
func (c *Client) SellableQuantity(ctx context.Context, symbol string) (domain.SellableQuantity, error) {
	q := url.Values{}
	q.Set("symbol", symbol)
	var raw apiSellableQuantity
	if err := c.getAcct(ctx, "/api/v1/sellable-quantity", q, &raw); err != nil {
		return domain.SellableQuantity{}, err
	}
	return adaptSellableQuantity(symbol, raw), nil
}

// adaptSellableQuantity converts the official SellableQuantityResponse to
// domain.SellableQuantity.
//
// Mapping rationale:
//
//   - symbol (caller-supplied) → Symbol.
//   - sellableQuantity (decimal string) → Quantity: parseDecimal.
//   - ProductCode, Name: not available; left empty.
func adaptSellableQuantity(symbol string, raw apiSellableQuantity) domain.SellableQuantity {
	return domain.SellableQuantity{
		Symbol:    symbol,
		Quantity:  parseDecimal(raw.SellableQuantity),
		FetchedAt: time.Now().UTC(),
		// ProductCode, Name — not available from /sellable-quantity response
	}
}
