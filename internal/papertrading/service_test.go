package papertrading

import (
	"context"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/orderintent"
)

type fakeClient struct {
	initCalls, depositCalls, previewCalls, placeCalls, cancelCalls, bulkCalls, statusCalls int
	pendingCalls, completedCalls                                                           int
	orders                                                                                 []domain.PaperOrder
	status                                                                                 domain.PaperStatus
	stickyCancel                                                                           bool
	stickyDeposit                                                                          bool
}

func (f *fakeClient) PreviewPaperOrder(_ context.Context, intent orderintent.OptionPlaceIntent) (domain.PaperOrderPreview, error) {
	f.previewCalls++
	intent.Exchange = "AMX"
	return domain.PaperOrderPreview{Environment: "paper", Product: "us-options", Intent: intent, PreparedQuantity: intent.Quantity}, nil
}

func (f *fakeClient) GetPaperStatus(context.Context) (domain.PaperStatus, error) {
	f.statusCalls++
	if f.status.Environment == "" {
		f.status = domain.PaperStatus{Environment: "paper", Product: "us-options"}
	}
	return f.status, nil
}
func (f *fakeClient) InitPaperTrading(context.Context) (domain.PaperMutationReceipt, error) {
	f.initCalls++
	return domain.PaperMutationReceipt{Environment: "paper", Message: "ok"}, nil
}
func (f *fakeClient) DepositPaperCash(_ context.Context, amount int64) (domain.PaperMutationReceipt, error) {
	f.depositCalls++
	if !f.stickyDeposit {
		f.status.Balance.Deposit += float64(amount)
	}
	return domain.PaperMutationReceipt{Environment: "paper", Message: "ok"}, nil
}
func (f *fakeClient) PlacePaperOrder(context.Context, orderintent.OptionPlaceIntent) (domain.PaperMutationReceipt, error) {
	f.placeCalls++
	return domain.PaperMutationReceipt{Environment: "paper", OrderID: "paper-1"}, nil
}
func (f *fakeClient) ListPaperPendingOrders(context.Context) ([]domain.PaperOrder, error) {
	f.pendingCalls++
	return f.orders, nil
}
func (f *fakeClient) ListPaperCompletedOrders(context.Context) ([]domain.PaperOrder, error) {
	f.completedCalls++
	return []domain.PaperOrder{{ID: "completed-1", Status: "filled"}}, nil
}
func (f *fakeClient) CancelPaperOrder(_ context.Context, orderID string) (domain.PaperMutationReceipt, error) {
	f.cancelCalls++
	if !f.stickyCancel {
		remaining := f.orders[:0]
		for _, order := range f.orders {
			if order.ID != orderID && order.OrderID != orderID && order.OrderNo != orderID && order.OrderDate+"/"+order.OrderNo != orderID {
				remaining = append(remaining, order)
			}
		}
		f.orders = remaining
	}
	return domain.PaperMutationReceipt{Environment: "paper", OrderID: "paper-1"}, nil
}
func (f *fakeClient) BulkCancelPaperOrders(_ context.Context, side string) (domain.PaperBulkCancelReceipt, error) {
	f.bulkCalls++
	remaining := f.orders[:0]
	for _, order := range f.orders {
		if side != "" && !strings.EqualFold(side, order.TradeType) {
			remaining = append(remaining, order)
		}
	}
	f.orders = remaining
	return domain.PaperBulkCancelReceipt{Environment: "paper", RequestedCount: 2}, nil
}

func TestPaperWritesPreviewByDefaultAndExecuteWithoutLiveConfirmation(t *testing.T) {
	t.Parallel()
	f := &fakeClient{status: domain.PaperStatus{
		Environment: "paper", Product: "us-options",
	}}
	s := NewService(f)

	preview, err := s.Deposit(context.Background(), 1000, ExecuteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Applied || preview.Environment != "paper" || preview.Authorization != "simulation_execute" || f.depositCalls != 0 {
		t.Fatalf("preview=%#v calls=%d", preview, f.depositCalls)
	}

	applied, err := s.Deposit(context.Background(), 1000, ExecuteOptions{Execute: true})
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Applied || applied.PostStatus == nil || applied.PostStatus.Balance.Deposit != 1000 || f.depositCalls != 1 || f.statusCalls != 2 {
		t.Fatalf("applied=%#v calls=%d", applied, f.depositCalls)
	}
}

func TestPaperDepositFailsClosedWhenBalanceDoesNotIncrease(t *testing.T) {
	t.Parallel()
	f := &fakeClient{
		stickyDeposit: true,
		status: domain.PaperStatus{
			Environment: "paper", Product: "us-options",
			Balance: domain.PaperCashBalance{Deposit: 5000},
		},
	}
	if _, err := NewService(f).Deposit(context.Background(), 1000, ExecuteOptions{Execute: true}); err == nil {
		t.Fatal("expected unchanged paper deposit balance to fail closed")
	}
	if f.depositCalls != 1 || f.statusCalls != 2 {
		t.Fatalf("deposit calls=%d status calls=%d", f.depositCalls, f.statusCalls)
	}
}

func TestPaperPlacePlanIsUSOptionsSimulationOnly(t *testing.T) {
	t.Parallel()
	f := &fakeClient{}
	s := NewService(f)
	intent := orderintent.OptionPlaceIntent{Symbol: "OPT_SPY", CurrencyMode: "USD", Side: "buy", OrderType: "limit", Price: .1, Quantity: 1}

	preview, err := s.Place(context.Background(), intent, ExecuteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Applied || preview.Intent == nil || preview.Intent.Exchange != "AMX" || preview.PortableCanonical == "" || f.previewCalls != 1 || f.placeCalls != 0 {
		t.Fatalf("preview=%#v previewCalls=%d placeCalls=%d", preview, f.previewCalls, f.placeCalls)
	}

	plan, err := s.Place(context.Background(), intent, ExecuteOptions{Execute: true})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Applied || plan.Action != ActionPlace || plan.Product != "us-options" || f.placeCalls != 1 {
		t.Fatalf("plan=%#v calls=%d", plan, f.placeCalls)
	}
}

func TestPaperCancelPreviewResolvesExactPendingOrder(t *testing.T) {
	t.Parallel()
	f := &fakeClient{orders: []domain.PaperOrder{{ID: "paper-1", OrderNo: "12", StockCode: "OPT1", TradeType: "buy", PendingQuantity: 1}}}
	s := NewService(f)

	plan, err := s.Cancel(context.Background(), "paper-1", ExecuteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Target == nil || plan.Target.ID != "paper-1" || f.cancelCalls != 0 {
		t.Fatalf("plan=%#v calls=%d", plan, f.cancelCalls)
	}
}

func TestPaperCancelExecutionVerifiesOrderDisappeared(t *testing.T) {
	t.Parallel()
	f := &fakeClient{orders: []domain.PaperOrder{{ID: "paper-1", OrderNo: "12", StockCode: "OPT1", TradeType: "buy", PendingQuantity: 1}}}
	plan, err := NewService(f).Cancel(context.Background(), "paper-1", ExecuteOptions{Execute: true})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Applied || f.cancelCalls != 1 || len(f.orders) != 0 {
		t.Fatalf("plan=%#v calls=%d remaining=%#v", plan, f.cancelCalls, f.orders)
	}
}

func TestPaperCancelExecutionFailsClosedWhenOrderRemainsPending(t *testing.T) {
	t.Parallel()
	f := &fakeClient{stickyCancel: true, orders: []domain.PaperOrder{{ID: "paper-1", OrderNo: "12", StockCode: "OPT1", TradeType: "buy", PendingQuantity: 1}}}
	if _, err := NewService(f).Cancel(context.Background(), "paper-1", ExecuteOptions{Execute: true}); err == nil {
		t.Fatal("expected post-write verification failure")
	}
}

func TestPaperBulkCancelExecutionReturnsVerifiedRemainingCount(t *testing.T) {
	t.Parallel()
	f := &fakeClient{orders: []domain.PaperOrder{
		{ID: "buy-1", TradeType: "buy"},
		{ID: "sell-1", TradeType: "sell"},
	}}
	plan, err := NewService(f).BulkCancel(context.Background(), "buy", ExecuteOptions{Execute: true})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Applied || plan.RemainingCount == nil || *plan.RemainingCount != 0 || len(f.orders) != 1 {
		t.Fatalf("plan=%#v remaining=%#v", plan, f.orders)
	}
}

func TestPaperCompletedOrdersRemainInSimulationBoundary(t *testing.T) {
	t.Parallel()
	orders, err := NewService(&fakeClient{}).CompletedOrders(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(orders) != 1 || orders[0].ID != "completed-1" {
		t.Fatalf("orders = %#v", orders)
	}
}

func TestPaperReadAndInitializeFlows(t *testing.T) {
	t.Parallel()
	f := &fakeClient{
		status: domain.PaperStatus{Environment: "paper", Product: "us-options"},
		orders: []domain.PaperOrder{{ID: "pending-1"}},
	}
	s := NewService(f)

	status, err := s.Status(context.Background())
	if err != nil || status.Environment != "paper" || f.statusCalls != 1 {
		t.Fatalf("status=%#v err=%v calls=%d", status, err, f.statusCalls)
	}
	pending, err := s.PendingOrders(context.Background())
	if err != nil || len(pending) != 1 || f.pendingCalls != 1 {
		t.Fatalf("pending=%#v err=%v calls=%d", pending, err, f.pendingCalls)
	}

	preview, err := s.Initialize(context.Background(), ExecuteOptions{})
	if err != nil || preview.Applied || preview.Action != ActionInitialize || f.initCalls != 0 {
		t.Fatalf("preview=%#v err=%v calls=%d", preview, err, f.initCalls)
	}
	applied, err := s.Initialize(context.Background(), ExecuteOptions{Execute: true})
	if err != nil || !applied.Applied || applied.Receipt == nil || f.initCalls != 1 {
		t.Fatalf("applied=%#v err=%v calls=%d", applied, err, f.initCalls)
	}
}

func TestPaperServiceRejectsInvalidOrMissingTargetsBeforeMutation(t *testing.T) {
	t.Parallel()
	s := NewService(&fakeClient{})
	if _, err := s.Deposit(context.Background(), 0, ExecuteOptions{Execute: true}); err == nil {
		t.Fatal("expected zero deposit to fail")
	}
	if _, err := s.Place(context.Background(), orderintent.OptionPlaceIntent{}, ExecuteOptions{Execute: true}); err == nil {
		t.Fatal("expected incomplete option intent to fail")
	}
	if _, err := s.Cancel(context.Background(), "missing", ExecuteOptions{Execute: true}); err == nil {
		t.Fatal("expected missing pending order to fail")
	}
	if _, err := s.BulkCancel(context.Background(), "hold", ExecuteOptions{Execute: true}); err == nil {
		t.Fatal("expected invalid bulk-cancel side to fail")
	}
	if _, err := NewService(nil).Status(context.Background()); err == nil {
		t.Fatal("expected nil paper client to fail closed")
	}
}
