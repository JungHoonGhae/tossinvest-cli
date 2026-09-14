package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/orderlineage"
)

type completedOrdersEnvelope struct {
	Result struct {
		Body []json.RawMessage `json:"body"`
	} `json:"result"`
}

func (c *Client) ListCompletedOrders(ctx context.Context, market string) ([]domain.Order, error) {
	now := time.Now()
	from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	return c.ListCompletedOrdersRange(ctx, market, from, now, 50, 1)
}

func (c *Client) ListCompletedOrdersRange(ctx context.Context, market string, from, to time.Time, size, number int) ([]domain.Order, error) {
	if err := c.requireSession(); err != nil {
		return nil, err
	}
	if err := c.ensureTradingMetadata(ctx); err != nil {
		return nil, err
	}

	markets, err := normalizeHistoryMarkets(market)
	if err != nil {
		return nil, err
	}

	orders := make([]domain.Order, 0)
	for _, entry := range markets {
		var envelope completedOrdersEnvelope
		endpoint := fmt.Sprintf(
			"%s/api/v2/trading/my-orders/markets/%s/by-date/completed?range.from=%s&range.to=%s&size=%d&number=%d",
			c.certBaseURL,
			entry,
			from.Format("2006-01-02"),
			to.Format("2006-01-02"),
			size,
			number,
		)
		if err := c.getJSON(ctx, endpoint, &envelope); err != nil {
			return nil, err
		}
		if envelope.Result.Body == nil {
			return nil, fmt.Errorf("invalid completed orders response: result.body must be an array")
		}

		for _, item := range envelope.Result.Body {
			orders = append(orders, parseCompletedOrder(item, entry))
		}
	}

	sort.SliceStable(orders, func(i, j int) bool {
		if orders[i].SubmittedAt == nil && orders[j].SubmittedAt == nil {
			return orders[i].ID > orders[j].ID
		}
		if orders[i].SubmittedAt == nil {
			return false
		}
		if orders[j].SubmittedAt == nil {
			return true
		}
		return orders[i].SubmittedAt.After(*orders[j].SubmittedAt)
	})

	return orders, nil
}

func (c *Client) FindOrder(ctx context.Context, orderID string, market string) (domain.Order, error) {
	return c.FindOrderWithAliases(ctx, orderID, market)
}

func (c *Client) FindOrderWithAliases(ctx context.Context, orderID string, market string, aliases ...string) (domain.Order, error) {
	candidates := uniqueOrderLookupCandidates(orderID, aliases...)

	pendingOrders, err := c.ListPendingOrders(ctx)
	if err != nil {
		return domain.Order{}, err
	}
	if order, ok := findOrderInCollection(pendingOrders, orderID, candidates); ok {
		return order, nil
	}

	completedOrders, err := c.ListCompletedOrders(ctx, market)
	if err != nil {
		return domain.Order{}, err
	}
	if order, ok := findOrderInCollection(completedOrders, orderID, candidates); ok {
		return order, nil
	}

	return domain.Order{}, fmt.Errorf("order %s was not found in pending or current-month completed history", orderID)
}

func (c *Client) FindCompletedOrderFromLineageHint(ctx context.Context, requestedOrderID string, market string, hint orderlineage.Entry) (domain.Order, bool, error) {
	if strings.TrimSpace(hint.Kind) != "cancel" {
		return domain.Order{}, false, nil
	}

	lookupMarket := strings.TrimSpace(market)
	if lookupMarket == "" || strings.EqualFold(lookupMarket, "all") {
		if strings.TrimSpace(hint.Market) != "" {
			lookupMarket = hint.Market
		} else {
			lookupMarket = "all"
		}
	}

	orders, err := c.ListCompletedOrders(ctx, lookupMarket)
	if err != nil {
		return domain.Order{}, false, err
	}

	earliestSubmittedAt := time.Time{}
	if !hint.UpdatedAt.IsZero() {
		earliestSubmittedAt = hint.UpdatedAt.Add(-mutationCompletedLookback)
	}

	candidates := make([]domain.Order, 0, 1)
	for _, order := range orders {
		if !matchesDelayedCancelRecoveryHint(order, requestedOrderID, hint, earliestSubmittedAt) {
			continue
		}
		candidates = append(candidates, order)
	}

	switch len(candidates) {
	case 0:
		return domain.Order{}, false, nil
	case 1:
		order := candidates[0]
		order.ResolvedFromID = requestedOrderID
		return order, true, nil
	default:
		return domain.Order{}, false, fmt.Errorf("order %s has multiple delayed cancel rollover candidates; inspect `tossctl orders completed` first", requestedOrderID)
	}
}

func uniqueOrderLookupCandidates(orderID string, aliases ...string) []string {
	candidates := make([]string, 0, len(aliases)+1)
	seen := map[string]struct{}{}
	for _, candidate := range append([]string{orderID}, aliases...) {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		candidates = append(candidates, candidate)
	}
	return candidates
}

func findOrderInCollection(orders []domain.Order, requestedOrderID string, candidates []string) (domain.Order, bool) {
	for _, candidate := range candidates {
		for _, order := range orders {
			if order.ID != candidate && !orderMatchesID(order.Raw, candidate) {
				continue
			}
			if candidate != requestedOrderID {
				order.ResolvedFromID = requestedOrderID
			}
			return order, true
		}
	}
	return domain.Order{}, false
}

func matchesDelayedCancelRecoveryHint(order domain.Order, requestedOrderID string, hint orderlineage.Entry, earliestSubmittedAt time.Time) bool {
	if order.ID == requestedOrderID || orderMatchesID(order.Raw, requestedOrderID) {
		return false
	}
	if !earliestSubmittedAt.IsZero() {
		if order.SubmittedAt != nil {
			if order.SubmittedAt.Before(earliestSubmittedAt) {
				return false
			}
		} else if hint.OrderDate != "" && order.OrderDate != hint.OrderDate {
			return false
		}
	}
	if hint.OrderDate != "" && order.OrderDate != "" && order.OrderDate != hint.OrderDate {
		return false
	}
	if hint.Symbol != "" &&
		!strings.EqualFold(order.Symbol, hint.Symbol) &&
		!strings.EqualFold(order.Name, hint.Symbol) {
		return false
	}
	if hint.Market != "" && order.Market != "" && !strings.EqualFold(order.Market, hint.Market) {
		return false
	}
	if !orderStatusLooksCanceled(order.Status) {
		return false
	}
	if hint.Quantity != 0 && !equalFloat(order.Quantity, hint.Quantity) && !equalFloat(order.FilledQuantity, hint.Quantity) {
		return false
	}
	if hint.Price != 0 && !equalFloat(order.Price, hint.Price) {
		return false
	}
	return true
}

func parseCompletedOrder(raw json.RawMessage, market string) domain.Order {
	order := domain.Order{Raw: raw}

	var payload struct {
		OrderedAt      string `json:"orderedAt"`
		LastExecutedAt string `json:"lastExecutedAt"`
		Version        string `json:"version"`
		OrderNo        any    `json:"orderNo"`
		OrderID        string `json:"orderId"`

		StockCode string `json:"stockCode"`
		StockName string `json:"stockName"`
		Symbol    string `json:"symbol"`
		TradeType string `json:"tradeType"`
		Status    string `json:"status"`

		OrderQuantity    float64 `json:"orderQuantity"`
		ExecutedQuantity float64 `json:"executedQuantity"`
		UserOrderDate    string  `json:"userOrderDate"`

		OrderPrice struct {
			KRW float64 `json:"krw"`
		} `json:"orderPrice"`
		AverageExecutionPrice struct {
			KRW float64 `json:"krw"`
		} `json:"averageExecutionPrice"`
	}

	if err := json.Unmarshal(raw, &payload); err != nil {
		return order
	}

	order.ID = referenceOrderIdentifier(payload.UserOrderDate, payload.OrderNo, payload.OrderID)
	order.Symbol = firstNonEmpty(payload.Symbol, payload.StockCode)
	order.Name = payload.StockName
	order.Market = strings.ToLower(market)
	order.Side = payload.TradeType
	order.Status = payload.Status
	order.Quantity = payload.OrderQuantity
	order.FilledQuantity = payload.ExecutedQuantity
	order.Price = payload.OrderPrice.KRW
	order.AverageExecutionPrice = payload.AverageExecutionPrice.KRW
	order.OrderDate = payload.UserOrderDate
	order.SubmittedAt = parseOrderTime(payload.LastExecutedAt, payload.Version, payload.OrderedAt)
	return order
}

func normalizeHistoryMarkets(value string) ([]string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "all":
		return []string{"us", "kr"}, nil
	case "us", "kr":
		return []string{strings.ToLower(strings.TrimSpace(value))}, nil
	default:
		return nil, fmt.Errorf("unsupported market %q; use us, kr, or all", value)
	}
}
