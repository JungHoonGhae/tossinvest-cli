package official

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

var stockSymbolPattern = regexp.MustCompile(`^[A-Za-z0-9.-]+$`)

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
	Symbol             string                 `json:"symbol"`
	Name               string                 `json:"name"`
	EnglishName        string                 `json:"englishName"`
	Market             string                 `json:"market"`
	Currency           string                 `json:"currency"`
	Status             string                 `json:"status"`
	SecurityType       string                 `json:"securityType"`
	IsCommonShare      bool                   `json:"isCommonShare"`
	IsinCode           string                 `json:"isinCode"`
	SharesOutstanding  string                 `json:"sharesOutstanding"`
	ListDate           *string                `json:"listDate"`
	DelistDate         *string                `json:"delistDate"`
	LeverageFactor     *string                `json:"leverageFactor"`
	KoreanMarketDetail *apiKoreanMarketDetail `json:"koreanMarketDetail"`
}

type apiKoreanMarketDetail struct {
	LiquidationTrading  bool  `json:"liquidationTrading"`
	NXTSupported        bool  `json:"nxtSupported"`
	KRXTradingSuspended bool  `json:"krxTradingSuspended"`
	NXTTradingSuspended *bool `json:"nxtTradingSuspended"`
}

// Stocks fetches basic stock metadata for a batch of symbols.
// symbols is joined as comma-separated query parameter (max 200 per API spec).
func (c *Client) Stocks(ctx context.Context, symbols []string) ([]domain.StockMetadata, error) {
	if len(symbols) == 0 {
		return nil, fmt.Errorf("stocks requires at least one symbol")
	}
	if len(symbols) > 200 {
		return nil, fmt.Errorf("stocks accepts at most 200 symbols (got %d)", len(symbols))
	}
	normalized := make([]string, len(symbols))
	for i, symbol := range symbols {
		symbol = strings.TrimSpace(symbol)
		if !stockSymbolPattern.MatchString(symbol) {
			return nil, fmt.Errorf("invalid stock symbol %q (letters, numbers, '.', and '-' only)", symbols[i])
		}
		normalized[i] = symbol
	}
	q := url.Values{}
	q.Set("symbols", strings.Join(normalized, ","))
	var raw []apiStockInfo
	if err := c.get(ctx, "/api/v1/stocks", q, &raw); err != nil {
		return nil, err
	}
	return adaptStocks(raw), nil
}

// adaptStocks converts official StockInfo records without discarding metadata.
//
// Unlike a quote, StockMetadata preserves the full reference-data response:
// identity, listing lifecycle, security classification, exact share counts,
// and the Korean KRX/NXT flags. Price fields do not belong to this endpoint.
func adaptStocks(raw []apiStockInfo) []domain.StockMetadata {
	out := make([]domain.StockMetadata, 0, len(raw))
	for _, s := range raw {
		metadata := domain.StockMetadata{
			Symbol: s.Symbol, Name: s.Name, EnglishName: s.EnglishName,
			ISINCode: s.IsinCode, MarketCode: s.Market, SecurityType: s.SecurityType,
			CommonShare: s.IsCommonShare, Status: s.Status, Currency: s.Currency,
			SharesOutstanding: s.SharesOutstanding, ListDate: s.ListDate,
			DelistDate: s.DelistDate, LeverageFactor: s.LeverageFactor,
			FetchedAt: time.Now().UTC(),
		}
		if s.KoreanMarketDetail != nil {
			metadata.KoreanMarketDetail = &domain.KoreanMarketDetail{
				LiquidationTrading:  s.KoreanMarketDetail.LiquidationTrading,
				NXTSupported:        s.KoreanMarketDetail.NXTSupported,
				KRXTradingSuspended: s.KoreanMarketDetail.KRXTradingSuspended,
				NXTTradingSuspended: s.KoreanMarketDetail.NXTTradingSuspended,
			}
		}
		out = append(out, metadata)
	}
	return out
}
