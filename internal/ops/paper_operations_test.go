package ops

import (
	"context"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/featuregate"
	"github.com/JungHoonGhae/tossinvest-cli/internal/hybrid"
	"github.com/JungHoonGhae/tossinvest-cli/internal/orderintent"
	"github.com/JungHoonGhae/tossinvest-cli/internal/papertrading"
)

type paperOpsClient struct{ placeCalls int }

func (f *paperOpsClient) GetPaperStatus(context.Context) (domain.PaperStatus, error) {
	return domain.PaperStatus{Environment: "paper", Product: "us-options"}, nil
}
func (f *paperOpsClient) InitPaperTrading(context.Context) (domain.PaperMutationReceipt, error) {
	return domain.PaperMutationReceipt{Environment: "paper"}, nil
}
func (f *paperOpsClient) DepositPaperCash(context.Context, int64) (domain.PaperMutationReceipt, error) {
	return domain.PaperMutationReceipt{Environment: "paper"}, nil
}
func (f *paperOpsClient) PreviewPaperOrder(_ context.Context, intent orderintent.OptionPlaceIntent) (domain.PaperOrderPreview, error) {
	intent.Exchange = "AMX"
	return domain.PaperOrderPreview{Environment: "paper", Product: "us-options", Intent: intent, PreparedQuantity: intent.Quantity}, nil
}
func (f *paperOpsClient) PlacePaperOrder(context.Context, orderintent.OptionPlaceIntent) (domain.PaperMutationReceipt, error) {
	f.placeCalls++
	return domain.PaperMutationReceipt{Environment: "paper", OrderID: "paper-1"}, nil
}
func (f *paperOpsClient) ListPaperPendingOrders(context.Context) ([]domain.PaperOrder, error) {
	return []domain.PaperOrder{{ID: "pending-1"}}, nil
}
func (f *paperOpsClient) ListPaperCompletedOrders(context.Context) ([]domain.PaperOrder, error) {
	return []domain.PaperOrder{{ID: "completed-1"}}, nil
}
func (f *paperOpsClient) CancelPaperOrder(context.Context, string) (domain.PaperMutationReceipt, error) {
	return domain.PaperMutationReceipt{Environment: "paper"}, nil
}
func (f *paperOpsClient) BulkCancelPaperOrders(context.Context, string) (domain.PaperBulkCancelReceipt, error) {
	return domain.PaperBulkCancelReceipt{Environment: "paper"}, nil
}

func TestPaperOperationsDeclareSimulationEnvironmentAndAuthorization(t *testing.T) {
	t.Parallel()
	catalog := NewCatalog(featuregate.PaperTrading)
	for _, id := range []string{"initialize_paper_trading", "deposit_paper_cash", "place_paper_order", "cancel_paper_order", "cancel_all_paper_orders"} {
		op, ok := catalog.Get(id)
		if !ok || !op.Write || op.Environment != "paper" || op.Backend != "wts" || op.Mutation == nil {
			t.Fatalf("%s = %#v, found=%v", id, op, ok)
		}
		if op.Mutation.RiskLevel != MutationRiskSimulation || op.Mutation.AuthorizationMode != MutationAuthorizationSimulation || op.Mutation.RequiresConfigOptIn || op.Mutation.RequiresFreshConfirmation {
			t.Fatalf("%s mutation = %#v", id, op.Mutation)
		}
	}
}

func TestPaperOrderOperationPreviewsThenExecutesWithoutLiveConfirmToken(t *testing.T) {
	t.Parallel()
	fake := &paperOpsClient{}
	deps := &Deps{WTS: &hybrid.Client{}, Paper: papertrading.NewService(fake), Auth: AuthStatus{WTS: BackendStatus{Connected: true}}}
	args := map[string]any{
		"stock_code": "OPT_SPY", "side": "buy", "order_type": "limit",
		"price": 0.01, "quantity": 1,
	}
	preview, err := NewCatalog(featuregate.PaperTrading).Call(context.Background(), deps, "place_paper_order", args)
	if err != nil {
		t.Fatal(err)
	}
	if plan := preview.(papertrading.Plan); plan.Applied || plan.Intent == nil || plan.Intent.Exchange != "AMX" || fake.placeCalls != 0 {
		t.Fatalf("preview=%#v calls=%d", preview, fake.placeCalls)
	}
	args["execute"] = true
	applied, err := NewCatalog(featuregate.PaperTrading).Call(context.Background(), deps, "place_paper_order", args)
	if err != nil {
		t.Fatal(err)
	}
	if plan := applied.(papertrading.Plan); !plan.Applied || fake.placeCalls != 1 {
		t.Fatalf("applied=%#v calls=%d", applied, fake.placeCalls)
	}
}

func TestPaperReadOperationsOwnRegressionProbes(t *testing.T) {
	t.Parallel()
	wanted := map[string]bool{"paper-cash-balance": false, "paper-education-summary": false, "paper-pending-orders": false, "paper-completed-orders": false}
	for _, probe := range NewCatalog(featuregate.PaperTrading).Probes() {
		if _, ok := wanted[probe.Name]; ok {
			wanted[probe.Name] = true
		}
	}
	for name, found := range wanted {
		if !found {
			t.Errorf("missing probe %s", name)
		}
	}
}

func TestPaperOperationHandlersCoverReadAndWritePreviews(t *testing.T) {
	t.Parallel()
	fake := &paperOpsClient{}
	deps := &Deps{WTS: &hybrid.Client{}, Paper: papertrading.NewService(fake), Auth: AuthStatus{WTS: BackendStatus{Connected: true}}}
	catalog := NewCatalog(featuregate.PaperTrading)
	cases := []struct {
		id   string
		args map[string]any
	}{
		{id: "get_paper_trading_status", args: map[string]any{}},
		{id: "list_pending_paper_orders", args: map[string]any{}},
		{id: "list_completed_paper_orders", args: map[string]any{}},
		{id: "initialize_paper_trading", args: map[string]any{}},
		{id: "deposit_paper_cash", args: map[string]any{"amount": 1000}},
		{id: "cancel_paper_order", args: map[string]any{"order_id": "pending-1"}},
		{id: "cancel_all_paper_orders", args: map[string]any{"side": "buy"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.id, func(t *testing.T) {
			t.Parallel()
			if _, err := catalog.Call(context.Background(), deps, tc.id, tc.args); err != nil {
				t.Fatalf("Call(%s): %v", tc.id, err)
			}
		})
	}
}
