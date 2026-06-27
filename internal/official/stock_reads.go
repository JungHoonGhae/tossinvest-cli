package official

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// apiStockInfo mirrors the StockInfo schema.
// Endpoint: GET /api/v1/stocks
// Schema (openapi.latest.json component "StockInfo"):
//
//	symbol          string — ticker (e.g. "005930", "AAPL")
//	name            string — Korean name
//	englishName     string — English name
//	isinCode        string — ISO 6166 ISIN
//	market          string enum — "KOSPI"|"KOSDAQ"|"NYSE"|"NASDAQ"|"AMEX"|"KR_ETC"|"US_ETC"
//	securityType    string enum — "STOCK"|"ETF"|...
//	isCommonShare   bool
//	status          string enum — "SCHEDULED"|"ACTIVE"|"DELISTED"
//	currency        string — "KRW" | "USD"
//	sharesOutstanding string (decimal)
//	listDate        string (date, nullable)
//	delistDate      string (date, nullable)
//	leverageFactor  string (decimal, nullable)
//	koreanMarketDetail object (KR only, nullable)
type apiStockInfo struct {
	Symbol         string `json:"symbol"`
	Name           string `json:"name"`
	EnglishName    string `json:"englishName"`
	Market         string `json:"market"`
	Currency       string `json:"currency"`
	Status         string `json:"status"`
	SecurityType   string `json:"securityType"`
	IsCommonShare  bool   `json:"isCommonShare"`
	IsinCode       string `json:"isinCode"`
}

// Stocks fetches basic stock metadata for a batch of symbols.
// symbols is joined as comma-separated query parameter (max 200 per API spec).
func (c *Client) Stocks(ctx context.Context, symbols []string) ([]domain.Quote, error) {
	q := url.Values{}
	q.Set("symbols", strings.Join(symbols, ","))
	var raw []apiStockInfo
	if err := c.get(ctx, "/api/v1/stocks", q, &raw); err != nil {
		return nil, err
	}
	return adaptStocks(raw), nil
}

// adaptStocks converts a slice of official StockInfo records to []domain.Quote.
//
// Mapping rationale (cross-referenced with internal/client/quote.go WTS adapter):
//
//   - symbol → Symbol:
//     WTS getStockInfo returns stockInfoResult.Symbol. Same concept.
//
//   - name → Name:
//     WTS stockInfoResult.Name (Korean name). Official also provides name (Korean).
//
//   - market (exchange segment) → MarketCode:
//     WTS stockInfoResult.Market.Code → domain.Quote.MarketCode (e.g. "KOSPI").
//     Official "market" field carries the same exchange segment string.
//
//   - currency → Currency:
//     WTS uses firstNonEmpty(price.Currency, info.Currency). Official provides it
//     directly in StockInfo.
//
//   - status → Status:
//     WTS stockInfoResult.Status → domain.Quote.Status.
//
//   - Last, Change, Volume, Open/High/Low etc.: not available from /stocks.
//     Call /prices or /prices+/stocks in combination for full quote.
//   - Market (display name): not available; MarketCode is the enum value.
//   - ProductCode, ReferencePrice etc.: not in /stocks.
func adaptStocks(raw []apiStockInfo) []domain.Quote {
	out := make([]domain.Quote, 0, len(raw))
	for _, s := range raw {
		out = append(out, domain.Quote{
			Symbol:     s.Symbol,
			Name:       s.Name,
			MarketCode: s.Market,
			Currency:   s.Currency,
			Status:     s.Status,
			FetchedAt:  time.Now().UTC(),
			// Last, Change, Volume, Open/High/Low — not in /stocks
			// Market (display name) — not in /stocks
			// ProductCode — not in official stocks endpoint
		})
	}
	return out
}
