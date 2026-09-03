package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/orderintent"
)

const paperEnvironment = "paper"

type paperEnvelope[T any] struct {
	Result        T    `json:"-"`
	ResultPresent bool `json:"-"`
}

func (e *paperEnvelope[T]) UnmarshalJSON(data []byte) error {
	var zero T
	e.Result = zero
	e.ResultPresent = false
	var wire struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	if len(wire.Result) == 0 || bytes.Equal(bytes.TrimSpace(wire.Result), []byte("null")) {
		return nil
	}
	if err := json.Unmarshal(wire.Result, &e.Result); err != nil {
		return err
	}
	e.ResultPresent = true
	return nil
}

func (e paperEnvelope[T]) requireResult(operation string) error {
	if !e.ResultPresent {
		return fmt.Errorf("%s returned a success response without a result", operation)
	}
	return nil
}

type paperCashBalanceWire struct {
	ID                  int64    `json:"id"`
	Deposit             *float64 `json:"deposit"`
	OrderableAmount     *float64 `json:"orderableAmount"`
	WithdrawableAmount  *float64 `json:"withdrawableAmount"`
	MarginAmount        *float64 `json:"marginAmount"`
	UnsettledAmount     *float64 `json:"unsettledAmount"`
	BuyExecutionAmount  *float64 `json:"buyExecutionAmount"`
	SellExecutionAmount *float64 `json:"sellExecutionAmount"`
}

type paperEducationProgressWire struct {
	TotalSeconds     int  `json:"totalSeconds"`
	RequiredSeconds  int  `json:"requiredSeconds"`
	RemainingSeconds int  `json:"remainingSeconds"`
	Completed        bool `json:"completed"`
}

type paperEducationSummaryWire struct {
	Lecture                    *paperEducationProgressWire `json:"lecture"`
	PaperTrading               *paperEducationProgressWire `json:"paperTrading"`
	AllCompleted               *bool                       `json:"allCompleted"`
	OverseasDerivativeEligible *bool                       `json:"overseasDerivativeEligible"`
}

type paperReceiptWire struct {
	Message   string `json:"message"`
	OrderDate string `json:"orderDate"`
	OrderNo   any    `json:"orderNo"`
	OrderID   string `json:"orderId"`
}

type paperPrepareWire struct {
	OrderKey     string `json:"orderKey"`
	AuthRequired struct {
		Required bool `json:"required"`
	} `json:"authRequired"`
	PreparedOrderInfo struct {
		Quantity int `json:"quantity"`
	} `json:"preparedOrderInfo"`
}

// GetPaperStatus reads only the dedicated paper ledger and its server-side
// prerequisite status. A successful response is capability evidence even when
// the WTS UI launch flag is off.
func (c *Client) GetPaperStatus(ctx context.Context) (domain.PaperStatus, error) {
	if err := c.requireSession(); err != nil {
		return domain.PaperStatus{}, err
	}
	var balance paperEnvelope[paperCashBalanceWire]
	if err := c.getJSON(ctx, c.certBaseURL+"/api/v1/paper/cash-balance", &balance); err != nil {
		return domain.PaperStatus{}, err
	}
	if err := balance.requireResult("paper cash balance"); err != nil {
		return domain.PaperStatus{}, err
	}
	if balance.Result.Deposit == nil || balance.Result.OrderableAmount == nil || balance.Result.WithdrawableAmount == nil ||
		balance.Result.MarginAmount == nil || balance.Result.UnsettledAmount == nil || balance.Result.BuyExecutionAmount == nil ||
		balance.Result.SellExecutionAmount == nil {
		return domain.PaperStatus{}, fmt.Errorf("paper cash balance returned an incomplete result")
	}
	var education paperEnvelope[paperEducationSummaryWire]
	if err := c.getJSON(ctx, c.certBaseURL+"/api/v1/paper/education/summary", &education); err != nil {
		return domain.PaperStatus{}, err
	}
	if err := education.requireResult("paper education summary"); err != nil {
		return domain.PaperStatus{}, err
	}
	if education.Result.Lecture == nil || education.Result.PaperTrading == nil || education.Result.AllCompleted == nil || education.Result.OverseasDerivativeEligible == nil {
		return domain.PaperStatus{}, fmt.Errorf("paper education summary returned an incomplete result")
	}
	return domain.PaperStatus{
		Environment: paperEnvironment,
		Product:     "us-options",
		Balance: domain.PaperCashBalance{
			Deposit:         *balance.Result.Deposit,
			OrderableAmount: *balance.Result.OrderableAmount, WithdrawableAmount: *balance.Result.WithdrawableAmount,
			MarginAmount: *balance.Result.MarginAmount, UnsettledAmount: *balance.Result.UnsettledAmount,
			BuyExecutionAmount: *balance.Result.BuyExecutionAmount, SellExecutionAmount: *balance.Result.SellExecutionAmount,
		},
		Education: domain.PaperEducationSummary{
			Lecture:                    mapPaperProgress(*education.Result.Lecture),
			PaperTrading:               mapPaperProgress(*education.Result.PaperTrading),
			AllCompleted:               *education.Result.AllCompleted,
			OverseasDerivativeEligible: *education.Result.OverseasDerivativeEligible,
		},
	}, nil
}

func mapPaperProgress(w paperEducationProgressWire) domain.PaperEducationProgress {
	return domain.PaperEducationProgress{
		TotalSeconds: w.TotalSeconds, RequiredSeconds: w.RequiredSeconds,
		RemainingSeconds: w.RemainingSeconds, Completed: w.Completed,
	}
}

func (c *Client) InitPaperTrading(ctx context.Context) (domain.PaperMutationReceipt, error) {
	if err := c.requireSession(); err != nil {
		return domain.PaperMutationReceipt{}, err
	}
	if err := c.ensureTradingMetadata(ctx); err != nil {
		return domain.PaperMutationReceipt{}, err
	}
	var env paperEnvelope[json.RawMessage]
	if err := c.postPaperNoBody(ctx, c.certBaseURL+"/api/v1/paper/init", &env); err != nil {
		return domain.PaperMutationReceipt{}, err
	}
	if err := env.requireResult("paper initialization"); err != nil {
		return domain.PaperMutationReceipt{}, err
	}
	return decodePaperReceipt(env.Result), nil
}

func (c *Client) DepositPaperCash(ctx context.Context, amount int64) (domain.PaperMutationReceipt, error) {
	if amount <= 0 {
		return domain.PaperMutationReceipt{}, fmt.Errorf("paper deposit amount must be greater than zero")
	}
	return c.paperMutation(ctx, "/api/v1/paper/deposit", map[string]any{"amount": amount}, nil)
}

func (c *Client) paperMutation(ctx context.Context, path string, body map[string]any, headers map[string]string) (domain.PaperMutationReceipt, error) {
	if err := c.requireSession(); err != nil {
		return domain.PaperMutationReceipt{}, err
	}
	if err := c.ensureTradingMetadata(ctx); err != nil {
		return domain.PaperMutationReceipt{}, err
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return domain.PaperMutationReceipt{}, err
	}
	var env paperEnvelope[json.RawMessage]
	if err := c.postTradingJSONWithHeaders(ctx, c.certBaseURL+path, payload, headers, &env); err != nil {
		return domain.PaperMutationReceipt{}, err
	}
	if err := env.requireResult("paper mutation"); err != nil {
		return domain.PaperMutationReceipt{}, err
	}
	return decodePaperReceipt(env.Result), nil
}

func decodePaperReceipt(raw json.RawMessage) domain.PaperMutationReceipt {
	receipt := domain.PaperMutationReceipt{Environment: paperEnvironment}
	var wire paperReceiptWire
	if len(raw) > 0 && json.Unmarshal(raw, &wire) == nil {
		return mapPaperReceipt(wire)
	}
	return receipt
}

func (c *Client) postPaperNoBody(ctx context.Context, endpoint string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return err
	}
	c.applySession(req)
	c.applyTradingHeaders(req)
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
	if target != nil && len(data) > 0 {
		return json.Unmarshal(data, target)
	}
	return nil
}

func mapPaperReceipt(w paperReceiptWire) domain.PaperMutationReceipt {
	return domain.PaperMutationReceipt{
		Environment: paperEnvironment, Message: w.Message, OrderDate: w.OrderDate,
		OrderNo: normalizeOrderIdentifier(w.OrderNo, ""), OrderID: w.OrderID,
	}
}

func (c *Client) PlacePaperOrder(ctx context.Context, intent orderintent.OptionPlaceIntent) (domain.PaperMutationReceipt, error) {
	preview, body, prepared, err := c.preparePaperOrder(ctx, intent)
	if err != nil {
		return domain.PaperMutationReceipt{}, err
	}
	if prepared.Result.AuthRequired.Required {
		return domain.PaperMutationReceipt{}, fmt.Errorf("paper order requires interactive server authentication")
	}
	if prepared.Result.PreparedOrderInfo.Quantity > 0 {
		body["quantity"] = prepared.Result.PreparedOrderInfo.Quantity
	}
	// Keep the same server-resolved exchange and normalized values that were
	// validated by prepare. Re-resolving here could create a TOCTOU mismatch.
	body["market"] = preview.Intent.Exchange
	payload, _ := json.Marshal(body)
	var created paperEnvelope[paperReceiptWire]
	var headers map[string]string
	if orderKey := strings.TrimSpace(prepared.Result.OrderKey); orderKey != "" {
		headers = map[string]string{"X-Order-Key": orderKey}
	}
	if err := c.postTradingJSONWithHeaders(ctx, c.certBaseURL+"/api/v2/paper/trading/order/create", payload, headers, &created); err != nil {
		return domain.PaperMutationReceipt{}, err
	}
	if err := created.requireResult("paper order create"); err != nil {
		return domain.PaperMutationReceipt{}, fmt.Errorf("%w; inspect pending and completed paper orders before retrying", err)
	}
	receipt := mapPaperReceipt(created.Result)
	if strings.TrimSpace(receipt.OrderID) == "" && strings.TrimSpace(receipt.OrderNo) == "" {
		return domain.PaperMutationReceipt{}, fmt.Errorf("paper order create returned no order identifier; inspect pending and completed paper orders before retrying")
	}
	return receipt, nil
}

// PreviewPaperOrder asks Toss's dedicated paper prepare endpoint to validate an
// intent but never calls create. The order key is retained only in the private
// prepare result and cannot be serialized by callers.
func (c *Client) PreviewPaperOrder(ctx context.Context, intent orderintent.OptionPlaceIntent) (domain.PaperOrderPreview, error) {
	preview, _, _, err := c.preparePaperOrder(ctx, intent)
	return preview, err
}

func (c *Client) preparePaperOrder(ctx context.Context, intent orderintent.OptionPlaceIntent) (domain.PaperOrderPreview, map[string]any, paperEnvelope[paperPrepareWire], error) {
	if err := c.requireSession(); err != nil {
		return domain.PaperOrderPreview{}, nil, paperEnvelope[paperPrepareWire]{}, err
	}
	if err := c.ensureTradingMetadata(ctx); err != nil {
		return domain.PaperOrderPreview{}, nil, paperEnvelope[paperPrepareWire]{}, err
	}
	resolved, err := c.resolvePaperOrderIntent(ctx, intent)
	if err != nil {
		return domain.PaperOrderPreview{}, nil, paperEnvelope[paperPrepareWire]{}, err
	}
	body, err := paperOrderBody(resolved)
	if err != nil {
		return domain.PaperOrderPreview{}, nil, paperEnvelope[paperPrepareWire]{}, err
	}
	prepareBody := cloneAnyMap(body)
	prepareBody["withOrderKey"] = true
	payload, _ := json.Marshal(prepareBody)
	var prepared paperEnvelope[paperPrepareWire]
	if err := c.postTradingJSON(ctx, c.certBaseURL+"/api/v2/paper/trading/order/prepare", payload, &prepared); err != nil {
		return domain.PaperOrderPreview{}, nil, paperEnvelope[paperPrepareWire]{}, err
	}
	if err := prepared.requireResult("paper order prepare"); err != nil {
		return domain.PaperOrderPreview{}, nil, paperEnvelope[paperPrepareWire]{}, err
	}
	if prepared.Result.PreparedOrderInfo.Quantity <= 0 {
		return domain.PaperOrderPreview{}, nil, paperEnvelope[paperPrepareWire]{}, fmt.Errorf("paper order prepare returned no positive prepared quantity")
	}
	preview := domain.PaperOrderPreview{
		Environment: paperEnvironment, Product: "us-options", Intent: resolved,
		PreparedQuantity: prepared.Result.PreparedOrderInfo.Quantity,
		AuthRequired:     prepared.Result.AuthRequired.Required,
	}
	return preview, body, prepared, nil
}

func (c *Client) resolvePaperOrderIntent(ctx context.Context, intent orderintent.OptionPlaceIntent) (orderintent.OptionPlaceIntent, error) {
	normalized, err := orderintent.NormalizeOptionPlace(orderintent.OptionPlaceInput{
		Symbol: intent.Symbol, Exchange: intent.Exchange, CurrencyMode: intent.CurrencyMode,
		Side: intent.Side, OrderType: intent.OrderType, Price: intent.Price, Quantity: intent.Quantity,
	})
	if err != nil {
		return orderintent.OptionPlaceIntent{}, err
	}
	if normalized.Exchange == "" {
		info, err := c.getStockInfo(ctx, normalized.Symbol)
		if err != nil {
			return orderintent.OptionPlaceIntent{}, fmt.Errorf("resolve paper option exchange: %w", err)
		}
		normalized.Exchange = strings.ToUpper(strings.TrimSpace(info.Market.Code))
		if normalized.Exchange == "" {
			return orderintent.OptionPlaceIntent{}, fmt.Errorf("paper option exchange was not returned for %s", normalized.Symbol)
		}
	}
	return normalized, nil
}

func paperOrderBody(intent orderintent.OptionPlaceIntent) (map[string]any, error) {
	stockCode := strings.TrimSpace(intent.Symbol)
	if stockCode == "" {
		return nil, fmt.Errorf("paper option stock code must not be empty")
	}
	market := strings.ToUpper(strings.TrimSpace(intent.Exchange))
	if market == "" {
		return nil, fmt.Errorf("paper option exchange must not be empty")
	}
	currency := strings.ToUpper(strings.TrimSpace(intent.CurrencyMode))
	if currency == "" {
		currency = "USD"
	}
	if currency != "USD" && currency != "KRW" {
		return nil, fmt.Errorf("paper currency mode must be USD or KRW")
	}
	side := strings.ToLower(strings.TrimSpace(intent.Side))
	if side != "buy" && side != "sell" {
		return nil, fmt.Errorf("paper order side must be buy or sell")
	}
	if intent.Quantity <= 0 {
		return nil, fmt.Errorf("paper option quantity must be a positive whole number")
	}
	orderType := strings.ToLower(strings.TrimSpace(intent.OrderType))
	if orderType == "" {
		orderType = "limit"
	}
	priceType := "00"
	price := intent.Price
	switch orderType {
	case "limit":
		if price <= 0 || math.IsNaN(price) || math.IsInf(price, 0) {
			return nil, fmt.Errorf("paper limit price must be greater than zero")
		}
	case "market":
		priceType, price = "01", 0
	default:
		return nil, fmt.Errorf("paper order type must be limit or market")
	}
	return map[string]any{
		"stockCode": stockCode, "market": market, "currencyMode": currency,
		"tradeType": side, "price": price, "quantity": intent.Quantity,
		"orderPriceType": priceType, "max": false, "marginTrading": false,
		"isReservationOrder": false, "openPriceSinglePriceYn": false,
	}, nil
}

func cloneAnyMap(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src)+1)
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func (c *Client) ListPaperPendingOrders(ctx context.Context) ([]domain.PaperOrder, error) {
	if err := c.requireSession(); err != nil {
		return nil, err
	}
	var env paperEnvelope[json.RawMessage]
	if err := c.getJSON(ctx, c.certBaseURL+"/api/v1/paper/trading/orders/histories/all/pending", &env); err != nil {
		return nil, err
	}
	if err := env.requireResult("pending paper orders"); err != nil {
		return nil, err
	}
	return decodePaperOrders(env.Result)
}

// ListPaperCompletedOrders returns the paper-only order history rendered by the
// US-options activity screen. It never falls back to the live account history.
func (c *Client) ListPaperCompletedOrders(ctx context.Context) ([]domain.PaperOrder, error) {
	if err := c.requireSession(); err != nil {
		return nil, err
	}
	var env paperEnvelope[struct {
		Body []json.RawMessage `json:"body"`
	}]
	if err := c.getJSON(ctx, c.certBaseURL+"/api/v2/paper/trading/my-orders/markets/us-opt/by-date/completed", &env); err != nil {
		return nil, err
	}
	if err := env.requireResult("completed paper orders"); err != nil {
		return nil, err
	}
	orders := make([]domain.PaperOrder, 0, len(env.Result.Body))
	for _, row := range env.Result.Body {
		var wire struct {
			OrderID    string  `json:"orderId"`
			OrderNo    any     `json:"orderNo"`
			OrderedAt  string  `json:"orderedAt"`
			StockCode  string  `json:"stockCode"`
			StockName  string  `json:"stockName"`
			TradeType  string  `json:"tradeType"`
			Status     string  `json:"status"`
			Quantity   float64 `json:"orderQuantity"`
			OrderPrice struct {
				KRW float64 `json:"krw"`
				USD float64 `json:"usd"`
			} `json:"orderPrice"`
			ExecutedQuantity float64 `json:"executedQuantity"`
			AveragePrice     struct {
				KRW float64 `json:"krw"`
				USD float64 `json:"usd"`
			} `json:"averageExecutionPrice"`
			OrderDate        string `json:"userOrderDate"`
			AfterMarketOrder bool   `json:"afterMarketOrder"`
		}
		if err := json.Unmarshal(row, &wire); err != nil {
			return nil, fmt.Errorf("decode completed paper order: %w", err)
		}
		orderNo := normalizeOrderIdentifier(wire.OrderNo, "")
		id := wire.OrderID
		if id == "" {
			id = referenceOrderIdentifier(wire.OrderDate, wire.OrderNo, "")
		}
		orders = append(orders, domain.PaperOrder{
			ID: id, OrderID: wire.OrderID, OrderNo: orderNo, OrderDate: wire.OrderDate,
			OrderedAt: wire.OrderedAt, StockCode: wire.StockCode, StockName: wire.StockName,
			TradeType: wire.TradeType, Status: wire.Status, Quantity: wire.Quantity,
			OrderPriceKRW: wire.OrderPrice.KRW, OrderPriceUSD: wire.OrderPrice.USD,
			ExecutedQuantity: wire.ExecutedQuantity, AveragePriceKRW: wire.AveragePrice.KRW,
			AveragePriceUSD: wire.AveragePrice.USD, IsAfterMarketOrder: wire.AfterMarketOrder,
			Raw: row,
		})
	}
	return orders, nil
}

func decodePaperOrders(raw json.RawMessage) ([]domain.PaperOrder, error) {
	var rows []json.RawMessage
	if err := json.Unmarshal(raw, &rows); err != nil {
		var page struct {
			Body []json.RawMessage `json:"body"`
		}
		if pageErr := json.Unmarshal(raw, &page); pageErr != nil || page.Body == nil {
			return nil, fmt.Errorf("decode paper orders: %w", err)
		}
		rows = page.Body
	}
	orders := make([]domain.PaperOrder, 0, len(rows))
	for _, row := range rows {
		var wire struct {
			OrderID            string  `json:"orderId"`
			OrderNo            any     `json:"orderNo"`
			OrderedDate        string  `json:"orderedDate"`
			OrderedAt          string  `json:"orderedAt"`
			StockCode          string  `json:"stockCode"`
			StockName          string  `json:"stockName"`
			TradeType          string  `json:"tradeType"`
			Status             string  `json:"status"`
			Quantity           float64 `json:"quantity"`
			PendingQuantity    float64 `json:"pendingQuantity"`
			OrderPrice         float64 `json:"orderPrice"`
			OrderUSDPrice      float64 `json:"orderUsdPrice"`
			IsAfterMarketOrder bool    `json:"isAfterMarketOrder"`
			IsReservationOrder bool    `json:"isReservationOrder"`
		}
		if err := json.Unmarshal(row, &wire); err != nil {
			return nil, fmt.Errorf("decode paper order: %w", err)
		}
		date := strings.TrimSpace(wire.OrderedDate)
		if date == "" && len(wire.OrderedAt) >= 10 {
			date = wire.OrderedAt[:10]
		}
		orderNo := normalizeOrderIdentifier(wire.OrderNo, "")
		id := wire.OrderID
		if id == "" {
			id = referenceOrderIdentifier(date, wire.OrderNo, "")
		}
		orders = append(orders, domain.PaperOrder{
			ID: id, OrderID: wire.OrderID, OrderNo: orderNo, OrderDate: date,
			OrderedAt: wire.OrderedAt, StockCode: wire.StockCode, StockName: wire.StockName,
			TradeType: wire.TradeType, Status: wire.Status, Quantity: wire.Quantity,
			PendingQuantity: wire.PendingQuantity, OrderPrice: wire.OrderPrice,
			OrderPriceKRW: wire.OrderPrice, OrderPriceUSD: wire.OrderUSDPrice,
			IsAfterMarketOrder: wire.IsAfterMarketOrder, IsReservationOrder: wire.IsReservationOrder,
			Raw: row,
		})
	}
	return orders, nil
}

func (c *Client) CancelPaperOrder(ctx context.Context, orderID string) (domain.PaperMutationReceipt, error) {
	if err := c.ensureTradingMetadata(ctx); err != nil {
		return domain.PaperMutationReceipt{}, err
	}
	orders, err := c.ListPaperPendingOrders(ctx)
	if err != nil {
		return domain.PaperMutationReceipt{}, err
	}
	var selected *domain.PaperOrder
	for i := range orders {
		if paperOrderMatches(orders[i], orderID) {
			selected = &orders[i]
			break
		}
	}
	if selected == nil {
		return domain.PaperMutationReceipt{}, fmt.Errorf("pending paper order %s was not found", orderID)
	}
	body := paperCancelBody(*selected, true)
	payload, _ := json.Marshal(body)
	preparePath := "/api/v2/paper/trading/order/cancel/prepare/" + url.PathEscape(selected.OrderDate) + "/" + url.PathEscape(selected.OrderNo)
	var prepared paperEnvelope[paperPrepareWire]
	if err := c.postTradingJSON(ctx, c.certBaseURL+preparePath, payload, &prepared); err != nil {
		return domain.PaperMutationReceipt{}, err
	}
	if err := prepared.requireResult("paper cancellation prepare"); err != nil {
		return domain.PaperMutationReceipt{}, err
	}
	if prepared.Result.AuthRequired.Required {
		return domain.PaperMutationReceipt{}, fmt.Errorf("paper cancellation requires interactive server authentication")
	}
	body = paperCancelBody(*selected, false)
	payload, _ = json.Marshal(body)
	executePath := "/api/v3/paper/trading/order/cancel/" + url.PathEscape(selected.OrderDate) + "/" + url.PathEscape(selected.OrderNo)
	var cancelled paperEnvelope[paperReceiptWire]
	var headers map[string]string
	if orderKey := strings.TrimSpace(prepared.Result.OrderKey); orderKey != "" {
		headers = map[string]string{"X-Order-Key": orderKey}
	}
	if err := c.postTradingJSONWithHeaders(ctx, c.certBaseURL+executePath, payload, headers, &cancelled); err != nil {
		return domain.PaperMutationReceipt{}, err
	}
	if err := cancelled.requireResult("paper cancellation"); err != nil {
		return domain.PaperMutationReceipt{}, err
	}
	return mapPaperReceipt(cancelled.Result), nil
}

func paperOrderMatches(order domain.PaperOrder, wanted string) bool {
	wanted = strings.TrimSpace(wanted)
	return wanted != "" && (order.ID == wanted || order.OrderID == wanted || order.OrderNo == wanted || order.OrderDate+"/"+order.OrderNo == wanted)
}

func paperCancelBody(order domain.PaperOrder, withOrderKey bool) map[string]any {
	body := map[string]any{
		"isAfterMarketOrder": order.IsAfterMarketOrder,
		"quantity":           order.PendingQuantity,
		"stockCode":          order.StockCode,
		"tradeType":          order.TradeType,
		"isReservationOrder": order.IsReservationOrder,
		"orderId":            order.OrderID,
	}
	if withOrderKey {
		body["withOrderKey"] = true
	}
	return body
}

func (c *Client) BulkCancelPaperOrders(ctx context.Context, side string) (domain.PaperBulkCancelReceipt, error) {
	if err := c.ensureTradingMetadata(ctx); err != nil {
		return domain.PaperBulkCancelReceipt{}, err
	}
	orders, err := c.ListPaperPendingOrders(ctx)
	if err != nil {
		return domain.PaperBulkCancelReceipt{}, err
	}
	side = strings.ToLower(strings.TrimSpace(side))
	if side != "" && side != "buy" && side != "sell" {
		return domain.PaperBulkCancelReceipt{}, fmt.Errorf("paper bulk-cancel side must be buy, sell, or empty")
	}
	selected := make([]domain.PaperOrder, 0, len(orders))
	for _, order := range orders {
		if side == "" || strings.EqualFold(order.TradeType, side) {
			selected = append(selected, order)
		}
	}
	result := domain.PaperBulkCancelReceipt{Environment: paperEnvironment, RequestedCount: len(selected)}
	if len(selected) == 0 {
		return result, nil
	}
	body := map[string]any{"orderCancels": paperBulkCancelItems(selected)}
	payload, _ := json.Marshal(body)
	var prepared paperEnvelope[json.RawMessage]
	if err := c.postTradingJSON(ctx, c.certBaseURL+"/api/v3/paper/trading/order/bulk-cancel/prepare", payload, &prepared); err != nil {
		return domain.PaperBulkCancelReceipt{}, err
	}
	if err := prepared.requireResult("paper bulk cancellation prepare"); err != nil {
		return domain.PaperBulkCancelReceipt{}, err
	}
	var executed paperEnvelope[struct {
		FailedCancelCount int `json:"failedCancelCount"`
	}]
	if err := c.postTradingJSON(ctx, c.certBaseURL+"/api/v3/paper/trading/order/bulk-cancel", payload, &executed); err != nil {
		return domain.PaperBulkCancelReceipt{}, err
	}
	if err := executed.requireResult("paper bulk cancellation"); err != nil {
		return domain.PaperBulkCancelReceipt{}, err
	}
	result.FailedCount = executed.Result.FailedCancelCount
	return result, nil
}

func paperBulkCancelItems(orders []domain.PaperOrder) []map[string]any {
	items := make([]map[string]any, 0, len(orders))
	for _, order := range orders {
		items = append(items, map[string]any{
			"orderDate": order.OrderDate, "orderNo": parseNumericString(order.OrderNo),
			"tradeType": order.TradeType, "isAfterMarketOrder": order.IsAfterMarketOrder,
			"stockCode": order.StockCode, "isReservationOrder": order.IsReservationOrder,
		})
	}
	return items
}

func parseNumericString(value string) any {
	if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
		return parsed
	}
	return value
}
