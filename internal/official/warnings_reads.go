package official

import (
	"context"
	"encoding/json"
	"net/url"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// apiStockWarning mirrors one element of the Warnings array.
// Endpoint: GET /api/v1/stocks/{symbol}/warnings
// Schema:
//
//	warningType string — e.g. "INVESTMENT_WARNING"
//	startDate   string — "2026-03-26"
//	endDate     string — "2026-03-27"
//	exchange    string — "KRX"
type apiStockWarning struct {
	WarningType string `json:"warningType"`
	StartDate   string `json:"startDate"`
	EndDate     string `json:"endDate"`
	Exchange    string `json:"exchange"`
}

// Warnings fetches investment-warning badges for the given symbol.
// The path uses the symbol as a path segment: /api/v1/stocks/{symbol}/warnings.
func (c *Client) Warnings(ctx context.Context, symbol string) (domain.StockWarnings, error) {
	var raw []apiStockWarning
	if err := c.get(ctx, "/api/v1/stocks/"+url.PathEscape(symbol)+"/warnings", nil, &raw); err != nil {
		return domain.StockWarnings{}, err
	}
	return adaptWarnings(symbol, raw), nil
}

// adaptWarnings converts the official warnings array to domain.StockWarnings.
//
// Mapping rationale:
//
//   - symbol (caller-supplied) → Symbol.
//
//   - warnings[].warningType → StockWarning.Type:
//     Direct pass-through of the classification string.
//
//   - Title, Text, Level: not available from this endpoint; left empty.
//
//   - Raw: the full warning object is re-marshaled to json.RawMessage so callers
//     can access startDate/endDate/exchange without loss of information.
//
//   - ProductCode, Name: not available; left empty.
func adaptWarnings(symbol string, raw []apiStockWarning) domain.StockWarnings {
	warnings := make([]domain.StockWarning, 0, len(raw))
	for _, w := range raw {
		rawBytes, _ := json.Marshal(w)
		warnings = append(warnings, domain.StockWarning{
			Type: w.WarningType,
			Raw:  json.RawMessage(rawBytes),
			// Title, Text, Level — not provided by /warnings endpoint
		})
	}
	return domain.StockWarnings{
		Symbol:    symbol,
		Warnings:  warnings,
		FetchedAt: time.Now().UTC(),
		// ProductCode, Name — not available from /warnings response
	}
}
