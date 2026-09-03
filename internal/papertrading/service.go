// Package papertrading owns the simulation-only execution boundary. It never
// accepts a live broker and all writes delegate to dedicated /paper/ routes.
package papertrading

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/orderintent"
)

type Action string

const (
	ActionInitialize Action = "initialize"
	ActionDeposit    Action = "deposit"
	ActionPlace      Action = "place"
	ActionCancel     Action = "cancel"
	ActionBulkCancel Action = "bulk_cancel"
)

type Client interface {
	GetPaperStatus(context.Context) (domain.PaperStatus, error)
	InitPaperTrading(context.Context) (domain.PaperMutationReceipt, error)
	DepositPaperCash(context.Context, int64) (domain.PaperMutationReceipt, error)
	PreviewPaperOrder(context.Context, orderintent.OptionPlaceIntent) (domain.PaperOrderPreview, error)
	PlacePaperOrder(context.Context, orderintent.OptionPlaceIntent) (domain.PaperMutationReceipt, error)
	ListPaperPendingOrders(context.Context) ([]domain.PaperOrder, error)
	ListPaperCompletedOrders(context.Context) ([]domain.PaperOrder, error)
	CancelPaperOrder(context.Context, string) (domain.PaperMutationReceipt, error)
	BulkCancelPaperOrders(context.Context, string) (domain.PaperBulkCancelReceipt, error)
}

type ExecuteOptions struct {
	Execute bool `json:"execute"`
}

// Plan is the common preview/execution receipt. simulation_execute means an
// automation may execute after explicitly setting execute=true; no live-order
// config or human confirmation token is involved.
type Plan struct {
	Kind          string `json:"kind"`
	Environment   string `json:"environment"`
	Product       string `json:"product"`
	Action        Action `json:"action"`
	Authorization string `json:"authorization"`
	// PortableCanonical is the target-independent economic intent. It is useful
	// for comparing paper and live previews, but never authorizes a live order.
	PortableCanonical string                         `json:"portable_canonical,omitempty"`
	Amount            int64                          `json:"amount,omitempty"`
	Side              string                         `json:"side,omitempty"`
	Intent            *orderintent.OptionPlaceIntent `json:"intent,omitempty"`
	Target            *domain.PaperOrder             `json:"target,omitempty"`
	TargetCount       int                            `json:"target_count,omitempty"`
	RemainingCount    *int                           `json:"remaining_count,omitempty"`
	Applied           bool                           `json:"applied"`
	Receipt           *domain.PaperMutationReceipt   `json:"receipt,omitempty"`
	BulkReceipt       *domain.PaperBulkCancelReceipt `json:"bulk_receipt,omitempty"`
	PostStatus        *domain.PaperStatus            `json:"post_status,omitempty"`
}

type Service struct{ client Client }

func NewService(client Client) *Service { return &Service{client: client} }

func (s *Service) Status(ctx context.Context) (domain.PaperStatus, error) {
	if err := s.ready(); err != nil {
		return domain.PaperStatus{}, err
	}
	return s.client.GetPaperStatus(ctx)
}

func (s *Service) PendingOrders(ctx context.Context) ([]domain.PaperOrder, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	return s.client.ListPaperPendingOrders(ctx)
}

func (s *Service) CompletedOrders(ctx context.Context) ([]domain.PaperOrder, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	return s.client.ListPaperCompletedOrders(ctx)
}

func (s *Service) Initialize(ctx context.Context, opts ExecuteOptions) (Plan, error) {
	if err := s.ready(); err != nil {
		return Plan{}, err
	}
	plan := newPlan(ActionInitialize)
	if !opts.Execute {
		return plan, nil
	}
	receipt, err := s.client.InitPaperTrading(ctx)
	if err != nil {
		return Plan{}, err
	}
	plan.Applied, plan.Receipt = true, &receipt
	return plan, nil
}

func (s *Service) Deposit(ctx context.Context, amount int64, opts ExecuteOptions) (Plan, error) {
	if err := s.ready(); err != nil {
		return Plan{}, err
	}
	if amount <= 0 {
		return Plan{}, fmt.Errorf("paper deposit amount must be greater than zero")
	}
	plan := newPlan(ActionDeposit)
	plan.Amount = amount
	if !opts.Execute {
		return plan, nil
	}
	before, err := s.client.GetPaperStatus(ctx)
	if err != nil {
		return Plan{}, fmt.Errorf("paper cash deposit pre-write status verification failed: %w", err)
	}
	receipt, err := s.client.DepositPaperCash(ctx, amount)
	if err != nil {
		return Plan{}, err
	}
	verifyCtx, cancel := paperVerificationContext(ctx)
	defer cancel()
	status, err := s.client.GetPaperStatus(verifyCtx)
	if err != nil {
		return Plan{}, fmt.Errorf("paper cash deposit was accepted but post-write status verification failed: %w; inspect paper status before retrying", err)
	}
	if increase := status.Balance.Deposit - before.Balance.Deposit; increase < float64(amount) {
		return Plan{}, fmt.Errorf("paper cash deposit was accepted but deposit balance increased by %.0f instead of at least %d; inspect paper status before retrying", increase, amount)
	}
	plan.Applied, plan.Receipt, plan.PostStatus = true, &receipt, &status
	return plan, nil
}

func (s *Service) Place(ctx context.Context, intent orderintent.OptionPlaceIntent, opts ExecuteOptions) (Plan, error) {
	if err := s.ready(); err != nil {
		return Plan{}, err
	}
	normalized, err := orderintent.NormalizeOptionPlace(orderintent.OptionPlaceInput{
		Symbol: intent.Symbol, Exchange: intent.Exchange, CurrencyMode: intent.CurrencyMode,
		Side: intent.Side, OrderType: intent.OrderType, Price: intent.Price, Quantity: intent.Quantity,
	})
	if err != nil {
		return Plan{}, err
	}
	intent = normalized
	plan := newPlan(ActionPlace)
	plan.Intent = &intent
	plan.PortableCanonical = intent.PortableCanonical()
	if !opts.Execute {
		preview, err := s.client.PreviewPaperOrder(ctx, intent)
		if err != nil {
			return Plan{}, err
		}
		plan.Intent = &preview.Intent
		plan.PortableCanonical = preview.Intent.PortableCanonical()
		return plan, nil
	}
	receipt, err := s.client.PlacePaperOrder(ctx, intent)
	if err != nil {
		return Plan{}, err
	}
	plan.Applied, plan.Receipt = true, &receipt
	return plan, nil
}

func (s *Service) Cancel(ctx context.Context, orderID string, opts ExecuteOptions) (Plan, error) {
	if err := s.ready(); err != nil {
		return Plan{}, err
	}
	orders, err := s.client.ListPaperPendingOrders(ctx)
	if err != nil {
		return Plan{}, err
	}
	wanted := strings.TrimSpace(orderID)
	plan := newPlan(ActionCancel)
	for i := range orders {
		order := &orders[i]
		if paperOrderMatches(*order, wanted) {
			plan.Target = order
			break
		}
	}
	if plan.Target == nil {
		return Plan{}, fmt.Errorf("pending paper order %s was not found", orderID)
	}
	if !opts.Execute {
		return plan, nil
	}
	receipt, err := s.client.CancelPaperOrder(ctx, orderID)
	if err != nil {
		return Plan{}, err
	}
	verifyCtx, cancel := paperVerificationContext(ctx)
	defer cancel()
	remaining, err := s.client.ListPaperPendingOrders(verifyCtx)
	if err != nil {
		return Plan{}, fmt.Errorf("paper cancellation was accepted but post-write verification failed: %w; inspect pending paper orders before retrying", err)
	}
	for _, order := range remaining {
		if paperOrderMatches(order, wanted) {
			return Plan{}, fmt.Errorf("paper cancellation was accepted but order %s remains pending; inspect pending and completed paper orders before retrying", orderID)
		}
	}
	plan.Applied, plan.Receipt = true, &receipt
	return plan, nil
}

func (s *Service) BulkCancel(ctx context.Context, side string, opts ExecuteOptions) (Plan, error) {
	if err := s.ready(); err != nil {
		return Plan{}, err
	}
	side = strings.ToLower(strings.TrimSpace(side))
	if side != "" && side != "buy" && side != "sell" {
		return Plan{}, fmt.Errorf("paper bulk-cancel side must be buy, sell, or empty")
	}
	orders, err := s.client.ListPaperPendingOrders(ctx)
	if err != nil {
		return Plan{}, err
	}
	plan := newPlan(ActionBulkCancel)
	plan.Side = side
	for _, order := range orders {
		if side == "" || strings.EqualFold(side, order.TradeType) {
			plan.TargetCount++
		}
	}
	if !opts.Execute || plan.TargetCount == 0 {
		return plan, nil
	}
	receipt, err := s.client.BulkCancelPaperOrders(ctx, side)
	if err != nil {
		return Plan{}, err
	}
	verifyCtx, cancel := paperVerificationContext(ctx)
	defer cancel()
	remaining, err := s.client.ListPaperPendingOrders(verifyCtx)
	if err != nil {
		return Plan{}, fmt.Errorf("paper bulk cancellation was accepted but post-write verification failed: %w; inspect pending paper orders before retrying", err)
	}
	remainingCount := 0
	for _, order := range remaining {
		if side == "" || strings.EqualFold(side, order.TradeType) {
			remainingCount++
		}
	}
	plan.RemainingCount = &remainingCount
	if remainingCount > receipt.FailedCount {
		return Plan{}, fmt.Errorf("paper bulk cancellation reported %d failures but %d matching orders remain pending; inspect pending orders before retrying", receipt.FailedCount, remainingCount)
	}
	plan.Applied, plan.BulkReceipt = true, &receipt
	return plan, nil
}

func paperOrderMatches(order domain.PaperOrder, wanted string) bool {
	wanted = strings.TrimSpace(wanted)
	return wanted != "" && (order.ID == wanted || order.OrderID == wanted || order.OrderNo == wanted || order.OrderDate+"/"+order.OrderNo == wanted)
}

func paperVerificationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
}

func newPlan(action Action) Plan {
	return Plan{
		Kind: "paper_trading_change", Environment: "paper", Product: "us-options",
		Action: action, Authorization: "simulation_execute",
	}
}

// LivePreviewIntent preserves the economic option order while deliberately
// dropping paper-only exchange metadata. The caller must pass the result to
// trading.Service, which re-applies all live config and confirmation gates.
func LivePreviewIntent(intent orderintent.OptionPlaceIntent) (orderintent.PlaceIntent, error) {
	return intent.LiveIntent()
}

func (s *Service) ready() error {
	if s == nil || s.client == nil {
		return fmt.Errorf("paper trading is not configured")
	}
	return nil
}
