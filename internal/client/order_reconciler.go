package client

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/trading"
)

const (
	mutationReconcileAttempts = 8
	mutationReconcileInterval = 250 * time.Millisecond
	mutationCompletedLookback = 2 * time.Minute
)

// orderHistory only observes a request already submitted by the caller; it
// cannot retry a mutation. *Client and scripted histories implement this seam.
type orderHistory interface {
	ListPendingOrders(context.Context) ([]domain.Order, error)
	ListCompletedOrders(context.Context, string) ([]domain.Order, error)
}

type reconciliationRequest struct {
	kind, originalOrderID       string
	productCode, symbol, market string
	side                        string
	priceKRW, quantity          float64
	startedAt                   time.Time
}

// orderReconciler owns history matching, bounded observation and result
// classification for place, amend and cancel. Callers supply order facts,
// not callbacks that decide whether the mutation succeeded.
type orderReconciler struct {
	history         orderHistory
	attempts        int
	interval        time.Duration
	completedWindow time.Duration
}

func newOrderReconciler(history orderHistory) orderReconciler {
	return orderReconciler{history: history, attempts: mutationReconcileAttempts,
		interval: mutationReconcileInterval, completedWindow: mutationCompletedLookback}
}

type orderObservation struct {
	order           *domain.Order
	completed       bool
	originalPending bool
	ambiguous       bool
}

func (r orderReconciler) reconcile(ctx context.Context, req reconciliationRequest) (trading.MutationResult, error) {
	switch req.kind {
	case "place", "amend", "cancel":
	default:
		return trading.MutationResult{}, fmt.Errorf("unsupported reconciliation kind %q", req.kind)
	}
	earliest := req.startedAt.Add(-r.completedWindow)
	var observation orderObservation
	for attempt := 0; attempt < r.attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return trading.MutationResult{}, err
		}
		var err error
		observation, err = r.observe(ctx, req, earliest)
		if err != nil {
			return trading.MutationResult{}, err
		}
		if observation.order != nil {
			return req.result(*observation.order, observation.completed), nil
		}
		if attempt+1 < r.attempts {
			timer := time.NewTimer(r.interval)
			select {
			case <-ctx.Done():
				timer.Stop()
				return trading.MutationResult{}, ctx.Err()
			case <-timer.C:
			}
		}
	}
	if req.kind == "cancel" && observation.originalPending {
		return trading.MutationResult{}, trading.ErrCancelStillPending
	}
	result := req.unknown()
	if observation.ambiguous {
		result.Warnings = append(result.Warnings, "Multiple matching orders were found; no order ID was selected. Inspect order history before retrying.")
	}
	return result, nil
}

func (r orderReconciler) observe(ctx context.Context, req reconciliationRequest, earliest time.Time) (orderObservation, error) {
	pending, err := r.history.ListPendingOrders(ctx)
	if err != nil {
		return orderObservation{}, err
	}
	if req.kind == "cancel" {
		for _, order := range pending {
			if order.ID == req.originalOrderID || orderMatchesID(order.Raw, req.originalOrderID) {
				return orderObservation{originalPending: true}, nil
			}
		}
	} else {
		var match *domain.Order
		for _, order := range pending {
			if req.originalOrderID != "" && (order.ID == req.originalOrderID || orderMatchesID(order.Raw, req.originalOrderID)) {
				continue
			}
			if req.matchesOrder(order) && equalFloat(order.Quantity, req.quantity) {
				if match != nil {
					return orderObservation{ambiguous: true}, nil
				}
				match = &order
			}
		}
		if match != nil {
			return orderObservation{order: match}, nil
		}
	}
	completed, err := r.history.ListCompletedOrders(ctx, req.market)
	if err != nil {
		return orderObservation{}, err
	}
	var match *domain.Order
	for _, order := range completed {
		if req.kind == "amend" && (order.ID == req.originalOrderID || orderMatchesID(order.Raw, req.originalOrderID)) {
			continue
		}
		if order.SubmittedAt != nil {
			if order.SubmittedAt.Before(earliest) {
				continue
			}
		} else if order.OrderDate != earliest.Format("2006-01-02") {
			continue
		}
		if !req.matchesOrder(order) {
			continue
		}
		if req.kind == "place" && order.Status != "체결완료" {
			continue
		}
		if req.kind == "cancel" && !orderStatusLooksCanceled(order.Status) {
			continue
		}
		if req.kind == "amend" && orderStatusLooksCanceled(order.Status) {
			continue
		}
		if equalFloat(order.Quantity, req.quantity) || equalFloat(order.FilledQuantity, req.quantity) {
			if match != nil {
				return orderObservation{ambiguous: true}, nil
			}
			match = &order
		}
	}
	return orderObservation{order: match, completed: true}, nil
}

func (req reconciliationRequest) matchesOrder(order domain.Order) bool {
	return matchesOrderSymbol(order, req.productCode, req.symbol) &&
		(req.side == "" || strings.EqualFold(order.Side, req.side)) &&
		(order.Market == "" || strings.EqualFold(order.Market, req.market)) &&
		equalFloat(order.Price, req.priceKRW)
}

func (req reconciliationRequest) result(order domain.Order, completed bool) trading.MutationResult {
	result := trading.MutationResult{
		Kind: req.kind, OrderID: order.ID, Symbol: req.symbol, Market: req.market,
		Quantity: order.Quantity, Price: order.Price, OrderDate: order.OrderDate,
	}
	if completed && req.kind != "cancel" {
		result.FilledQuantity = order.FilledQuantity
		result.AverageExecutionPrice = order.AverageExecutionPrice
	}
	switch req.kind {
	case "place":
		result.Status = "accepted_pending"
		if completed {
			result.Status = "filled_completed"
		}
	case "amend":
		result.OriginalOrderID, result.CurrentOrderID = req.originalOrderID, order.ID
		result.Status = "amended_pending"
		if completed {
			result.Status = "amended_completed"
		}
	case "cancel":
		result.Status = "canceled"
		if order.ID != req.originalOrderID {
			result.OriginalOrderID, result.CurrentOrderID = req.originalOrderID, order.ID
		}
	}
	return result
}

func (req reconciliationRequest) unknown() trading.MutationResult {
	result := trading.MutationResult{Kind: req.kind, Status: "unknown", Symbol: req.symbol,
		Market: req.market, Quantity: req.quantity, Price: req.priceKRW, OriginalOrderID: req.originalOrderID}
	switch req.kind {
	case "place":
		result.Warnings = []string{"Broker accepted the request but the final state was not visible in pending or completed history yet."}
	case "amend":
		result.Warnings = []string{"Broker accepted the amend request but the surviving order state is not yet visible."}
	case "cancel":
		result.OrderID = req.originalOrderID
		result.Warnings = []string{"Pending order disappeared, but cancellation is not confirmed in completed history. Inspect order history before retrying."}
	}
	return result
}

func matchesOrderSymbol(order domain.Order, productCode, symbol string) bool {
	return strings.EqualFold(order.Symbol, productCode) || strings.EqualFold(order.Symbol, symbol)
}

func orderStatusLooksCanceled(status string) bool {
	return strings.Contains(strings.TrimSpace(status), "취소")
}
