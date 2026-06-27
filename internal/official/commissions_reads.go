package official

import (
	"context"
	"net/url"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// apiCommission mirrors one element of the Commissions array.
// Endpoint: GET /api/v1/commissions
// Schema:
//
//	marketCountry  string — "KR" | "US"
//	commissionRate string (decimal) — e.g. "0.015" (1.5%)
//	startDate      string — effective from date
//	endDate        string — effective until date
type apiCommission struct {
	MarketCountry  string `json:"marketCountry"`
	CommissionRate string `json:"commissionRate"`
	StartDate      string `json:"startDate"`
	EndDate        string `json:"endDate"`
}

// Commissions fetches the commission rate schedule applicable to symbol.
// Returns the first element of the array as a single domain.Commission value.
// If the response is empty, returns a zero-value Commission with FetchedAt set.
func (c *Client) Commissions(ctx context.Context, symbol string) (domain.Commission, error) {
	q := url.Values{}
	q.Set("symbol", symbol)
	var raw []apiCommission
	if err := c.get(ctx, "/api/v1/commissions", q, &raw); err != nil {
		return domain.Commission{}, err
	}
	return adaptCommissions(symbol, raw), nil
}

// adaptCommissions converts the official Commissions array to domain.Commission.
//
// Mapping rationale:
//
//   - symbol (caller-supplied) → Symbol.
//   - raw[0].commissionRate (decimal string) → CommissionRate: parseDecimal.
//     Only the first element is used; the API returns per-period entries but the
//     domain type represents a single current rate.
//   - TaxRate: not included in this endpoint's response; left 0.
//   - ProductCode, Name: not available; left empty.
func adaptCommissions(symbol string, raw []apiCommission) domain.Commission {
	var rate float64
	if len(raw) > 0 {
		rate = parseDecimal(raw[0].CommissionRate)
	}
	return domain.Commission{
		Symbol:         symbol,
		CommissionRate: rate,
		FetchedAt:      time.Now().UTC(),
		// TaxRate — not in /commissions response
		// ProductCode, Name — not available
	}
}
