package official

import (
	"context"
	"net/url"
)

// CancelConditionalOrder cancels a conditional order by id (account-scoped).
func (c *Client) CancelConditionalOrder(ctx context.Context, id string) error {
	return c.deleteAcct(ctx, "/api/v1/conditional-orders/"+url.PathEscape(id), nil)
}
