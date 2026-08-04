package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

type quoteEnvelope[T any] struct {
	Result T `json:"result"`
}

type stockInfoResult struct {
	Symbol   string `json:"symbol"`
	Name     string `json:"name"`
	Currency string `json:"currency"`
	Status   string `json:"status"`
	Market   struct {
		Code        string `json:"code"`
		DisplayName string `json:"displayName"`
	} `json:"market"`
}

type stockDetailCommonResult struct {
	Badges  []json.RawMessage `json:"badges"`
	Notices []json.RawMessage `json:"notices"`
}

type stockPriceResult struct {
	ProductCode string  `json:"productCode"`
	Exchange    string  `json:"exchange"`
	Currency    string  `json:"currency"`
	Base        float64 `json:"base"`
	Close       float64 `json:"close"`
	Volume      float64 `json:"volume"`
}

// stockPriceDetailResult is the richer v3 details payload used to enrich
// `quote get` (OHLC, 52-week high/low, market cap, trading value/strength).
type stockPriceDetailResult struct {
	Code            string  `json:"code"`
	Open            float64 `json:"open"`
	High            float64 `json:"high"`
	Low             float64 `json:"low"`
	Close           float64 `json:"close"`
	PreDayVolume    float64 `json:"preDayVolume"`
	High52w         float64 `json:"high52w"`
	Low52w          float64 `json:"low52w"`
	MarketCap       float64 `json:"marketCap"`
	Value           float64 `json:"value"`           // 거래대금
	TradingStrength float64 `json:"tradingStrength"` // 체결강도
	UpperLimit      float64 `json:"upperLimit"`
	LowerLimit      float64 `json:"lowerLimit"`
}

type stockSearchEnvelope struct {
	Result struct {
		Stocks []struct {
			StockCode string `json:"stockCode"`
			StockName string `json:"stockName"`
			MatchType string `json:"matchType"`
		} `json:"stocks"`
	} `json:"result"`
}

func (c *Client) GetQuote(ctx context.Context, symbol string) (domain.Quote, error) {
	productCode, err := c.resolveProductCode(ctx, symbol)
	if err != nil {
		return domain.Quote{}, err
	}

	// The four per-product lookups below all depend only on productCode and are
	// independent of each other, so we fan them out concurrently — turning four
	// sequential round-trips into roughly one. info/price errors are fatal;
	// detail (badges/notices) and the v3 details enrichment are non-fatal.
	var (
		info     stockInfoResult
		infoErr  error
		price    stockPriceResult
		priceErr error
		detail   *stockDetailCommonResult
		details  *stockPriceDetailResult
		wg       sync.WaitGroup
	)
	wg.Add(4)
	go func() { defer wg.Done(); info, infoErr = c.getStockInfo(ctx, productCode) }()
	go func() { defer wg.Done(); price, priceErr = c.getStockPrice(ctx, productCode) }()
	go func() {
		defer wg.Done()
		if d, err := c.getStockDetailCommon(ctx, productCode); err == nil {
			detail = d
		}
	}()
	go func() {
		defer wg.Done()
		if d, err := c.getStockPriceDetails(ctx, productCode); err == nil {
			details = d
		}
	}()
	wg.Wait()

	if infoErr != nil {
		return domain.Quote{}, infoErr
	}
	if priceErr != nil {
		return domain.Quote{}, priceErr
	}

	quote := domain.Quote{
		ProductCode:    price.ProductCode,
		Symbol:         info.Symbol,
		Name:           info.Name,
		MarketCode:     info.Market.Code,
		Market:         info.Market.DisplayName,
		Currency:       firstNonEmpty(price.Currency, info.Currency),
		ReferencePrice: price.Base,
		Last:           price.Close,
		Change:         price.Close - price.Base,
		Volume:         price.Volume,
		Status:         info.Status,
		FetchedAt:      time.Now().UTC(),
	}

	if price.Base != 0 {
		quote.ChangeRate = quote.Change / price.Base
	}

	if detail != nil {
		quote.BadgeCount = len(detail.Badges)
		quote.NoticeCount = len(detail.Notices)
	}

	// Enrich with richer v3 details (non-fatal — base quote stands on its own).
	if d := details; d != nil {
		quote.Open = d.Open
		quote.High = d.High
		quote.Low = d.Low
		quote.High52w = d.High52w
		quote.Low52w = d.Low52w
		quote.MarketCap = d.MarketCap
		quote.TradingValue = d.Value
		quote.TradingStrength = d.TradingStrength
		quote.PrevVolume = d.PreDayVolume
		quote.UpperLimit = d.UpperLimit
		quote.LowerLimit = d.LowerLimit
	}

	return quote, nil
}

func (c *Client) resolveProductCode(ctx context.Context, symbol string) (string, error) {
	normalized := normalizeProductCode(symbol)
	if normalized == "" {
		return "", fmt.Errorf("symbol is required")
	}
	if looksLikeProductCode(normalized) {
		return normalized, nil
	}

	var envelope stockSearchEnvelope
	body := []byte(fmt.Sprintf(`{"query":%q}`, normalized))
	if err := c.postJSON(ctx, fmt.Sprintf("%s/api/v2/search/stocks", c.infoBaseURL), body, &envelope); err != nil {
		return "", err
	}
	if len(envelope.Result.Stocks) == 0 {
		return "", fmt.Errorf("no product code result returned for %s", normalized)
	}
	return envelope.Result.Stocks[0].StockCode, nil
}

func (c *Client) getStockInfo(ctx context.Context, productCode string) (stockInfoResult, error) {
	var envelope quoteEnvelope[stockInfoResult]
	if err := c.getJSON(ctx, fmt.Sprintf("%s/api/v2/stock-infos/%s", c.infoBaseURL, productCode), &envelope); err != nil {
		return stockInfoResult{}, err
	}
	return envelope.Result, nil
}

func (c *Client) getStockDetailCommon(ctx context.Context, productCode string) (*stockDetailCommonResult, error) {
	var envelope quoteEnvelope[stockDetailCommonResult]
	if err := c.getJSON(
		ctx,
		fmt.Sprintf("%s/api/v1/stock-detail/ui/%s/common", c.infoBaseURL, productCode),
		&envelope,
	); err != nil {
		return nil, err
	}
	return &envelope.Result, nil
}

func (c *Client) getStockPrice(ctx context.Context, productCode string) (stockPriceResult, error) {
	endpoint, err := url.Parse(fmt.Sprintf("%s/api/v1/product/stock-prices", c.infoBaseURL))
	if err != nil {
		return stockPriceResult{}, err
	}

	query := endpoint.Query()
	query.Set("meta", "true")
	query.Set("productCodes", productCode)
	endpoint.RawQuery = query.Encode()

	var envelope quoteEnvelope[[]stockPriceResult]
	if err := c.getJSON(ctx, endpoint.String(), &envelope); err != nil {
		return stockPriceResult{}, err
	}

	if len(envelope.Result) == 0 {
		return stockPriceResult{}, fmt.Errorf("no price result returned for %s", productCode)
	}

	return envelope.Result[0], nil
}

// getStockPriceDetails fetches the richer v3 details payload. Used only to
// enrich `quote get`; failure is non-fatal (caller falls back to base fields).
func (c *Client) getStockPriceDetails(ctx context.Context, productCode string) (*stockPriceDetailResult, error) {
	endpoint, err := url.Parse(fmt.Sprintf("%s/api/v3/stock-prices/details", c.infoBaseURL))
	if err != nil {
		return nil, err
	}
	query := endpoint.Query()
	query.Set("productCodes", productCode)
	endpoint.RawQuery = query.Encode()

	var envelope quoteEnvelope[[]stockPriceDetailResult]
	if err := c.getJSON(ctx, endpoint.String(), &envelope); err != nil {
		return nil, err
	}
	if len(envelope.Result) == 0 {
		return nil, fmt.Errorf("no detail result for %s", productCode)
	}
	return &envelope.Result[0], nil
}

func normalizeProductCode(symbol string) string {
	trimmed := strings.ToUpper(strings.TrimSpace(symbol))
	if trimmed == "" {
		return trimmed
	}

	if len(trimmed) == 6 && trimmed[0] >= '0' && trimmed[0] <= '9' {
		return "A" + trimmed
	}

	return trimmed
}

func looksLikeProductCode(value string) bool {
	// Option contract guids (OPT_AAPL260805C00230000_20260722) are product codes
	// too, but the shape check below rejects them: it allows only digits past
	// position 2, and these carry letters and underscores. Without this they get
	// sent to stock search, which has no entry for a contract — so every quote of
	// an option from `quote options` failed. The prefix is a safe discriminator:
	// the chain produces these, nobody types them as a ticker.
	if strings.HasPrefix(value, "OPT_") {
		return true
	}
	if len(value) == 7 && value[0] == 'A' {
		return true
	}
	if len(value) >= 8 && value[0] >= 'A' && value[0] <= 'Z' && value[1] >= 'A' && value[1] <= 'Z' {
		hasDigit := false
		for i := 2; i < len(value); i++ {
			if value[i] >= '0' && value[i] <= '9' {
				hasDigit = true
				continue
			}
			return false
		}
		return hasDigit
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
