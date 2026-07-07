package official

import (
	"context"
	"net/url"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// CancelConditionalOrder cancels a conditional order by id (account-scoped).
func (c *Client) CancelConditionalOrder(ctx context.Context, id string) error {
	return c.deleteAcct(ctx, "/api/v1/conditional-orders/"+url.PathEscape(id), nil)
}

// ConditionLegBody mirrors ConditionRequest (string decimals; orderPrice
// omitted for MARKET). Exported so the hybrid layer can construct it.
type ConditionLegBody struct {
	OrderSide    string `json:"orderSide"`
	TriggerPrice string `json:"triggerPrice"`
	OrderPrice   string `json:"orderPrice,omitempty"`
}

// ConditionalCreateBody mirrors ConditionalOrderCreateRequest.
type ConditionalCreateBody struct {
	Symbol                string            `json:"symbol"`
	Type                  string            `json:"type"`
	Quantity              string            `json:"quantity"`
	OrderType             string            `json:"orderType"`
	ClientOrderID         string            `json:"clientOrderId,omitempty"`
	ExpireDate            string            `json:"expireDate"`
	First                 ConditionLegBody  `json:"first"`
	Second                *ConditionLegBody `json:"second,omitempty"`
	ConfirmHighValueOrder bool              `json:"confirmHighValueOrder"`
}

// apiConditionalCreateResponse mirrors ConditionalOrderCreateResponse (unwrapped result).
type apiConditionalCreateResponse struct {
	ConditionalOrderID string `json:"conditionalOrderId"`
	ClientOrderID      string `json:"clientOrderId"`
}

// CreateConditionalOrder creates a conditional order (account-scoped, POST).
func (c *Client) CreateConditionalOrder(ctx context.Context, body ConditionalCreateBody) (domain.ConditionalOrderRef, error) {
	var raw apiConditionalCreateResponse
	if err := c.postAcct(ctx, "/api/v1/conditional-orders", body, &raw); err != nil {
		return domain.ConditionalOrderRef{}, err
	}
	return domain.ConditionalOrderRef{ID: raw.ConditionalOrderID, ClientOrderID: raw.ClientOrderID}, nil
}

// ConditionalModifyBody mirrors ConditionalOrderModifyRequest (no symbol/clientOrderId).
type ConditionalModifyBody struct {
	Type                  string            `json:"type"`
	Quantity              string            `json:"quantity"`
	OrderType             string            `json:"orderType"`
	ExpireDate            string            `json:"expireDate"`
	First                 ConditionLegBody  `json:"first"`
	Second                *ConditionLegBody `json:"second,omitempty"`
	ConfirmHighValueOrder bool              `json:"confirmHighValueOrder"`
}

// ModifyConditionalOrder modifies an existing conditional order (POST /modify).
func (c *Client) ModifyConditionalOrder(ctx context.Context, id string, body ConditionalModifyBody) error {
	return c.postAcct(ctx, "/api/v1/conditional-orders/"+url.PathEscape(id)+"/modify", body, nil)
}
