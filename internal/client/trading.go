package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/orderintent"
	tradingflow "github.com/JungHoonGhae/tossinvest-cli/internal/trading"
)

type pendingOrderDetails struct {
	OrderID            string
	OrderNo            string
	OrderedDate        string
	StockCode          string
	TradeType          string
	OrderPrice         float64
	OrderUSDPrice      float64
	Quantity           float64
	PendingQuantity    float64
	OrderPriceTypeCode string
	IsFractionalOrder  bool
	IsAfterMarketOrder bool
}

type stockPriceMetadata struct {
	Close        float64
	CloseKRW     float64
	ExchangeRate float64
}

type mutationEnvelope[T any] struct {
	Result T `json:"result"`
}

type authRequiredResult struct {
	Required    bool `json:"required"`
	SimpleTrade bool `json:"simpleTrade"`
	Verifier    any  `json:"verifier"`
}

type placePreparedOrderInfo struct {
	NeedExchange float64 `json:"needExchange"`
}

type placePrepareResult struct {
	OrderKey          string                 `json:"orderKey"`
	AuthRequired      authRequiredResult     `json:"authRequired"`
	PreparedOrderInfo placePreparedOrderInfo `json:"preparedOrderInfo"`
}

type cancelPrepareResult struct {
	DelayCancelExchange bool               `json:"delayCancelExchange"`
	OrderKey            string             `json:"orderKey"`
	AuthRequired        authRequiredResult `json:"authRequired"`
}

type cancelResult struct {
	Message   string `json:"message"`
	OrderDate string `json:"orderDate"`
	OrderNo   any    `json:"orderNo"`
	OrderID   string `json:"orderId"`
	UUID      string `json:"uuid"`
	Modulus   string `json:"modulus"`
	Exponent  string `json:"exponent"`
	Keyboard  string `json:"keyboard"`
}

type tradingSettingToggleEnvelope struct {
	Result struct {
		CategoryName string `json:"categoryName"`
		TurnedOn     bool   `json:"turnedOn"`
	} `json:"result"`
}

type exchangeQuoteForBuyEnvelope struct {
	Result struct {
		RateQuoteID      string  `json:"rateQuoteId"`
		BuyCurrency      string  `json:"buyCurrency"`
		SellCurrency     string  `json:"sellCurrency"`
		ValidFrom        string  `json:"validFrom"`
		ValidTill        string  `json:"validTill"`
		USDRate          float64 `json:"usdRate"`
		DisplayUSDRate   float64 `json:"displayUsdRate"`
		FavorablePercent float64 `json:"favorablePercent"`
	} `json:"result"`
}

var (
	mutationReconcileAttempts = 8
	mutationReconcileInterval = 250 * time.Millisecond
	mutationCompletedLookback = 2 * time.Minute
)

func (c *Client) PlacePendingOrder(ctx context.Context, intent orderintent.PlaceIntent) (tradingflow.MutationResult, error) {
	startedAt := time.Now()
	if err := c.ensureTradingMetadata(ctx); err != nil {
		return tradingflow.MutationResult{}, err
	}
	productCode, err := c.resolveProductCode(ctx, intent.Symbol)
	if err != nil {
		return tradingflow.MutationResult{}, err
	}

	info, err := c.getStockInfo(ctx, productCode)
	if err != nil {
		return tradingflow.MutationResult{}, err
	}
	price, err := c.getStockPrice(ctx, productCode)
	if err != nil {
		return tradingflow.MutationResult{}, err
	}

	var meta stockPriceMetadata
	if intent.Market == "kr" {
		meta = stockPriceMetadata{
			Close:        price.Close,
			CloseKRW:     price.Close, // KR stocks are already in KRW
			ExchangeRate: 1,           // no conversion needed
		}
	} else {
		usdRate, err := c.getUSDBaseExchangeRate(ctx)
		if err != nil {
			return tradingflow.MutationResult{}, err
		}
		meta = stockPriceMetadata{
			Close:        price.Close,
			CloseKRW:     math.Round(price.Close * usdRate),
			ExchangeRate: usdRate,
		}
	}
	bodyPrepare, err := buildPlaceBody(productCode, info.Market.Code, intent, meta, true)
	if err != nil {
		return tradingflow.MutationResult{}, err
	}
	bodyCreate, err := buildPlaceBody(productCode, info.Market.Code, intent, meta, false)
	if err != nil {
		return tradingflow.MutationResult{}, err
	}
	autoAcceptFXConsent := false

	var prepare mutationEnvelope[placePrepareResult]
	if err := c.postTradingJSON(ctx, fmt.Sprintf("%s/api/v2/wts/trading/order/prepare", c.certBaseURL), bodyPrepare, &prepare); err != nil {
		return tradingflow.MutationResult{}, classifyPlacePrepareFailure(err)
	}
	if prepare.Result.AuthRequired.Required {
		return tradingflow.MutationResult{}, tradingflow.ErrInteractiveAuthRequired
	}
	if strings.TrimSpace(prepare.Result.OrderKey) == "" {
		return tradingflow.MutationResult{}, fmt.Errorf("place prepare response did not include order key")
	}
	if intent.Market != "kr" && prepare.Result.PreparedOrderInfo.NeedExchange > 0 {
		if !c.tradingPolicy.DangerousAutomation.AcceptFXConsent {
			return tradingflow.MutationResult{}, c.buildPostPrepareFXBranch(ctx, prepare.Result.PreparedOrderInfo.NeedExchange)
		}
		c.prefetchPostPrepareFXContext(ctx)
		autoAcceptFXConsent = true
	}

	var create mutationEnvelope[cancelResult]
	if err := c.postTradingJSONWithHeaders(ctx, fmt.Sprintf("%s/api/v2/wts/trading/order/create", c.certBaseURL), bodyCreate, map[string]string{
		"X-Order-Key": prepare.Result.OrderKey,
	}, &create); err != nil {
		return tradingflow.MutationResult{}, classifyPlaceCreateFailure(err)
	}
	if autoAcceptFXConsent {
		c.acceptPostPrepareFXConsent(ctx)
	}
	if strings.TrimSpace(create.Result.UUID) != "" &&
		strings.TrimSpace(create.Result.Modulus) != "" &&
		strings.TrimSpace(create.Result.Exponent) != "" {
		return tradingflow.MutationResult{}, tradingflow.ErrInteractiveAuthRequired
	}

	return c.reconcilePlacedOrder(ctx, productCode, info.Symbol, intent.Market, intent.Price, intent.Quantity, startedAt)
}

func (c *Client) GetOrderAvailableActions(ctx context.Context, orderID string) (map[string]any, error) {
	if err := c.ensureTradingMetadata(ctx); err != nil {
		return nil, err
	}
	if err := c.requireSession(); err != nil {
		return nil, err
	}

	order, err := c.lookupPendingOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}

	query := url.Values{}
	query.Set("stockCode", order.StockCode)
	query.Set("tradeType", order.TradeType)
	query.Set("orderPriceType", order.OrderPriceTypeCode)
	query.Set("fractional", strconv.FormatBool(order.IsFractionalOrder))
	query.Set("isReservationOrder", strconv.FormatBool(order.IsAfterMarketOrder))

	brokerOrderID := strings.TrimSpace(order.OrderID)
	if brokerOrderID == "" {
		brokerOrderID = orderID
	}

	endpoint := fmt.Sprintf("%s/api/v3/trading/order/%s/available-actions?%s", c.certBaseURL, url.PathEscape(brokerOrderID), query.Encode())

	result := map[string]any{}
	if err := c.getJSON(ctx, endpoint, &result); err != nil {
		var statusErr *StatusError
		if errors.As(err, &statusErr) && (statusErr.StatusCode == 400 || statusErr.StatusCode == 404) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	return result, nil
}

func classifyPlacePrepareFailure(err error) error {
	var statusErr *StatusError
	if !errors.As(err, &statusErr) {
		return err
	}

	message := extractBrokerMessage(statusErr.Body)
	switch classifyPrepareBranch(statusErr.Body, message) {
	case tradingflow.BranchFundingRequired:
		return &tradingflow.BranchRequiredError{
			Branch:        tradingflow.BranchFundingRequired,
			Source:        tradingflow.BranchSourcePrepareRejection,
			StatusCode:    statusErr.StatusCode,
			BrokerMessage: message,
		}
	case tradingflow.BranchFXConsentRequired:
		return &tradingflow.BranchRequiredError{
			Branch:        tradingflow.BranchFXConsentRequired,
			Source:        tradingflow.BranchSourcePrepareRejection,
			StatusCode:    statusErr.StatusCode,
			BrokerMessage: message,
		}
	default:
		return &tradingflow.PrepareRejectedError{
			StatusCode:    statusErr.StatusCode,
			BrokerMessage: message,
		}
	}
}

// classifyPlaceCreateFailure surfaces the broker's rejection reason from an
// order/create error. The prepare step classifies funding/FX branches; a
// create-stage rejection is terminal, so we just want the user to SEE why
// (market closed, price band, etc.) instead of a bare "unexpected status 400".
func classifyPlaceCreateFailure(err error) error {
	var statusErr *StatusError
	if !errors.As(err, &statusErr) {
		return err
	}
	if message := extractBrokerMessage(statusErr.Body); message != "" {
		return fmt.Errorf("주문 생성이 거부되었습니다 (HTTP %d): %s", statusErr.StatusCode, message)
	}
	return err
}

func classifyPrepareBranch(body string, message string) tradingflow.Branch {
	signals := strings.ToLower(strings.Join(append(extractJSONStrings(body), message), "\n"))

	if containsAny(signals,
		"계좌 잔액이 부족",
		"잔액이 부족",
		"주문가능금액",
		"출금가능금액이 부족",
		"원화 출금가능금액이 부족",
		"환전에 필요한 원화",
		"구매를 위해",
		"채울게요",
		"모바일에서 채우기",
		"insufficient balance",
		"insufficient buying power",
	) {
		return tradingflow.BranchFundingRequired
	}

	hasFXKeyword := containsAny(signals,
		"환전",
		"외화 사용",
		"fx",
		"foreign exchange",
		"conversion",
		"convert",
	)
	hasConsentKeyword := containsAny(signals,
		"동의",
		"승인",
		"확인",
		"consent",
		"approve",
		"agreement",
	)
	if hasFXKeyword && hasConsentKeyword {
		return tradingflow.BranchFXConsentRequired
	}

	return ""
}

func (c *Client) buildPostPrepareFXBranch(ctx context.Context, needExchange float64) error {
	branchErr := &tradingflow.BranchRequiredError{
		Branch:     tradingflow.BranchFXConsentRequired,
		Source:     tradingflow.BranchSourcePostPrepareConfirmation,
		StatusCode: 200,
		FX: &tradingflow.FXConfirmationContext{
			NeedExchangeUSD: needExchange,
		},
	}

	if turnedOn, err := c.getTradingSettingToggle(ctx, "GETTING_BACK_KRW"); err == nil {
		branchErr.FX.GettingBackKRWKnown = true
		branchErr.FX.GettingBackKRW = turnedOn
	}

	if quote, err := c.getExchangeQuoteForBuy(ctx); err == nil {
		branchErr.FX.RateQuoteID = strings.TrimSpace(quote.Result.RateQuoteID)
		branchErr.FX.ValidFrom = strings.TrimSpace(quote.Result.ValidFrom)
		branchErr.FX.ValidTill = strings.TrimSpace(quote.Result.ValidTill)

		rate := quote.Result.USDRate
		if rate <= 0 {
			rate = quote.Result.DisplayUSDRate
		}
		if rate > 0 {
			branchErr.FX.USDExchangeRate = rate
			branchErr.FX.EstimatedExchangeKRW = math.Round(needExchange * rate)
		}
	}

	return branchErr
}

func (c *Client) acceptPostPrepareFXConsent(ctx context.Context) {
	_ = c.setTradingSettingToggle(ctx, "EXCHANGE_INFO_CHECK", true)
}

func (c *Client) prefetchPostPrepareFXContext(ctx context.Context) {
	_, _ = c.getTradingSettingToggle(ctx, "GETTING_BACK_KRW")
	_, _ = c.getExchangeQuoteForBuy(ctx)
}

func extractBrokerMessage(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}

	preferred := collectJSONStringsForKeys(body, map[string]struct{}{
		"title":       {},
		"message":     {},
		"body":        {},
		"description": {},
		"error":       {},
		"detail":      {},
	})
	if len(preferred) > 0 {
		return strings.Join(uniqueNonEmptyStrings(preferred), " / ")
	}

	all := uniqueNonEmptyStrings(extractJSONStrings(body))
	if len(all) == 0 {
		return body
	}
	limit := len(all)
	if limit > 3 {
		limit = 3
	}
	return strings.Join(all[:limit], " / ")
}

func collectJSONStringsForKeys(body string, keys map[string]struct{}) []string {
	var payload any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return nil
	}
	values := []string{}
	var visit func(any)
	visit = func(node any) {
		switch typed := node.(type) {
		case map[string]any:
			for key, value := range typed {
				if _, ok := keys[strings.ToLower(strings.TrimSpace(key))]; ok {
					if text, ok := value.(string); ok {
						values = append(values, text)
					}
				}
				visit(value)
			}
		case []any:
			for _, value := range typed {
				visit(value)
			}
		}
	}
	visit(payload)
	return values
}

func extractJSONStrings(body string) []string {
	var payload any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return nil
	}
	values := []string{}
	var visit func(any)
	visit = func(node any) {
		switch typed := node.(type) {
		case map[string]any:
			for _, value := range typed {
				visit(value)
			}
		case []any:
			for _, value := range typed {
				visit(value)
			}
		case string:
			values = append(values, typed)
		}
	}
	visit(payload)
	return values
}

func uniqueNonEmptyStrings(values []string) []string {
	seen := map[string]struct{}{}
	unique := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	return unique
}

func containsAny(haystack string, needles ...string) bool {
	for _, needle := range needles {
		if needle == "" {
			continue
		}
		if strings.Contains(haystack, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func (c *Client) CancelPendingOrder(ctx context.Context, intent orderintent.CancelIntent) (tradingflow.MutationResult, error) {
	startedAt := time.Now()
	if err := c.ensureTradingMetadata(ctx); err != nil {
		return tradingflow.MutationResult{}, err
	}
	order, err := c.lookupPendingOrder(ctx, intent.OrderID)
	if err != nil {
		return tradingflow.MutationResult{}, err
	}

	bodyPrepare, bodyCancel, err := buildCancelBodies(order)
	if err != nil {
		return tradingflow.MutationResult{}, err
	}

	var prepare mutationEnvelope[cancelPrepareResult]
	if err := c.postTradingJSON(ctx, fmt.Sprintf("%s/api/v2/wts/trading/order/cancel/prepare/%s/%s", c.certBaseURL, order.OrderedDate, order.OrderNo), bodyPrepare, &prepare); err != nil {
		return tradingflow.MutationResult{}, err
	}
	if prepare.Result.AuthRequired.Required {
		return tradingflow.MutationResult{}, tradingflow.ErrInteractiveAuthRequired
	}
	if strings.TrimSpace(prepare.Result.OrderKey) == "" {
		return tradingflow.MutationResult{}, fmt.Errorf("cancel prepare response did not include order key")
	}

	var cancel mutationEnvelope[cancelResult]
	if err := c.postTradingJSONWithHeaders(ctx, fmt.Sprintf("%s/api/v3/wts/trading/order/cancel/%s/%s", c.certBaseURL, order.OrderedDate, order.OrderNo), bodyCancel, map[string]string{
		"X-Order-Key": prepare.Result.OrderKey,
	}, &cancel); err != nil {
		return tradingflow.MutationResult{}, err
	}
	if strings.TrimSpace(cancel.Result.UUID) != "" &&
		strings.TrimSpace(cancel.Result.Modulus) != "" &&
		strings.TrimSpace(cancel.Result.Exponent) != "" {
		return tradingflow.MutationResult{}, tradingflow.ErrInteractiveAuthRequired
	}
	if strings.TrimSpace(cancel.Result.Message) == "" {
		return tradingflow.MutationResult{}, fmt.Errorf("cancel response did not include confirmation message")
	}

	expectedQty := order.PendingQuantity
	if expectedQty == 0 {
		expectedQty = order.Quantity
	}

	return c.reconcileCanceledOrder(ctx, intent.OrderID, order.StockCode, intent.Symbol, orderintent.InferMarketFromStockCode(order.StockCode), order.OrderPrice, expectedQty, startedAt)
}

func (c *Client) AmendPendingOrder(ctx context.Context, intent orderintent.AmendIntent) (tradingflow.MutationResult, error) {
	startedAt := time.Now()
	if err := c.ensureTradingMetadata(ctx); err != nil {
		return tradingflow.MutationResult{}, err
	}
	order, err := c.lookupPendingOrder(ctx, intent.OrderID)
	if err != nil {
		return tradingflow.MutationResult{}, err
	}

	info, err := c.getStockInfo(ctx, order.StockCode)
	if err != nil {
		return tradingflow.MutationResult{}, err
	}

	rate, err := c.getUSDBaseExchangeRate(ctx)
	if err != nil {
		return tradingflow.MutationResult{}, err
	}

	bodyPrepare, expectedPriceKRW, expectedQty, err := buildAmendBody(order, info.Market.Code, rate, intent.Quantity, intent.Price, true)
	if err != nil {
		return tradingflow.MutationResult{}, err
	}
	bodyCorrect, _, _, err := buildAmendBody(order, info.Market.Code, rate, intent.Quantity, intent.Price, false)
	if err != nil {
		return tradingflow.MutationResult{}, err
	}

	var prepare mutationEnvelope[cancelPrepareResult]
	if err := c.postTradingJSON(ctx, fmt.Sprintf("%s/api/v2/wts/trading/order/correct/prepare/%s/%s", c.certBaseURL, order.OrderedDate, order.OrderNo), bodyPrepare, &prepare); err != nil {
		return tradingflow.MutationResult{}, err
	}
	if prepare.Result.AuthRequired.Required {
		return tradingflow.MutationResult{}, tradingflow.ErrInteractiveAuthRequired
	}

	var correct mutationEnvelope[cancelResult]
	if err := c.postTradingJSON(ctx, fmt.Sprintf("%s/api/v2/wts/trading/order/correct/%s/%s", c.certBaseURL, order.OrderedDate, order.OrderNo), bodyCorrect, &correct); err != nil {
		return tradingflow.MutationResult{}, err
	}
	if strings.TrimSpace(correct.Result.UUID) != "" &&
		strings.TrimSpace(correct.Result.Modulus) != "" &&
		strings.TrimSpace(correct.Result.Exponent) != "" {
		return tradingflow.MutationResult{}, tradingflow.ErrInteractiveAuthRequired
	}

	return c.reconcileAmendedOrder(ctx, intent.OrderID, order.StockCode, info.Symbol, orderintent.InferMarketFromStockCode(order.StockCode), expectedPriceKRW, expectedQty, startedAt)
}

func (c *Client) HasPendingOrder(ctx context.Context, orderID string) (bool, error) {
	orders, err := c.ListPendingOrders(ctx)
	if err != nil {
		return false, err
	}

	for _, order := range orders {
		if orderMatchesID(order.Raw, orderID) {
			return true, nil
		}
	}

	return false, nil
}

func (c *Client) getUSDBaseExchangeRate(ctx context.Context) (float64, error) {
	var envelope struct {
		Result struct {
			Rate float64 `json:"rate"`
		} `json:"result"`
	}
	if err := c.getJSON(ctx, c.apiBaseURL+"/api/v1/exchange/usd/base-exchange-rate", &envelope); err != nil {
		return 0, err
	}
	if envelope.Result.Rate <= 0 {
		return 0, fmt.Errorf("usd base exchange rate was empty")
	}
	return envelope.Result.Rate, nil
}

func (c *Client) getTradingSettingToggle(ctx context.Context, category string) (bool, error) {
	var envelope tradingSettingToggleEnvelope
	endpoint := fmt.Sprintf("%s/api/v1/trading/settings/toggle/find?categoryName=%s", c.apiBaseURL, url.QueryEscape(category))
	if err := c.getJSON(ctx, endpoint, &envelope); err != nil {
		return false, err
	}
	return envelope.Result.TurnedOn, nil
}

func (c *Client) setTradingSettingToggle(ctx context.Context, category string, turnedOn bool) error {
	body, err := json.Marshal(map[string]any{
		"categoryName": category,
		"turnedOn":     turnedOn,
	})
	if err != nil {
		return err
	}
	return c.postEmptyJSONWithBody(ctx, c.apiBaseURL+"/api/v1/trading/settings/toggle", body)
}

func (c *Client) getExchangeQuoteForBuy(ctx context.Context) (exchangeQuoteForBuyEnvelope, error) {
	var envelope exchangeQuoteForBuyEnvelope
	if err := c.getJSON(ctx, c.apiBaseURL+"/api/v1/exchange/current-quote/for-buy", &envelope); err != nil {
		return exchangeQuoteForBuyEnvelope{}, err
	}
	return envelope, nil
}

func (c *Client) postEmptyJSON(ctx context.Context, endpoint string) error {
	return c.postRawJSON(ctx, endpoint, []byte("{}"))
}

func (c *Client) postEmptyJSONWithBody(ctx context.Context, endpoint string, body []byte) error {
	return c.postRawJSON(ctx, endpoint, body)
}

func (c *Client) postRawJSON(ctx context.Context, endpoint string, body []byte) error {
	_, err := c.postJSONBytes(ctx, endpoint, body, nil)
	return err
}

func (c *Client) postTradingJSON(ctx context.Context, endpoint string, body []byte, target any) error {
	return c.postTradingJSONWithHeaders(ctx, endpoint, body, nil, target)
}

func (c *Client) postTradingJSONWithHeaders(ctx context.Context, endpoint string, body []byte, extraHeaders map[string]string, target any) error {
	data, err := c.postJSONBytes(ctx, endpoint, body, extraHeaders)
	if err != nil {
		return err
	}
	if target == nil || len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, target)
}

func (c *Client) postJSONBytes(ctx context.Context, endpoint string, body []byte, extraHeaders map[string]string) ([]byte, error) {
	if len(body) == 0 {
		body = []byte("{}")
	}
	req, err := httpNewRequestWithBody(ctx, endpoint, body)
	if err != nil {
		return nil, err
	}
	c.applySession(req)
	c.applyTradingHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	for name, value := range extraHeaders {
		req.Header.Set(name, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, newStatusError(resp.StatusCode, endpoint, data)
	}
	return data, nil
}

func (c *Client) lookupPendingOrder(ctx context.Context, orderID string) (pendingOrderDetails, error) {
	orders, err := c.ListPendingOrders(ctx)
	if err != nil {
		return pendingOrderDetails{}, err
	}

	for _, order := range orders {
		if order.ID != orderID && !orderMatchesID(order.Raw, orderID) {
			continue
		}
		return decodePendingOrderDetails(order.Raw)
	}

	return pendingOrderDetails{}, fmt.Errorf("pending order %s was not found", orderID)
}

func orderMatchesID(raw json.RawMessage, orderID string) bool {
	var envelope struct {
		OrderNo       any    `json:"orderNo"`
		OrderID       string `json:"orderId"`
		ID            string `json:"id"`
		OrderedDate   string `json:"orderedDate"`
		UserOrderDate string `json:"userOrderDate"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return false
	}

	if envelope.OrderID == orderID || envelope.ID == orderID {
		return true
	}
	if referenceOrderIdentifier(envelope.OrderedDate, envelope.OrderNo, envelope.OrderID) == orderID {
		return true
	}
	if referenceOrderIdentifier(envelope.UserOrderDate, envelope.OrderNo, envelope.OrderID) == orderID {
		return true
	}

	switch value := envelope.OrderNo.(type) {
	case string:
		return strings.TrimSpace(value) == orderID
	case float64:
		return strconv.FormatInt(int64(value), 10) == orderID
	}

	return false
}

func decodePendingOrderDetails(raw json.RawMessage) (pendingOrderDetails, error) {
	var payload struct {
		OrderID            string  `json:"orderId"`
		OrderNo            any     `json:"orderNo"`
		OrderedDate        string  `json:"orderedDate"`
		StockCode          string  `json:"stockCode"`
		TradeType          string  `json:"tradeType"`
		OrderPrice         float64 `json:"orderPrice"`
		OrderUSDPrice      float64 `json:"orderUsdPrice"`
		Quantity           float64 `json:"quantity"`
		PendingQuantity    float64 `json:"pendingQuantity"`
		OrderPriceTypeCode string  `json:"orderPriceTypeCode"`
		IsFractionalOrder  bool    `json:"isFractionalOrder"`
		IsAfterMarketOrder bool    `json:"isAfterMarketOrder"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return pendingOrderDetails{}, err
	}

	return pendingOrderDetails{
		OrderID:            payload.OrderID,
		OrderNo:            normalizeOrderIdentifier(payload.OrderNo, ""),
		OrderedDate:        payload.OrderedDate,
		StockCode:          payload.StockCode,
		TradeType:          payload.TradeType,
		OrderPrice:         payload.OrderPrice,
		OrderUSDPrice:      payload.OrderUSDPrice,
		Quantity:           payload.Quantity,
		PendingQuantity:    payload.PendingQuantity,
		OrderPriceTypeCode: payload.OrderPriceTypeCode,
		IsFractionalOrder:  payload.IsFractionalOrder,
		IsAfterMarketOrder: payload.IsAfterMarketOrder,
	}, nil
}

func buildAmendBody(order pendingOrderDetails, marketCode string, usdRate float64, quantity *float64, priceKRW *float64, withOrderKey bool) ([]byte, float64, float64, error) {
	targetQty := order.Quantity
	if quantity != nil {
		targetQty = *quantity
	}

	targetPriceKRW := order.OrderPrice
	targetPriceUSD := order.OrderUSDPrice
	if priceKRW != nil {
		targetPriceKRW = *priceKRW
		targetPriceUSD = round2(targetPriceKRW / usdRate)
	}

	payload := map[string]any{
		"stockCode":              order.StockCode,
		"market":                 marketCode,
		"currencyMode":           "KRW",
		"tradeType":              order.TradeType,
		"price":                  targetPriceUSD,
		"quantity":               targetQty,
		"orderAmount":            0,
		"orderPriceType":         order.OrderPriceTypeCode,
		"agreedOver100Million":   false,
		"max":                    false,
		"isReservationOrder":     order.IsAfterMarketOrder,
		"openPriceSinglePriceYn": false,
	}
	if withOrderKey {
		payload["withOrderKey"] = true
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, 0, err
	}
	return body, targetPriceKRW, targetQty, nil
}

func buildCancelBodies(order pendingOrderDetails) ([]byte, []byte, error) {
	base := map[string]any{
		"isAfterMarketOrder": order.IsAfterMarketOrder,
		"quantity":           order.PendingQuantity,
		"stockCode":          order.StockCode,
		"tradeType":          order.TradeType,
		"isReservationOrder": order.IsAfterMarketOrder,
	}

	preparePayload := make(map[string]any, len(base)+1)
	for key, value := range base {
		preparePayload[key] = value
	}
	preparePayload["withOrderKey"] = true

	bodyPrepare, err := json.Marshal(preparePayload)
	if err != nil {
		return nil, nil, err
	}
	bodyCancel, err := json.Marshal(base)
	if err != nil {
		return nil, nil, err
	}
	return bodyPrepare, bodyCancel, nil
}

func buildPlaceBody(productCode, marketCode string, intent orderintent.PlaceIntent, meta stockPriceMetadata, withOrderKey bool) ([]byte, error) {
	var priceValue float64
	var quantityValue float64
	var orderAmount float64
	orderPriceType := "00" // limit

	switch {
	case intent.Fractional:
		// Fractional US market order. BUY is amount-based (`orderAmount`); SELL
		// is quantity-based (a decimal share count) — matching the official
		// Open API (≥1.1.5: decimal `quantity` for US MARKET SELL).
		priceValue = 0
		orderPriceType = "01" // market
		if intent.Side == "sell" {
			// Decimal share count goes through the `quantity` field as-is;
			// json.Marshal serializes e.g. 0.5 → 0.5.
			quantityValue = intent.Quantity
			orderAmount = 0
		} else {
			// Wire payload always carries currencyMode="KRW" (matches Toss web
			// spec and the rest of buildPlaceBody), so a USD amount must be
			// converted to KRW before it lands in the orderAmount field. Without
			// this, server reads `orderAmount: 100` as ₩100 and rejects with
			// "금액주문은 $1 또는 1,000원 이상" — see issue #28.
			quantityValue = 0
			if intent.CurrencyMode == "USD" {
				orderAmount = math.Round(intent.Amount * meta.ExchangeRate)
			} else {
				orderAmount = math.Round(intent.Amount)
			}
		}
	case intent.Market == "kr":
		priceValue = intent.Price
		quantityValue = intent.Quantity
	case intent.Market == "us" && intent.CurrencyMode == "USD":
		// US limit + USD input: price field carries USD as-is.
		priceValue = round2(intent.Price)
		quantityValue = intent.Quantity
	default:
		// US limit + KRW input: convert to USD using server-provided rate.
		priceValue = round2(intent.Price / meta.ExchangeRate)
		quantityValue = intent.Quantity
	}

	// Toss wire payload always uses currencyMode="KRW" even for US orders;
	// the price field carries USD in that case. Matches every captured
	// order/prepare body in docs/trading/rpc-catalog.md.
	payload := map[string]any{
		"stockCode":              productCode,
		"market":                 marketCode,
		"currencyMode":           "KRW",
		"tradeType":              intent.Side,
		"price":                  priceValue,
		"quantity":               quantityValue,
		"orderAmount":            orderAmount,
		"orderPriceType":         orderPriceType,
		"agreedOver100Million":   false,
		"marginTrading":          false,
		"max":                    false,
		"isReservationOrder":     false,
		"openPriceSinglePriceYn": false,
	}

	if intent.Fractional {
		payload["isFractionalOrder"] = true
	}

	// US stocks need allowAutoExchange for KRW→USD conversion
	if intent.Market != "kr" {
		payload["allowAutoExchange"] = true
	}

	if withOrderKey {
		payload["withOrderKey"] = true
	} else {
		extra := map[string]any{
			"close":       meta.Close,
			"orderMethod": "종목상세__주문하기",
		}
		if intent.Market != "kr" {
			extra["closeKrw"] = meta.CloseKRW
			extra["exchangeRate"] = meta.ExchangeRate
		}
		payload["extra"] = extra
	}

	return json.Marshal(payload)
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

func equalFloat(a, b float64) bool {
	return math.Abs(a-b) < 0.000001
}

func (c *Client) reconcilePlacedOrder(ctx context.Context, productCode, symbol, market string, expectedPriceKRW, expectedQty float64, startedAt time.Time) (tradingflow.MutationResult, error) {
	completedEarliest := startedAt.Add(-mutationCompletedLookback)
	for attempt := 0; attempt < mutationReconcileAttempts; attempt++ {
		if order, err := c.findMatchingPendingOrder(ctx, productCode, symbol, expectedPriceKRW, expectedQty, ""); err != nil {
			return tradingflow.MutationResult{}, err
		} else if order != nil {
			return tradingflow.MutationResult{
				Kind:      "place",
				Status:    "accepted_pending",
				OrderID:   order.ID,
				Symbol:    symbol,
				Market:    market,
				Quantity:  order.Quantity,
				Price:     order.Price,
				OrderDate: order.OrderDate,
			}, nil
		}

		if order, err := c.findMatchingCompletedOrder(ctx, market, productCode, symbol, expectedPriceKRW, expectedQty, completedEarliest, true, nil); err != nil {
			return tradingflow.MutationResult{}, err
		} else if order != nil {
			return tradingflow.MutationResult{
				Kind:                  "place",
				Status:                "filled_completed",
				OrderID:               order.ID,
				Symbol:                symbol,
				Market:                market,
				Quantity:              order.Quantity,
				FilledQuantity:        order.FilledQuantity,
				Price:                 order.Price,
				AverageExecutionPrice: order.AverageExecutionPrice,
				OrderDate:             order.OrderDate,
			}, nil
		}

		if attempt < mutationReconcileAttempts-1 {
			if err := waitForNextMutationCheck(ctx); err != nil {
				return tradingflow.MutationResult{}, err
			}
		}
	}

	return tradingflow.MutationResult{
		Kind:     "place",
		Status:   "unknown",
		Symbol:   symbol,
		Market:   market,
		Quantity: expectedQty,
		Price:    expectedPriceKRW,
		Warnings: []string{"Broker accepted the request but the final state was not visible in pending or completed history yet."},
	}, nil
}

func (c *Client) reconcileAmendedOrder(ctx context.Context, originalOrderID, productCode, symbol, market string, expectedPriceKRW, expectedQty float64, startedAt time.Time) (tradingflow.MutationResult, error) {
	completedEarliest := startedAt.Add(-mutationCompletedLookback)
	for attempt := 0; attempt < mutationReconcileAttempts; attempt++ {
		if order, err := c.findMatchingPendingOrder(ctx, productCode, symbol, expectedPriceKRW, expectedQty, originalOrderID); err != nil {
			return tradingflow.MutationResult{}, err
		} else if order != nil {
			return tradingflow.MutationResult{
				Kind:            "amend",
				Status:          "amended_pending",
				OrderID:         order.ID,
				OriginalOrderID: originalOrderID,
				CurrentOrderID:  order.ID,
				Symbol:          symbol,
				Market:          market,
				Quantity:        order.Quantity,
				Price:           order.Price,
				OrderDate:       order.OrderDate,
			}, nil
		}

		if order, err := c.findMatchingCompletedOrder(ctx, market, productCode, symbol, expectedPriceKRW, expectedQty, completedEarliest, false, nil); err != nil {
			return tradingflow.MutationResult{}, err
		} else if order != nil {
			return tradingflow.MutationResult{
				Kind:                  "amend",
				Status:                "amended_completed",
				OrderID:               order.ID,
				OriginalOrderID:       originalOrderID,
				CurrentOrderID:        order.ID,
				Symbol:                symbol,
				Market:                market,
				Quantity:              order.Quantity,
				FilledQuantity:        order.FilledQuantity,
				Price:                 order.Price,
				AverageExecutionPrice: order.AverageExecutionPrice,
				OrderDate:             order.OrderDate,
			}, nil
		}

		if attempt < mutationReconcileAttempts-1 {
			if err := waitForNextMutationCheck(ctx); err != nil {
				return tradingflow.MutationResult{}, err
			}
		}
	}

	return tradingflow.MutationResult{
		Kind:            "amend",
		Status:          "unknown",
		OriginalOrderID: originalOrderID,
		Symbol:          symbol,
		Market:          "us",
		Quantity:        expectedQty,
		Price:           expectedPriceKRW,
		Warnings:        []string{"Broker accepted the amend request but the surviving order state is not yet visible."},
	}, nil
}

func (c *Client) reconcileCanceledOrder(ctx context.Context, originalOrderID, productCode, symbol, market string, expectedPriceKRW, expectedQty float64, startedAt time.Time) (tradingflow.MutationResult, error) {
	completedEarliest := startedAt.Add(-mutationCompletedLookback)
	for attempt := 0; attempt < mutationReconcileAttempts; attempt++ {
		stillPending, err := c.HasPendingOrder(ctx, originalOrderID)
		if err != nil {
			return tradingflow.MutationResult{}, err
		}
		if !stillPending {
			if order, err := c.findMatchingCompletedOrder(ctx, market, productCode, symbol, expectedPriceKRW, expectedQty, completedEarliest, false, func(order domain.Order) bool {
				return orderStatusLooksCanceled(order.Status)
			}); err != nil {
				return tradingflow.MutationResult{}, err
			} else if order != nil {
				result := tradingflow.MutationResult{
					Kind:      "cancel",
					Status:    "canceled",
					OrderID:   order.ID,
					Symbol:    symbol,
					Market:    market,
					Quantity:  order.Quantity,
					Price:     order.Price,
					OrderDate: order.OrderDate,
				}
				if order.ID != originalOrderID {
					result.OriginalOrderID = originalOrderID
					result.CurrentOrderID = order.ID
				}
				return result, nil
			}

			if attempt == mutationReconcileAttempts-1 {
				return tradingflow.MutationResult{
					Kind:            "cancel",
					Status:          "canceled",
					OrderID:         originalOrderID,
					OriginalOrderID: originalOrderID,
					Symbol:          symbol,
					Market:          market,
					Quantity:        expectedQty,
					Price:           expectedPriceKRW,
					Warnings:        []string{"Pending order disappeared, but the canceled completed-history row is not visible yet."},
				}, nil
			}
		}

		if attempt < mutationReconcileAttempts-1 {
			if err := waitForNextMutationCheck(ctx); err != nil {
				return tradingflow.MutationResult{}, err
			}
		}
	}

	return tradingflow.MutationResult{}, tradingflow.ErrCancelStillPending
}

func (c *Client) findMatchingPendingOrder(ctx context.Context, productCode, symbol string, expectedPriceKRW, expectedQty float64, excludeID string) (*domain.Order, error) {
	orders, err := c.ListPendingOrders(ctx)
	if err != nil {
		return nil, err
	}
	for _, order := range orders {
		if excludeID != "" && (order.ID == excludeID || orderMatchesID(order.Raw, excludeID)) {
			continue
		}
		if !matchesOrderSymbol(order, productCode, symbol) {
			continue
		}
		if equalFloat(order.Price, expectedPriceKRW) && equalFloat(order.Quantity, expectedQty) {
			orderCopy := order
			return &orderCopy, nil
		}
	}
	return nil, nil
}

func (c *Client) findMatchingCompletedOrder(ctx context.Context, market, productCode, symbol string, expectedPriceKRW, expectedQty float64, earliestSubmittedAt time.Time, requireFilled bool, predicate func(domain.Order) bool) (*domain.Order, error) {
	orders, err := c.ListCompletedOrders(ctx, market)
	if err != nil {
		return nil, err
	}
	for _, order := range orders {
		if !earliestSubmittedAt.IsZero() {
			if order.SubmittedAt != nil {
				if order.SubmittedAt.Before(earliestSubmittedAt) {
					continue
				}
			} else if order.OrderDate != earliestSubmittedAt.Format("2006-01-02") {
				continue
			}
		}
		if !matchesOrderSymbol(order, productCode, symbol) {
			continue
		}
		if requireFilled && order.Status != "체결완료" {
			continue
		}
		if predicate != nil && !predicate(order) {
			continue
		}
		if equalFloat(order.Price, expectedPriceKRW) && (equalFloat(order.Quantity, expectedQty) || equalFloat(order.FilledQuantity, expectedQty)) {
			orderCopy := order
			return &orderCopy, nil
		}
	}
	return nil, nil
}

func matchesOrderSymbol(order domain.Order, productCode, symbol string) bool {
	return strings.EqualFold(order.Symbol, productCode) || strings.EqualFold(order.Symbol, symbol)
}

func orderStatusLooksCanceled(status string) bool {
	return strings.Contains(strings.TrimSpace(status), "취소")
}

func waitForNextMutationCheck(ctx context.Context) error {
	timer := time.NewTimer(mutationReconcileInterval)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
