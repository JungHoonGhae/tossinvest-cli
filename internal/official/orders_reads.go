package official

import (
	"context"
	"net/url"
	"strconv"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// apiOrderExecution mirrors the execution sub-object inside an Order.
// All fields are nullable in the API (null while order is pending).
type apiOrderExecution struct {
	FilledQuantity     string `json:"filledQuantity"`
	AverageFilledPrice string `json:"averageFilledPrice"`
	FilledAmount       string `json:"filledAmount"`
	Commission         string `json:"commission"`
	Tax                string `json:"tax"`
	FilledAt           string `json:"filledAt"`
	SettlementDate     string `json:"settlementDate"`
}

// apiOrder mirrors the Order schema in PaginatedOrderResponse / single-order
// response.
// Endpoint: GET /api/v1/orders and GET /api/v1/orders/{orderId}
// Schema:
//
//	orderId     string — unique order identifier
//	symbol      string — ticker
//	side        string — "BUY" | "SELL"
//	orderType   string — "LIMIT" | "MARKET" etc.
//	timeInForce string — "DAY" etc.
//	status      string — "OPEN" | "CLOSED" etc.
//	quantity    string (decimal) — ordered quantity
//	price       string (decimal, nullable) — limit price; null for market orders
//	currency    string — "KRW" | "USD"
//	orderedAt   string (datetime, nullable) — order submission time (RFC3339)
//	canceledAt  string (datetime, nullable)
//	orderAmount string (decimal, nullable)
//	execution   apiOrderExecution
type apiOrder struct {
	OrderID     string            `json:"orderId"`
	Symbol      string            `json:"symbol"`
	Side        string            `json:"side"`
	OrderType   string            `json:"orderType"`
	TimeInForce string            `json:"timeInForce"`
	Status      string            `json:"status"`
	Quantity    string            `json:"quantity"`
	Price       string            `json:"price"`
	Currency    string            `json:"currency"`
	OrderedAt   string            `json:"orderedAt"`
	CanceledAt  string            `json:"canceledAt"`
	OrderAmount string            `json:"orderAmount"`
	Execution   apiOrderExecution `json:"execution"`
}

// apiOrderPage mirrors the PaginatedOrderResponse envelope.
type apiOrderPage struct {
	Orders     []apiOrder `json:"orders"`
	NextCursor string     `json:"nextCursor"`
	HasNext    bool       `json:"hasNext"`
}

// OrdersFilter specifies optional filters for the Orders list endpoint.
// Zero-value fields are omitted from the request query string.
type OrdersFilter struct {
	// Status filters orders by lifecycle state ("OPEN" | "CLOSED").
	Status string
	// Symbol restricts to a specific ticker.
	Symbol string
	// From is the start date (inclusive) in "YYYY-MM-DD" format.
	From string
	// To is the end date (inclusive) in "YYYY-MM-DD" format.
	To string
	// Cursor is the pagination cursor returned by a previous call's NextCursor.
	Cursor string
	// Limit is the maximum number of orders per page (0 = API default).
	Limit int
}

// Orders fetches the order history for the authenticated account.
// filter may be a zero-value OrdersFilter to use API defaults.
// Requires the X-Tossinvest-Account header; configure via WithAccountSeq.
func (c *Client) Orders(ctx context.Context, filter OrdersFilter) ([]domain.Order, error) {
	q := url.Values{}
	if filter.Status != "" {
		q.Set("status", filter.Status)
	}
	if filter.Symbol != "" {
		q.Set("symbol", filter.Symbol)
	}
	if filter.From != "" {
		q.Set("from", filter.From)
	}
	if filter.To != "" {
		q.Set("to", filter.To)
	}
	if filter.Cursor != "" {
		q.Set("cursor", filter.Cursor)
	}
	if filter.Limit > 0 {
		q.Set("limit", strconv.Itoa(filter.Limit))
	}

	var raw apiOrderPage
	if err := c.getAcct(ctx, "/api/v1/orders", q, &raw); err != nil {
		return nil, err
	}
	return adaptOrders(raw.Orders), nil
}

// OrderByID fetches a single order by its unique ID.
// Requires the X-Tossinvest-Account header; configure via WithAccountSeq.
func (c *Client) OrderByID(ctx context.Context, orderID string) (domain.Order, error) {
	var raw apiOrder
	if err := c.getAcct(ctx, "/api/v1/orders/"+orderID, nil, &raw); err != nil {
		return domain.Order{}, err
	}
	return adaptOrder(raw), nil
}

// adaptOrders converts a slice of official Order records to []domain.Order.
func adaptOrders(raw []apiOrder) []domain.Order {
	out := make([]domain.Order, 0, len(raw))
	for _, o := range raw {
		out = append(out, adaptOrder(o))
	}
	return out
}

// adaptOrder converts a single official Order to domain.Order.
//
// Mapping rationale (cross-referenced with internal/client/order.go WTS adapter):
//
//   - orderId → ID: direct pass-through.
//
//   - symbol → Symbol: direct pass-through.
//
//   - side → Side: "BUY" | "SELL", same as WTS.
//
//   - status → Status: "OPEN" | "CLOSED" etc.
//
//   - quantity (decimal string) → Quantity: parseDecimal.
//
//   - price (nullable decimal string) → Price: parseDecimal (0 for null/market).
//
//   - orderedAt → OrderDate: date portion (first 10 chars of RFC3339 string).
//     Also parsed to *time.Time for SubmittedAt if valid RFC3339.
//
//   - execution.filledQuantity → FilledQuantity: parseDecimal.
//
//   - execution.averageFilledPrice (nullable) → AverageExecutionPrice: parseDecimal.
//
//   - Name, Market: not available from this endpoint; left empty.
func adaptOrder(raw apiOrder) domain.Order {
	var orderDate string
	var submittedAt *time.Time
	if raw.OrderedAt != "" {
		if len(raw.OrderedAt) >= 10 {
			orderDate = raw.OrderedAt[:10]
		} else {
			orderDate = raw.OrderedAt
		}
		if t, err := time.Parse(time.RFC3339, raw.OrderedAt); err == nil {
			submittedAt = &t
		}
	}

	return domain.Order{
		ID:                    raw.OrderID,
		Symbol:                raw.Symbol,
		Side:                  raw.Side,
		Status:                raw.Status,
		Quantity:              parseDecimal(raw.Quantity),
		Price:                 parseDecimal(raw.Price),
		FilledQuantity:        parseDecimal(raw.Execution.FilledQuantity),
		AverageExecutionPrice: parseDecimal(raw.Execution.AverageFilledPrice),
		OrderDate:             orderDate,
		SubmittedAt:           submittedAt,
		// Name, Market — not available from /orders response
	}
}
