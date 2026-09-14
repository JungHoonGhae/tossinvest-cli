package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

// ListCompletedOrdersAllDates reads the unified history without a date filter.
// The upstream API requires prior-page cursors, so reaching page N takes N reads.
func (c *Client) ListCompletedOrdersAllDates(ctx context.Context, market string, size, page int) ([]domain.Order, error) {
	if size < 1 || size > 100 || page < 1 || page > 100 {
		return nil, fmt.Errorf("all-dates history requires size and page between 1 and 100")
	}
	markets, err := normalizeHistoryMarkets(market)
	if err != nil {
		return nil, err
	}
	if err := c.requireSession(); err != nil {
		return nil, err
	}
	accountKey, err := c.primaryAccountKey(ctx)
	if err != nil {
		return nil, err
	}
	query := url.Values{"executedOnly": {"false"}, "size": {strconv.Itoa(size)}}
	if len(markets) == 1 {
		query.Set("market", markets[0])
	}
	seenCursors := make(map[string]bool)
	for current := 1; current <= page; current++ {
		query.Set("number", strconv.Itoa(current))
		var envelope struct {
			Result struct {
				Body        []json.RawMessage `json:"body"`
				LastPage    *bool             `json:"lastPage"`
				PagingParam *struct {
					Number int    `json:"number"`
					Size   int    `json:"size"`
					Key    string `json:"key"`
				} `json:"pagingParam"`
			} `json:"result"`
		}
		endpoint := c.certBaseURL + "/api/v3/trading/my-orders/completed?" + query.Encode()
		if err := c.getJSONWithAccountKey(ctx, endpoint, accountKey, &envelope); err != nil {
			return nil, err
		}
		r := envelope.Result
		if r.Body == nil || r.LastPage == nil || r.PagingParam == nil {
			return nil, fmt.Errorf("invalid all-dates history response: expected result.body array, lastPage and pagingParam")
		}
		if r.PagingParam.Number != current || r.PagingParam.Size != size {
			return nil, fmt.Errorf("invalid all-dates history response: pagination does not match the request")
		}
		if !*r.LastPage && (r.PagingParam.Key == "" || seenCursors[r.PagingParam.Key]) {
			return nil, fmt.Errorf("invalid all-dates history response: missing or repeated next-page cursor")
		}
		if current == page {
			orders := make([]domain.Order, 0, len(r.Body))
			for _, raw := range r.Body {
				var row struct {
					Market string `json:"market"`
				}
				if json.Unmarshal(raw, &row) != nil || strings.TrimSpace(row.Market) == "" {
					return nil, fmt.Errorf("invalid all-dates history response: order market is missing")
				}
				order := parseCompletedOrder(raw, row.Market)
				if order.ID == "" {
					return nil, fmt.Errorf("invalid all-dates history response: order identifier is missing")
				}
				orders = append(orders, order)
			}
			return orders, nil
		}
		if *r.LastPage {
			return []domain.Order{}, nil
		}
		seenCursors[r.PagingParam.Key] = true
		query.Set("key", r.PagingParam.Key)
	}
	return nil, fmt.Errorf("all-dates history pagination did not reach the requested page")
}
