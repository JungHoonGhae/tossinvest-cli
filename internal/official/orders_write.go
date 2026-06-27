package official

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/JungHoonGhae/tossinvest-cli/internal/orderintent"
	trading "github.com/JungHoonGhae/tossinvest-cli/internal/trading"
)

// ---------------------------------------------------------------------------
// Wire-format types
// ---------------------------------------------------------------------------

// orderCreateV0 is the quantity-based variant of OrderCreateRequest.
// Covers: non-fractional LIMIT, non-fractional MARKET, fractional SELL.
//
// Schema: OrderCreateQuantityBased (openapi.latest.json)
// Required: symbol, side, orderType, quantity.
// Optional: price (LIMIT only), timeInForce (LIMIT only).
type orderCreateV0 struct {
	Symbol                string `json:"symbol"`
	Side                  string `json:"side"`
	OrderType             string `json:"orderType"`
	Quantity              string `json:"quantity"`
	Price                 string `json:"price,omitempty"`
	TimeInForce           string `json:"timeInForce,omitempty"`
	ConfirmHighValueOrder bool   `json:"confirmHighValueOrder"`
}

// orderCreateV1 is the amount-based variant of OrderCreateRequest.
// Covers: fractional BUY only.
//
// Schema: OrderCreateAmountBased (openapi.latest.json)
// Required: symbol, side, orderType=MARKET, orderAmount.
type orderCreateV1 struct {
	Symbol                string `json:"symbol"`
	Side                  string `json:"side"`
	OrderType             string `json:"orderType"`
	OrderAmount           string `json:"orderAmount"`
	ConfirmHighValueOrder bool   `json:"confirmHighValueOrder"`
}

// apiOrderCreateResponse mirrors the OrderResponse schema returned by
// POST /api/v1/orders (inside the {"result": ...} envelope).
type apiOrderCreateResponse struct {
	OrderID string `json:"orderId"`
}

// apiOrderOperationResponse mirrors the OrderOperationResponse schema returned
// by POST /api/v1/orders/{orderId}/cancel and /modify.
type apiOrderOperationResponse struct {
	OrderID string `json:"orderId"`
}

// orderModifyRequest mirrors the OrderModifyRequest schema.
// Required: orderType.
// Optional: price (LIMIT only), quantity (KR only).
type orderModifyRequest struct {
	OrderType             string `json:"orderType"`
	Price                 string `json:"price,omitempty"`
	Quantity              string `json:"quantity,omitempty"`
	ConfirmHighValueOrder bool   `json:"confirmHighValueOrder"`
}

// ---------------------------------------------------------------------------
// Builder
// ---------------------------------------------------------------------------

// buildOrderCreate constructs the correct OrderCreateRequest variant from a
// PlaceIntent.
//
// Variant mapping table:
//
//	Intent                | Variant | orderType | Key fields
//	----------------------|---------|-----------|-------------------------------
//	fractional BUY        | 1       | MARKET    | orderAmount (USD decimal str)
//	fractional SELL       | 0       | MARKET    | quantity (decimal str), no price
//	non-fractional LIMIT  | 0       | LIMIT     | price + quantity + timeInForce=DAY
//	non-fractional MARKET | 0       | MARKET    | quantity only
//
// side is uppercased from intent (intent.Side is lowercase "buy"/"sell").
// All numeric values are formatted as shortest decimal strings.
func buildOrderCreate(intent orderintent.PlaceIntent) (any, error) {
	side := strings.ToUpper(intent.Side)

	if intent.Fractional && intent.Side == "buy" {
		// variant1: amount-based (US MARKET fractional buy)
		return orderCreateV1{
			Symbol:                intent.Symbol,
			Side:                  side,
			OrderType:             "MARKET",
			OrderAmount:           formatDecimal(intent.Amount),
			ConfirmHighValueOrder: false,
		}, nil
	}

	// variant0: quantity-based
	v0 := orderCreateV0{
		Symbol:                intent.Symbol,
		Side:                  side,
		ConfirmHighValueOrder: false,
	}

	if intent.Fractional && intent.Side == "sell" {
		// fractional sell: MARKET, decimal quantity, no price
		v0.OrderType = "MARKET"
		v0.Quantity = formatDecimal(intent.Quantity)
		return v0, nil
	}

	orderType := strings.ToUpper(intent.OrderType)
	v0.OrderType = orderType
	v0.Quantity = formatDecimal(intent.Quantity)

	if orderType == "LIMIT" {
		v0.Price = formatDecimal(intent.Price)
		v0.TimeInForce = "DAY"
	}

	return v0, nil
}

// formatDecimal converts a float64 to its shortest decimal string.
// Examples: 100 → "100", 0.5 → "0.5", 150.25 → "150.25".
func formatDecimal(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// acctHeaders returns the extra-headers map for account-scoped order endpoints.
// Returns nil when accountSeq is 0 (header omitted).
func (c *Client) acctHeaders() map[string]string {
	if c.accountSeq == 0 {
		return nil
	}
	return map[string]string{"X-Tossinvest-Account": strconv.Itoa(c.accountSeq)}
}

// ---------------------------------------------------------------------------
// Mutation methods
// ---------------------------------------------------------------------------

// PlaceOrder submits a new order via the official Open API.
//
// Endpoint: POST /api/v1/orders
// Response: OrderResponse → orderId
// Sends X-Tossinvest-Account when accountSeq is set.
func (c *Client) PlaceOrder(ctx context.Context, intent orderintent.PlaceIntent) (trading.MutationResult, error) {
	body, err := buildOrderCreate(intent)
	if err != nil {
		return trading.MutationResult{}, fmt.Errorf("building order body: %w", err)
	}
	var resp apiOrderCreateResponse
	if err := c.postWithHeaders(ctx, "/api/v1/orders", body, c.acctHeaders(), &resp); err != nil {
		return trading.MutationResult{}, err
	}
	return trading.MutationResult{
		Kind:    "place",
		Status:  "accepted",
		OrderID: resp.OrderID,
	}, nil
}

// CancelOrder cancels an existing order by its orderId via the official Open API.
//
// Endpoint: POST /api/v1/orders/{orderId}/cancel
// Response: OrderOperationResponse → orderId (new identifier issued for the cancel)
// Sends X-Tossinvest-Account when accountSeq is set.
// The orderId path segment is percent-encoded via url.PathEscape.
func (c *Client) CancelOrder(ctx context.Context, orderID string) (trading.MutationResult, error) {
	path := "/api/v1/orders/" + url.PathEscape(orderID) + "/cancel"
	var resp apiOrderOperationResponse
	if err := c.postWithHeaders(ctx, path, struct{}{}, c.acctHeaders(), &resp); err != nil {
		return trading.MutationResult{}, err
	}
	return trading.MutationResult{
		Kind:    "cancel",
		Status:  "accepted",
		OrderID: resp.OrderID,
	}, nil
}

// ModifyOrder amends an existing order via the official Open API.
//
// Endpoint: POST /api/v1/orders/{orderId}/modify
// Response: OrderOperationResponse → orderId
// Sends X-Tossinvest-Account when accountSeq is set.
//
// orderType inference:
//   - intent.Price != nil → LIMIT (price is required for LIMIT)
//   - intent.Price == nil → MARKET
//
// quantity is forwarded when set (required for KR stocks, forbidden for US).
func (c *Client) ModifyOrder(ctx context.Context, intent orderintent.AmendIntent) (trading.MutationResult, error) {
	body := orderModifyRequest{
		ConfirmHighValueOrder: false,
	}
	if intent.Price != nil {
		body.OrderType = "LIMIT"
		body.Price = formatDecimal(*intent.Price)
	} else {
		body.OrderType = "MARKET"
	}
	if intent.Quantity != nil {
		body.Quantity = formatDecimal(*intent.Quantity)
	}

	path := "/api/v1/orders/" + url.PathEscape(intent.OrderID) + "/modify"
	var resp apiOrderOperationResponse
	if err := c.postWithHeaders(ctx, path, body, c.acctHeaders(), &resp); err != nil {
		return trading.MutationResult{}, err
	}
	return trading.MutationResult{
		Kind:    "amend",
		Status:  "accepted",
		OrderID: resp.OrderID,
	}, nil
}
