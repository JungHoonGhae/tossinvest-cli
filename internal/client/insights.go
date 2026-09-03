package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

type relatedStockRaw struct {
	Code                string `json:"code"`
	StockCode           string `json:"stockCode"`
	Name                string `json:"name"`
	StockName           string `json:"stockName"`
	Symbol              string `json:"symbol"`
	Market              string `json:"market"`
	InvestmentType      string `json:"investmentType"`
	InvestmentTypeValue string `json:"investmentTypeValue"`
	CompanyCode         string `json:"companyCode"`
	CompanyName         string `json:"companyName"`
	LogoImageURL        string `json:"logoImageUrl"`
	Status              string `json:"status"`
	CommonShare         string `json:"commonShareYn"`
	Display             string `json:"displayYn"`
}

func mapRelatedStocks(values []relatedStockRaw) []domain.RelatedStock {
	result := make([]domain.RelatedStock, 0, len(values))
	for _, value := range values {
		productCode := value.StockCode
		if productCode == "" {
			productCode = value.Code
		}
		name := value.StockName
		if name == "" {
			name = value.Name
		}
		result = append(result, domain.RelatedStock{
			ProductCode:         productCode,
			Name:                name,
			Symbol:              value.Symbol,
			Market:              value.Market,
			InvestmentType:      value.InvestmentType,
			InvestmentTypeValue: value.InvestmentTypeValue,
			CompanyCode:         value.CompanyCode,
			CompanyName:         value.CompanyName,
			LogoImageURL:        value.LogoImageURL,
			Status:              value.Status,
			CommonShare:         value.CommonShare,
			Display:             value.Display,
		})
	}
	return result
}

type stockReasoningRaw struct {
	SectionTitle   string            `json:"sectionTitle"`
	SectionSummary string            `json:"sectionSummary"`
	SignalID       string            `json:"signalId"`
	Direction      int               `json:"signalDirection"`
	Keyword        string            `json:"keyword"`
	CreatedAt      string            `json:"createdAt"`
	RelatedStocks  []relatedStockRaw `json:"relatedStocks"`
}

// GetStockReasoning returns Toss's AI explanation of why a stock moved.
// 공식 Open API 에 없는 web 전용 표면.
func (c *Client) GetStockReasoning(ctx context.Context, symbol string) (domain.StockReasoning, error) {
	if err := c.requireSession(); err != nil {
		return domain.StockReasoning{}, err
	}
	productCode, err := c.resolveProductCode(ctx, symbol)
	if err != nil {
		return domain.StockReasoning{}, err
	}

	var envelope quoteEnvelope[stockReasoningRaw]
	endpoint := fmt.Sprintf("%s/api/v2/reasoning/stocks/%s", c.certBaseURL, url.PathEscape(productCode))
	if err := c.getJSON(ctx, endpoint, &envelope); err != nil {
		return domain.StockReasoning{}, err
	}

	r := envelope.Result
	out := domain.StockReasoning{
		Symbol:      symbol,
		ProductCode: productCode,
		Title:       r.SectionTitle,
		Summary:     r.SectionSummary,
		Direction:   r.Direction,
		Keyword:     r.Keyword,
		SignalID:    r.SignalID,
		CreatedAt:   r.CreatedAt,
		FetchedAt:   time.Now().UTC(),
	}
	out.RelatedStock = mapRelatedStocks(r.RelatedStocks)
	return out, nil
}

type stockSignalRaw struct {
	Label    string `json:"signalLabel"`
	Info     string `json:"signalInfo"`
	SignalID int64  `json:"signalId"`
	DateTime string `json:"datetime"`
}

type stockSignalsRaw struct {
	StockCode string           `json:"stockCode"`
	Signals   []stockSignalRaw `json:"signals"`
}

// GetStockSignals returns the per-stock signal cards. Distinct from
// GetAISignals, which is the market-wide personalized feed.
// 공식 Open API 에 없는 web 전용 표면.
func (c *Client) GetStockSignals(ctx context.Context, symbol string) (domain.StockSignals, error) {
	if err := c.requireSession(); err != nil {
		return domain.StockSignals{}, err
	}
	productCode, err := c.resolveProductCode(ctx, symbol)
	if err != nil {
		return domain.StockSignals{}, err
	}

	endpoint, err := url.Parse(c.infoBaseURL + "/api/v1/dashboard/wts/overview/signals")
	if err != nil {
		return domain.StockSignals{}, err
	}
	query := endpoint.Query()
	query.Set("codes", productCode)
	endpoint.RawQuery = query.Encode()

	var envelope quoteEnvelope[stockSignalsRaw]
	if err := c.getJSON(ctx, endpoint.String(), &envelope); err != nil {
		return domain.StockSignals{}, err
	}

	out := domain.StockSignals{
		Symbol:      symbol,
		ProductCode: productCode,
		FetchedAt:   time.Now().UTC(),
	}
	for _, s := range envelope.Result.Signals {
		out.Signals = append(out.Signals, domain.StockSignal{
			Label:    s.Label,
			Info:     s.Info,
			SignalID: s.SignalID,
			DateTime: s.DateTime,
		})
	}
	return out, nil
}

type marginNoticeRaw struct {
	NoticeType         string  `json:"depositNoticeType"`
	ReceivableAmount   float64 `json:"receivableAmount"`
	DeadlineAt         *string `json:"deadlineAt"`
	ForcedLiquidatedAt *string `json:"forcedLiquidatedAt"`
	SuspensionStart    *string `json:"suspensionStartDate"`
	SuspensionEnd      *string `json:"suspensionEndDate"`
}

// GetMarginNotice returns the receivable (미수금) and forced-liquidation warning
// state for one currency. 공식 Open API 에 없는 web 전용 표면.
//
// The nullable timestamps stay pointers all the way through: a healthy account
// has none of them, and materialising a zero time would render as an epoch date
// — indistinguishable from a long-overdue account.
func (c *Client) GetMarginNotice(ctx context.Context, currency string) (domain.MarginNotice, error) {
	if err := c.requireSession(); err != nil {
		return domain.MarginNotice{}, err
	}
	cur := strings.ToUpper(strings.TrimSpace(currency))
	if cur == "" {
		cur = "KRW"
	}

	endpoint, err := url.Parse(c.certBaseURL + "/api/v1/margin/cert/notice/receivable")
	if err != nil {
		return domain.MarginNotice{}, err
	}
	query := endpoint.Query()
	query.Set("currency", cur)
	endpoint.RawQuery = query.Encode()

	var envelope quoteEnvelope[marginNoticeRaw]
	if err := c.getJSON(ctx, endpoint.String(), &envelope); err != nil {
		return domain.MarginNotice{}, err
	}
	r := envelope.Result
	return domain.MarginNotice{
		Currency:           cur,
		NoticeType:         r.NoticeType,
		ReceivableAmount:   r.ReceivableAmount,
		DeadlineAt:         r.DeadlineAt,
		ForcedLiquidatedAt: r.ForcedLiquidatedAt,
		SuspensionStart:    r.SuspensionStart,
		SuspensionEnd:      r.SuspensionEnd,
		FetchedAt:          time.Now().UTC(),
	}, nil
}

type searchHitRaw struct {
	Keyword     string `json:"keyword"`
	SubKeyword  string `json:"subKeyword"`
	StockCode   string `json:"stockCode"`
	Symbol      string `json:"symbol"`
	CompanyName string `json:"companyName"`
	Market      string `json:"market"`
}

// Search runs Toss's unified autocomplete. 공식 Open API 에 없는 web 전용 표면.
func (c *Client) Search(ctx context.Context, query string) (domain.SearchResults, error) {
	if err := c.requireSession(); err != nil {
		return domain.SearchResults{}, err
	}
	q := strings.TrimSpace(query)
	if q == "" {
		return domain.SearchResults{}, fmt.Errorf("query is required")
	}

	body, err := json.Marshal(map[string]string{"query": q})
	if err != nil {
		return domain.SearchResults{}, err
	}

	var envelope quoteEnvelope[[]searchHitRaw]
	endpoint := c.infoBaseURL + "/api/v1/search-all/wts-auto-complete"
	if err := c.postJSON(ctx, endpoint, body, &envelope); err != nil {
		return domain.SearchResults{}, err
	}

	out := domain.SearchResults{Query: q, FetchedAt: time.Now().UTC()}
	for _, h := range envelope.Result {
		out.Results = append(out.Results, domain.SearchHit{
			Keyword:     h.Keyword,
			SubKeyword:  h.SubKeyword,
			ProductCode: h.StockCode,
			Symbol:      h.Symbol,
			CompanyName: h.CompanyName,
			Market:      h.Market,
		})
	}
	return out, nil
}

// GetStockReasons returns Toss's one-line AI explanation for many stocks in a
// single request (the web sends up to 100 at a time).
//
// Endpoint: POST /api/v1/dashboard/wts/overview/ai-signals (info host) with
// {"productCodes": [...]}. Distinct from GetStockReasoning, which fetches the
// full card — summary, direction, related stocks — for one symbol. This is the
// portfolio-wide "why is everything moving today" view, at one request instead
// of N.
//
// The server omits codes it has no reasoning for, so the response is shorter
// than the request and NOT positionally aligned. Results are restored to
// request order and omissions are returned explicitly.
func (c *Client) GetStockReasons(ctx context.Context, symbols []string) (domain.StockReasons, error) {
	if err := c.requireSession(); err != nil {
		return domain.StockReasons{}, err
	}
	// 응답은 productCode 로만 오므로 원래 입력 표기를 ordered pair 로 함께 둔다.
	// map 역인덱스는 같은 종목의 티커·이름 별칭이 서로를 덮어써서 쓸 수 없다.
	requests, err := c.resolveProductCodes(ctx, symbols)
	if err != nil {
		return domain.StockReasons{}, err
	}
	codes := batchCodes(requests)

	var envelope quoteEnvelope[struct {
		Signals []struct {
			ProductCode string `json:"productCode"`
			Description string `json:"reasoningDescription"`
		} `json:"signals"`
	}]
	endpoint := c.infoBaseURL + "/api/v1/dashboard/wts/overview/ai-signals"
	body, err := json.Marshal(map[string]any{"productCodes": codes})
	if err != nil {
		return domain.StockReasons{}, err
	}
	if err := c.postJSON(ctx, endpoint, body, &envelope); err != nil {
		return domain.StockReasons{}, err
	}
	reasonsByCode := make(map[string]domain.StockReason, len(envelope.Result.Signals))
	for _, s := range envelope.Result.Signals {
		reasonsByCode[s.ProductCode] = domain.StockReason{
			ProductCode: s.ProductCode,
			Description: s.Description,
		}
	}
	reasons, missing, sequence := reconcileBatch(requests, reasonsByCode, func(reason domain.StockReason, symbol string) domain.StockReason {
		reason.Symbol = symbol
		return reason
	})
	out := domain.StockReasons{Reasons: reasons, Missing: missing, Sequence: sequence, FetchedAt: time.Now().UTC()}
	return out, nil
}
