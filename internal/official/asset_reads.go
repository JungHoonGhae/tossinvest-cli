package official

import (
	"context"
	"net/url"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// apiHoldingsItem mirrors the HoldingsItem schema.
// Endpoint: GET /api/v1/holdings (within HoldingsOverview.items)
// Schema (openapi.latest.json component "HoldingsItem"):
//
//	symbol               string — ticker
//	name                 string — Korean name
//	marketCountry        string — "KR" | "US"
//	currency             string — "KRW" | "USD"
//	quantity             string (decimal)
//	lastPrice            string (decimal) — current price in trading currency
//	averagePurchasePrice string (decimal) — average buy price
//	marketValue.amount   string (decimal) — current market value
//	profitLoss.amount    string (decimal) — unrealized P&L
//	profitLoss.rate      string (decimal) — unrealized P&L rate (e.g. 0.1077 = 10.77%)
//	dailyProfitLoss.amount string (decimal) — daily P&L
//	dailyProfitLoss.rate   string (decimal) — daily P&L rate
type apiHoldingsItem struct {
	Symbol               string `json:"symbol"`
	Name                 string `json:"name"`
	MarketCountry        string `json:"marketCountry"`
	Currency             string `json:"currency"`
	Quantity             string `json:"quantity"`
	LastPrice            string `json:"lastPrice"`
	AveragePurchasePrice string `json:"averagePurchasePrice"`
	MarketValue          struct {
		Amount          string `json:"amount"`
		AmountAfterCost string `json:"amountAfterCost"`
		PurchaseAmount  string `json:"purchaseAmount"`
	} `json:"marketValue"`
	ProfitLoss struct {
		Amount          string `json:"amount"`
		AmountAfterCost string `json:"amountAfterCost"`
		Rate            string `json:"rate"`
		RateAfterCost   string `json:"rateAfterCost"`
	} `json:"profitLoss"`
	DailyProfitLoss struct {
		Amount string `json:"amount"`
		Rate   string `json:"rate"`
	} `json:"dailyProfitLoss"`
}

// apiHoldingsOverview mirrors the HoldingsOverview schema (outer envelope).
// We only extract the items slice; aggregate totals are not mapped to domain.
type apiHoldingsOverview struct {
	Items []apiHoldingsItem `json:"items"`
}

// Holdings fetches the authenticated user's stock positions.
// symbol is optional: when non-empty, only that symbol is returned.
// Pass an empty string to retrieve all holdings.
//
// Note: the official API also requires an X-Tossinvest-Account header.
// Configure the account with WithAccountSeq when constructing the Client.
func (c *Client) Holdings(ctx context.Context, symbol string) ([]domain.Position, error) {
	q := url.Values{}
	if symbol != "" {
		q.Set("symbol", symbol)
	}
	var raw apiHoldingsOverview
	if err := c.getAcct(ctx, "/api/v1/holdings", q, &raw); err != nil {
		return nil, err
	}
	return adaptHoldings(raw.Items), nil
}

// adaptHoldings converts a slice of official HoldingsItem records to
// []domain.Position.
//
// Mapping rationale (cross-referenced with internal/client/portfolio.go WTS adapter):
//
//   - symbol → Symbol:
//     WTS uses stockSymbol (or stockCode). Official provides the public symbol.
//
//   - name → Name:
//     Direct pass-through, same as WTS stockName.
//
//   - marketCountry ("KR"/"US") → MarketType:
//     WTS uses product.MarketType (e.g. "KR", "US"). Official marketCountry
//     serves the same purpose; we map it directly.
//
//   - quantity (string decimal) → Quantity:
//     WTS uses item.Quantity (already float64). We parseDecimal.
//
//   - averagePurchasePrice → AveragePrice:
//     WTS coalesceMoney(purchasePrice.KRW, purchasePrice.USD). Official provides
//     a single currency-specific value.
//
//   - lastPrice → CurrentPrice:
//     WTS coalesceMoney(currentPrice.KRW, currentPrice.USD).
//
//   - marketValue.amount → MarketValue:
//     WTS evaluatedAmount.
//
//   - profitLoss.amount → UnrealizedPnL:
//     WTS profitLossAmount.
//
//   - profitLoss.rate → ProfitRate:
//     WTS profitLossRate. Stored as decimal ratio (0.1077 = 10.77%).
//
//   - dailyProfitLoss.amount → DailyProfitLoss:
//     WTS dailyProfitLossAmount.
//
//   - dailyProfitLoss.rate → DailyProfitRate:
//     WTS dailyProfitLossRate.
//
//   - ProductCode, MarketCode: not available from /holdings; left empty.
//
//   - USD-specific fields (AveragePriceUSD etc.): not individually provided;
//     left zero (official uses a single-currency HoldingsItem).
func adaptHoldings(items []apiHoldingsItem) []domain.Position {
	out := make([]domain.Position, 0, len(items))
	for _, item := range items {
		out = append(out, domain.Position{
			Symbol:          item.Symbol,
			Name:            item.Name,
			MarketType:      item.MarketCountry,
			Quantity:        parseDecimal(item.Quantity),
			AveragePrice:    parseDecimal(item.AveragePurchasePrice),
			CurrentPrice:    parseDecimal(item.LastPrice),
			MarketValue:     parseDecimal(item.MarketValue.Amount),
			UnrealizedPnL:   parseDecimal(item.ProfitLoss.Amount),
			ProfitRate:      parseDecimal(item.ProfitLoss.Rate),
			DailyProfitLoss: parseDecimal(item.DailyProfitLoss.Amount),
			DailyProfitRate: parseDecimal(item.DailyProfitLoss.Rate),
			// ProductCode, MarketCode — not in official holdings response
			// AveragePriceUSD, CurrentPriceUSD, etc. — single-currency per item
		})
	}
	return out
}
