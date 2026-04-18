package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/junghoonkye/tossinvest-cli/internal/domain"
)

type accountListEnvelope struct {
	Result struct {
		AccountList []struct {
			AccountNo   string   `json:"accountNo"`
			Key         string   `json:"key"`
			Name        string   `json:"name"`
			DisplayName string   `json:"displayName"`
			Type        string   `json:"type"`
			Markets     []string `json:"markets"`
			BuyMarkets  []string `json:"buyMarkets"`
			SellMarkets []string `json:"sellMarkets"`
		} `json:"accountList"`
		PrimaryKey string `json:"primaryKey"`
	} `json:"result"`
}

type accountSummaryEnvelope struct {
	Result struct {
		AccountNo             string                           `json:"accountNo"`
		TotalAssetAmount      float64                          `json:"totalAssetAmount"`
		EvaluatedProfitAmount float64                          `json:"evaluatedProfitAmount"`
		ProfitRate            float64                          `json:"profitRate"`
		OverviewByMarket      map[string]accountMarketOverview `json:"overviewByMarket"`
	} `json:"result"`
}

type accountMarketOverview struct {
	Market                string  `json:"market"`
	AccountNo             string  `json:"accountNo"`
	PendingBuyOrderAmount float64 `json:"pendingBuyOrderAmount"`
	EvaluatedAmount       float64 `json:"evaluatedAmount"`
	PrincipalAmount       float64 `json:"principalAmount"`
	EvaluatedProfitAmount float64 `json:"evaluatedProfitAmount"`
	ProfitRate            float64 `json:"profitRate"`
	TotalAssetAmount      float64 `json:"totalAssetAmount"`
	OrderableAmount       struct {
		KRW *float64 `json:"krw"`
		USD *float64 `json:"usd"`
	} `json:"orderableAmount"`
}

type orderableAmountEnvelope struct {
	Result struct {
		OrderableAmountKr struct {
			KRW *float64 `json:"krw"`
			USD *float64 `json:"usd"`
		} `json:"orderableAmountKr"`
		OrderableAmountUs struct {
			KRW *float64 `json:"krw"`
			USD *float64 `json:"usd"`
		} `json:"orderableAmountUs"`
	} `json:"result"`
}

type withdrawableAmountEnvelope struct {
	Result map[string]any `json:"result"`
}

type pendingOrdersEnvelope struct {
	Result []json.RawMessage `json:"result"`
}

func (c *Client) ListAccounts(ctx context.Context) ([]domain.Account, string, error) {
	if err := c.requireSession(); err != nil {
		return nil, "", err
	}

	var envelope accountListEnvelope
	if err := c.getJSON(ctx, c.apiBaseURL+"/api/v1/account/list", &envelope); err != nil {
		return nil, "", err
	}

	accounts := make([]domain.Account, 0, len(envelope.Result.AccountList))
	for _, item := range envelope.Result.AccountList {
		accounts = append(accounts, domain.Account{
			ID:          item.Key,
			DisplayName: item.DisplayName,
			Name:        item.Name,
			Type:        item.Type,
			Markets:     item.Markets,
			Primary:     item.Key == envelope.Result.PrimaryKey,
		})
	}

	return accounts, envelope.Result.PrimaryKey, nil
}

func (c *Client) GetAccountSummary(ctx context.Context) (domain.AccountSummary, error) {
	if err := c.requireSession(); err != nil {
		return domain.AccountSummary{}, err
	}

	var overview accountSummaryEnvelope
	if err := c.getJSON(ctx, c.certBaseURL+"/api/v3/my-assets/summaries/markets/all/overview", &overview); err != nil {
		return domain.AccountSummary{}, err
	}

	var orderable orderableAmountEnvelope
	if err := c.getJSON(ctx, c.certBaseURL+"/api/v1/dashboard/common/cached-orderable-amount", &orderable); err != nil {
		return domain.AccountSummary{}, err
	}

	krWithdraw, err := c.getWithdrawable(ctx, c.apiBaseURL+"/api/v1/my-assets/summaries/markets/kr/withdrawable-amount")
	if err != nil {
		return domain.AccountSummary{}, err
	}
	usWithdraw, err := c.getWithdrawable(ctx, c.apiBaseURL+"/api/v1/my-assets/summaries/markets/us/withdrawable-amount")
	if err != nil {
		return domain.AccountSummary{}, err
	}

	summary := domain.AccountSummary{
		TotalAssetAmount:      overview.Result.TotalAssetAmount,
		EvaluatedProfitAmount: overview.Result.EvaluatedProfitAmount,
		ProfitRate:            overview.Result.ProfitRate,
		OrderableAmountKRW:    pointerFloat(orderable.Result.OrderableAmountKr.KRW),
		OrderableAmountUSD:    pointerFloat(orderable.Result.OrderableAmountUs.USD),
		WithdrawableKR:        krWithdraw,
		WithdrawableUS:        usWithdraw,
		Markets:               map[string]domain.AccountMarketSummary{},
	}

	for key, item := range overview.Result.OverviewByMarket {
		summary.Markets[key] = domain.AccountMarketSummary{
			Market:                item.Market,
			PendingBuyOrderAmount: item.PendingBuyOrderAmount,
			EvaluatedAmount:       item.EvaluatedAmount,
			PrincipalAmount:       item.PrincipalAmount,
			EvaluatedProfitAmount: item.EvaluatedProfitAmount,
			ProfitRate:            item.ProfitRate,
			TotalAssetAmount:      item.TotalAssetAmount,
			OrderableAmountKRW:    pointerFloat(item.OrderableAmount.KRW),
			OrderableAmountUSD:    pointerFloat(item.OrderableAmount.USD),
		}
	}

	return summary, nil
}

func (c *Client) ListPendingOrders(ctx context.Context) ([]domain.Order, error) {
	if err := c.requireSession(); err != nil {
		return nil, err
	}

	var envelope pendingOrdersEnvelope
	if err := c.getJSON(ctx, c.certBaseURL+"/api/v1/trading/orders/histories/all/pending", &envelope); err != nil {
		return nil, err
	}

	orders := make([]domain.Order, 0, len(envelope.Result))
	for _, item := range envelope.Result {
		orders = append(orders, parsePendingOrder(item))
	}

	return orders, nil
}

func parsePendingOrder(raw json.RawMessage) domain.Order {
	order := domain.Order{Raw: raw}

	var payload struct {
		OrderNo         any     `json:"orderNo"`
		OrderID         string  `json:"orderId"`
		OrderedDate     string  `json:"orderedDate"`
		StockCode       string  `json:"stockCode"`
		StockName       string  `json:"stockName"`
		Symbol          string  `json:"symbol"`
		MarketDivision  string  `json:"marketDivision"`
		TradeType       string  `json:"tradeType"`
		Status          string  `json:"status"`
		Quantity        float64 `json:"quantity"`
		PendingQuantity float64 `json:"pendingQuantity"`
		OrderPrice      float64 `json:"orderPrice"`
		CreatedAt       string  `json:"createdAt"`
		OrderedAt       string  `json:"orderedAt"`
	}

	if err := json.Unmarshal(raw, &payload); err != nil {
		return order
	}

	order.ID = referenceOrderIdentifier(payload.OrderedDate, payload.OrderNo, payload.OrderID)
	order.Symbol = firstNonEmpty(payload.Symbol, payload.StockCode)
	order.Name = payload.StockName
	order.Market = strings.ToLower(payload.MarketDivision)
	order.Side = payload.TradeType
	order.Status = payload.Status
	order.Quantity = payload.PendingQuantity
	if order.Quantity == 0 {
		order.Quantity = payload.Quantity
	}
	order.Price = payload.OrderPrice
	order.SubmittedAt = parseOrderTime(payload.OrderedAt, payload.CreatedAt)
	return order
}

func normalizeOrderIdentifier(orderNo any, fallback string) string {
	switch value := orderNo.(type) {
	case string:
		if value != "" {
			return value
		}
	case float64:
		return strconv.FormatInt(int64(value), 10)
	}
	return fallback
}

func referenceOrderIdentifier(orderDate string, orderNo any, fallback string) string {
	normalized := normalizeOrderIdentifier(orderNo, fallback)
	if strings.TrimSpace(orderDate) == "" {
		return normalized
	}
	if normalized == "" {
		return strings.TrimSpace(orderDate)
	}
	return strings.TrimSpace(orderDate) + "/" + normalized
}

func parseOrderTime(values ...string) *time.Time {
	for _, value := range values {
		if value == "" {
			continue
		}
		if parsed, err := time.ParseInLocation("2006-01-02T15:04:05.999999999", value, time.Local); err == nil {
			return &parsed
		}
		if parsed, err := time.ParseInLocation("2006-01-02 15:04:05.000", value, time.Local); err == nil {
			return &parsed
		}
		if parsed, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.Local); err == nil {
			return &parsed
		}
		if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
			return &parsed
		}
		if parsed, err := time.Parse(time.RFC3339, value); err == nil {
			return &parsed
		}
	}
	return nil
}

func (c *Client) getWithdrawable(ctx context.Context, endpoint string) (map[string]any, error) {
	var envelope withdrawableAmountEnvelope
	if err := c.getJSON(ctx, endpoint, &envelope); err != nil {
		return nil, err
	}
	return envelope.Result, nil
}

func pointerFloat(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func (c *Client) getJSON(ctx context.Context, endpoint string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	c.applySession(req)
	if err := c.prepareCertRequest(ctx, endpoint, req); err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return newStatusError(resp.StatusCode, endpoint, data)
	}

	return json.Unmarshal(data, target)
}

func (c *Client) ValidateSession(ctx context.Context) error {
	if err := c.requireSession(); err != nil {
		return err
	}

	var envelope accountListEnvelope
	return c.getJSON(ctx, c.apiBaseURL+"/api/v1/account/list", &envelope)
}

func (c *Client) requireSession() error {
	if c.session == nil || len(c.session.Cookies) == 0 {
		return ErrNoSession
	}

	return nil
}

func (c *Client) applySession(req *http.Request) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Referer", "https://www.tossinvest.com/account")
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", defaultBrowserUserAgent)
	}

	if c.session == nil {
		return
	}

	for name, value := range c.session.Cookies {
		req.AddCookie(&http.Cookie{Name: name, Value: value})
	}

	for name, value := range c.session.Headers {
		req.Header.Set(name, value)
	}
}

func (c *Client) applyTradingHeaders(req *http.Request) {
	if strings.TrimSpace(c.browserTabID) != "" {
		req.Header.Set("Browser-Tab-Id", c.browserTabID)
	}
	if req.Header.Get("X-Tossinvest-Account") == "" {
		req.Header.Set("X-Tossinvest-Account", "1")
	}
	if strings.TrimSpace(c.appVersion) != "" {
		req.Header.Set("App-Version", c.appVersion)
	}
}

func (c *Client) prepareCertRequest(ctx context.Context, endpoint string, req *http.Request) error {
	if !strings.HasPrefix(endpoint, c.certBaseURL+"/") {
		return nil
	}
	if err := c.ensureTradingMetadata(ctx); err != nil {
		return err
	}
	c.applyTradingHeaders(req)
	return nil
}
