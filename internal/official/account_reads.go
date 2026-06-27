package official

import (
	"context"
	"net/url"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// apiBuyingPower mirrors the BuyingPowerResponse schema.
// Endpoint: GET /api/v1/buying-power
// Schema (openapi.latest.json component "BuyingPowerResponse"):
//
//	cashBuyingPower string (decimal) — cash buying power in the given currency
//	currency        string           — "KRW" or "USD"
type apiBuyingPower struct {
	CashBuyingPower string `json:"cashBuyingPower"`
	Currency        string `json:"currency"`
}

// BuyingPower fetches the cash-based buying power for the given currency.
// currency must be "KRW" or "USD" as defined by the official API.
//
// Note: the official API also requires an X-Tossinvest-Account header.
// Configure the account with WithAccountSeq when constructing the Client.
func (c *Client) BuyingPower(ctx context.Context, currency string) (domain.BuyingPower, error) {
	q := url.Values{}
	q.Set("currency", currency)
	var raw apiBuyingPower
	if err := c.getAcct(ctx, "/api/v1/buying-power", q, &raw); err != nil {
		return domain.BuyingPower{}, err
	}
	return adaptBuyingPower(raw), nil
}

// adaptBuyingPower converts the official API shape to domain.BuyingPower.
//
// Mapping rationale:
//   - cashBuyingPower (string decimal) → CashBuyingPower (float64):
//     The WTS adapter stores orderable amounts as float64 (pointerFloat(KRW/USD)).
//     We parse the decimal string to float64 (via the package-shared parseDecimal)
//     for consistency with the other read adapters.
//   - currency (string enum) → Currency:
//     Direct pass-through ("KRW" or "USD").
func adaptBuyingPower(raw apiBuyingPower) domain.BuyingPower {
	return domain.BuyingPower{
		Currency:        raw.Currency,
		CashBuyingPower: parseDecimal(raw.CashBuyingPower),
	}
}
