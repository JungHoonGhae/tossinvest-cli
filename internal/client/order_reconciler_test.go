package client

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/trading"
)

type historyFrame struct {
	pending, completed       []domain.Order
	pendingErr, completedErr error
}

type scriptedOrderHistory struct {
	frames                       []historyFrame
	pendingCalls, completedCalls int
	markets                      []string
}

func (h *scriptedOrderHistory) frame() historyFrame {
	i := h.pendingCalls - 1
	if i >= len(h.frames) {
		i = len(h.frames) - 1
	}
	if i < 0 {
		return historyFrame{}
	}
	return h.frames[i]
}

func (h *scriptedOrderHistory) ListPendingOrders(context.Context) ([]domain.Order, error) {
	h.pendingCalls++
	f := h.frame()
	return f.pending, f.pendingErr
}

func (h *scriptedOrderHistory) ListCompletedOrders(_ context.Context, market string) ([]domain.Order, error) {
	h.completedCalls++
	h.markets = append(h.markets, market)
	f := h.frame()
	return f.completed, f.completedErr
}

func TestOrderReconcilerClassifiesHistory(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC)
	order := domain.Order{ID: "new", Symbol: "A005930", Market: "kr", Quantity: 2, Price: 500,
		OrderDate: "2026-09-07", SubmittedAt: &now}
	filled := order
	filled.Status, filled.FilledQuantity, filled.AverageExecutionPrice = "체결완료", 2, 499
	canceled := order
	canceled.Status = "취소"
	original := order
	original.ID = "original"
	old := filled
	oldTime := now.Add(-3 * time.Minute)
	old.SubmittedAt = &oldTime
	wrongSymbol, wrongPrice, wrongQuantity := order, order, order
	wrongSymbol.Symbol, wrongPrice.Price, wrongQuantity.Quantity = "other", 501, 3

	for _, tc := range []struct {
		name, kind, status string
		frames             []historyFrame
		wantID             string
		pending, completed int
		wantErr            error
	}{
		{name: "place pending", kind: "place", status: "accepted_pending", frames: []historyFrame{{pending: []domain.Order{order}}}, wantID: "new", pending: 1},
		{name: "place completed", kind: "place", status: "filled_completed", frames: []historyFrame{{completed: []domain.Order{filled}}}, wantID: "new", pending: 1, completed: 1},
		{name: "place canceled is not filled", kind: "place", status: "unknown", frames: []historyFrame{{completed: []domain.Order{canceled}}}, pending: 3, completed: 3},
		{name: "old completed row", kind: "place", status: "unknown", frames: []historyFrame{{completed: []domain.Order{old}}}, pending: 3, completed: 3},
		{name: "unrelated pending rows", kind: "place", status: "unknown", frames: []historyFrame{{pending: []domain.Order{wrongSymbol, wrongPrice, wrongQuantity}}}, pending: 3, completed: 3},
		{name: "amend new pending", kind: "amend", status: "amended_pending", frames: []historyFrame{{pending: []domain.Order{original, order}}}, wantID: "new", pending: 1},
		{name: "amend original excluded", kind: "amend", status: "unknown", frames: []historyFrame{{pending: []domain.Order{original}}}, pending: 3, completed: 3},
		{name: "amend completed", kind: "amend", status: "amended_completed", frames: []historyFrame{{completed: []domain.Order{filled}}}, wantID: "new", pending: 1, completed: 1},
		{name: "amend unknown preserves KR market", kind: "amend", status: "unknown", pending: 3, completed: 3},
		{name: "cancel confirmed rollover", kind: "cancel", status: "canceled", frames: []historyFrame{{completed: []domain.Order{canceled}}}, wantID: "new", pending: 1, completed: 1},
		{name: "cancel still pending", kind: "cancel", frames: []historyFrame{{pending: []domain.Order{original}}}, pending: 3, wantErr: trading.ErrCancelStillPending},
		{name: "disappearance is not cancellation", kind: "cancel", status: "unknown", wantID: "original", pending: 3, completed: 3},
		{name: "fill during cancellation", kind: "cancel", status: "unknown", frames: []historyFrame{{completed: []domain.Order{filled}}}, wantID: "original", pending: 3, completed: 3},
		{name: "cancel eventually visible", kind: "cancel", status: "canceled", frames: []historyFrame{{pending: []domain.Order{original}}, {}, {completed: []domain.Order{canceled}}}, wantID: "new", pending: 3, completed: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := &scriptedOrderHistory{frames: tc.frames}
			r := newOrderReconciler(h)
			r.attempts, r.interval = 3, 0
			req := reconciliationRequest{kind: tc.kind, productCode: "A005930", symbol: "005930", market: "kr", priceKRW: 500, quantity: 2, startedAt: now}
			if tc.kind != "place" {
				req.originalOrderID = "original"
			}
			got, err := r.reconcile(context.Background(), req)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			if h.pendingCalls != tc.pending || h.completedCalls != tc.completed {
				t.Errorf("history reads = %d/%d, want %d/%d", h.pendingCalls, h.completedCalls, tc.pending, tc.completed)
			}
			for _, market := range h.markets {
				if market != "kr" {
					t.Errorf("history market = %q, want kr", market)
				}
			}
			if err != nil {
				return
			}
			if got.Kind != tc.kind || got.Status != tc.status || got.OrderID != tc.wantID || got.Market != "kr" || got.Symbol != req.symbol || got.Quantity != 2 || got.Price != 500 {
				t.Fatalf("result = %+v", got)
			}
			if got.Status == "unknown" && len(got.Warnings) == 0 {
				t.Error("unknown result must carry an inspection warning")
			}
			if tc.kind == "amend" || (tc.kind == "cancel" && tc.wantID == "new") {
				if got.OriginalOrderID != "original" {
					t.Errorf("original ID = %q", got.OriginalOrderID)
				}
				if tc.status != "unknown" && got.CurrentOrderID != tc.wantID {
					t.Errorf("current ID = %q", got.CurrentOrderID)
				}
			}
			if tc.status == "filled_completed" || tc.status == "amended_completed" {
				if got.FilledQuantity != 2 || got.AverageExecutionPrice != 499 {
					t.Errorf("execution details lost: %+v", got)
				}
			}
		})
	}
}

func TestOrderReconcilerHistoryErrorsStopObservation(t *testing.T) {
	t.Parallel()
	want := errors.New("history unavailable")
	for _, completed := range []bool{false, true} {
		f := historyFrame{pendingErr: want}
		if completed {
			f = historyFrame{completedErr: want}
		}
		h := &scriptedOrderHistory{frames: []historyFrame{f}}
		r := newOrderReconciler(h)
		_, err := r.reconcile(context.Background(), reconciliationRequest{kind: "place", startedAt: time.Now()})
		if !errors.Is(err, want) || h.pendingCalls != 1 {
			t.Fatalf("error = %v; pending reads = %d", err, h.pendingCalls)
		}
	}
}

func TestOrderReconcilerDoesNotGuessAmongMultipleOrders(t *testing.T) {
	t.Parallel()
	now := time.Now()
	for _, kind := range []string{"place", "amend", "cancel"} {
		t.Run(kind, func(t *testing.T) {
			status := "체결완료"
			if kind == "cancel" {
				status = "취소"
			}
			first := domain.Order{ID: "one", Symbol: "A005930", Price: 500, Quantity: 2, Status: status, SubmittedAt: &now}
			second := first
			second.ID = "two"
			for _, pending := range []bool{false, true} {
				if kind == "cancel" && pending {
					continue
				}
				frame := historyFrame{completed: []domain.Order{first, second}}
				if pending {
					frame = historyFrame{pending: []domain.Order{first, second}, completed: []domain.Order{first}}
				}
				h := &scriptedOrderHistory{frames: []historyFrame{frame}}
				r := newOrderReconciler(h)
				r.attempts, r.interval = 2, 0
				got, err := r.reconcile(context.Background(), reconciliationRequest{kind: kind, originalOrderID: "original", productCode: "A005930", market: "kr", priceKRW: 500, quantity: 2, startedAt: now})
				if err != nil || got.Status != "unknown" || got.CurrentOrderID != "" || got.OrderID == "one" || got.OrderID == "two" {
					t.Fatalf("pending=%t: ambiguous result = %+v, %v", pending, got, err)
				}
			}
		})
	}
}

func TestOrderReconcilerBoundedTiming(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		now := time.Now()
		order := domain.Order{ID: "new", Symbol: "A005930", Quantity: 2, Price: 500}
		h := &scriptedOrderHistory{frames: []historyFrame{{}, {}, {pending: []domain.Order{order}}}}
		r := newOrderReconciler(h)
		got, err := r.reconcile(context.Background(), reconciliationRequest{kind: "place",
			productCode: "A005930", market: "kr", priceKRW: 500, quantity: 2, startedAt: now})
		if err != nil || got.Status != "accepted_pending" || h.pendingCalls != 3 || time.Since(now) != 2*mutationReconcileInterval {
			t.Fatalf("result = %+v, error = %v; reads = %d, elapsed = %s", got, err, h.pendingCalls, time.Since(now))
		}
	})
}

func TestOrderReconcilerRejectsUnrelatedHistory(t *testing.T) {
	t.Parallel()
	now := time.Now()
	for _, tc := range []struct {
		name, kind, side, market, id, status string
		pending                              bool
	}{
		{name: "opposite pending side", kind: "place", side: "sell", market: "kr", id: "new", pending: true},
		{name: "missing pending side", kind: "place", market: "kr", id: "new", pending: true},
		{name: "different pending market", kind: "place", side: "buy", market: "us", id: "new", pending: true},
		{name: "opposite completed side", kind: "cancel", side: "sell", market: "kr", id: "new", status: "취소"},
		{name: "original completed order", kind: "amend", side: "buy", market: "kr", id: "original", status: "체결완료"},
		{name: "canceled amendment row", kind: "amend", side: "buy", market: "kr", id: "new", status: "취소"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			order := domain.Order{ID: tc.id, Symbol: "A005930", Market: tc.market, Side: tc.side, Status: tc.status, Price: 500, Quantity: 2, SubmittedAt: &now}
			frame := historyFrame{completed: []domain.Order{order}}
			if tc.pending {
				frame = historyFrame{pending: []domain.Order{order}}
			}
			r := newOrderReconciler(&scriptedOrderHistory{frames: []historyFrame{frame}})
			r.attempts, r.interval = 2, 0
			got, err := r.reconcile(context.Background(), reconciliationRequest{kind: tc.kind, originalOrderID: "original", productCode: "A005930", side: "buy", market: "kr", priceKRW: 500, quantity: 2, startedAt: now})
			if err != nil || got.Status != "unknown" || got.CurrentOrderID != "" {
				t.Fatalf("unrelated history matched: %+v, %v", got, err)
			}
		})
	}
}

func TestOrderReconcilerCancellationStopsHistoryReads(t *testing.T) {
	t.Parallel()
	for _, preCancelled := range []bool{false, true} {
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if preCancelled {
				cancel()
			}
			h := &scriptedOrderHistory{}
			r := newOrderReconciler(h)
			var err error
			go func() { _, err = r.reconcile(ctx, reconciliationRequest{kind: "cancel", startedAt: time.Now()}) }()
			synctest.Wait()
			cancel()
			synctest.Wait()
			wantReads := 1
			if preCancelled {
				wantReads = 0
			}
			if !errors.Is(err, context.Canceled) || h.pendingCalls != wantReads {
				t.Fatalf("error = %v; reads = %d, want %d", err, h.pendingCalls, wantReads)
			}
		})
	}
}
