package official

import (
	"context"
	"net/url"
	"strconv"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// apiConditionalCondition mirrors ConditionalOrderCondition (nullable numeric
// fields arrive as "" when JSON null).
type apiConditionalCondition struct {
	Type             string `json:"type"`
	Status           string `json:"status"`
	TriggerPrice     string `json:"triggerPrice"`
	TargetProfitRate string `json:"targetProfitRate"`
	OrderPrice       string `json:"orderPrice"`
	TriggeredOrderID string `json:"triggeredOrderId"`
}

// apiConditionalOrder mirrors ConditionalOrderDetailResponse. second is a
// pointer so a JSON null (SINGLE) stays nil rather than a zero-value leg.
type apiConditionalOrder struct {
	ConditionalOrderID string                   `json:"conditionalOrderId"`
	Type               string                   `json:"type"`
	Status             string                   `json:"status"`
	Symbol             string                   `json:"symbol"`
	Market             string                   `json:"market"`
	Quantity           string                   `json:"quantity"`
	OrderType          string                   `json:"orderType"`
	ExpireDate         string                   `json:"expireDate"`
	First              apiConditionalCondition  `json:"first"`
	Second             *apiConditionalCondition `json:"second"`
	CreatedAt          string                   `json:"createdAt"`
}

// apiConditionalOrderPage mirrors PaginatedConditionalOrderResponse.
type apiConditionalOrderPage struct {
	ConditionalOrders []apiConditionalOrder `json:"conditionalOrders"`
	NextCursor        string                `json:"nextCursor"`
	HasNext           bool                  `json:"hasNext"`
}

// ConditionalOrders lists conditional orders (account-scoped, official-only).
// status/symbol/cursor/limit are optional filters (empty/0 → omitted).
func (c *Client) ConditionalOrders(ctx context.Context, status, symbol, cursor string, limit int) (domain.ConditionalOrderList, error) {
	q := url.Values{}
	if status != "" {
		q.Set("status", status)
	}
	if symbol != "" {
		q.Set("symbol", symbol)
	}
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	var raw apiConditionalOrderPage
	if err := c.getAcct(ctx, "/api/v1/conditional-orders", q, &raw); err != nil {
		return domain.ConditionalOrderList{}, err
	}
	orders := make([]domain.ConditionalOrder, 0, len(raw.ConditionalOrders))
	for _, o := range raw.ConditionalOrders {
		orders = append(orders, adaptConditionalOrder(o))
	}
	return domain.ConditionalOrderList{
		Orders:     orders,
		NextCursor: raw.NextCursor,
		HasNext:    raw.HasNext,
		FetchedAt:  time.Now().UTC(),
	}, nil
}

// ConditionalOrder fetches one conditional order by id (account-scoped).
func (c *Client) ConditionalOrder(ctx context.Context, id string) (domain.ConditionalOrder, error) {
	var raw apiConditionalOrder
	if err := c.getAcct(ctx, "/api/v1/conditional-orders/"+url.PathEscape(id), nil, &raw); err != nil {
		return domain.ConditionalOrder{}, err
	}
	return adaptConditionalOrder(raw), nil
}

// adaptCondition converts one raw watch leg to domain.
func adaptCondition(a apiConditionalCondition) domain.ConditionalOrderCondition {
	return domain.ConditionalOrderCondition{
		Type:             a.Type,
		Status:           a.Status,
		TriggerPrice:     parseDecimal(a.TriggerPrice),
		TargetProfitRate: parseDecimal(a.TargetProfitRate),
		OrderPrice:       parseDecimal(a.OrderPrice),
		TriggeredOrderID: a.TriggeredOrderID,
	}
}

// adaptConditionalOrder converts the official detail response to domain.
// second stays nil for SINGLE (raw pointer nil).
func adaptConditionalOrder(o apiConditionalOrder) domain.ConditionalOrder {
	out := domain.ConditionalOrder{
		ID:         o.ConditionalOrderID,
		Type:       o.Type,
		Status:     o.Status,
		Symbol:     o.Symbol,
		Market:     o.Market,
		Quantity:   parseDecimal(o.Quantity),
		OrderType:  o.OrderType,
		ExpireDate: o.ExpireDate,
		First:      adaptCondition(o.First),
		CreatedAt:  o.CreatedAt,
	}
	if o.Second != nil {
		s := adaptCondition(*o.Second)
		out.Second = &s
	}
	return out
}
