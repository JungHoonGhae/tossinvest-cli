package hybrid

// broker.go implements the order-mutation (write) side of the hybrid router.
//
// SAFETY-CRITICAL — the cardinal rule is NO DOUBLE ORDER EXECUTION. Unlike the
// read overrides in client.go (which freely fall back official -> WTS), order
// writes NEVER cross-retry on the other backend. A write is routed to exactly
// one backend up front; if that backend errors, we return the error with
// guidance and stop. Retrying the other backend could submit a second live
// order for an order that is already in flight at the brokerage.
//
// Routing:
//   - PlacePendingOrder: WTS when off==nil, Prefer=="wts", or the intent is a
//     WTS-unique capability (see officialEligiblePlace); otherwise official.
//   - Cancel/Amend: WTS when off==nil or Prefer=="wts"; otherwise official.
//   - GetOrderAvailableActions: ALWAYS WTS — the official API has no equivalent.

import (
	"context"
	"fmt"
	"io"

	"github.com/JungHoonGhae/tossinvest-cli/internal/orderintent"
	"github.com/JungHoonGhae/tossinvest-cli/internal/trading"
)

// officialOrderer is the subset of *official.Client the broker depends on. It
// exists so tests can substitute a fake without live HTTP; *official.Client
// satisfies it. Keeping it an interface (rather than the concrete type) is the
// only reason the double-order guard is unit-testable.
type officialOrderer interface {
	PlaceOrder(ctx context.Context, intent orderintent.PlaceIntent) (trading.MutationResult, error)
	CancelOrder(ctx context.Context, orderID string) (trading.MutationResult, error)
	ModifyOrder(ctx context.Context, intent orderintent.AmendIntent) (trading.MutationResult, error)
}

// hybridBroker routes order mutations between the official Open API and the WTS
// web session. It implements trading.Broker.
type hybridBroker struct {
	wts    trading.Broker
	off    officialOrderer
	pol    Policy
	stderr io.Writer
}

// Broker returns a trading.Broker that routes mutations per the client's policy.
// c.Client is the embedded WTS client, which already implements trading.Broker.
//
// The nil handling is deliberate: assigning a nil *official.Client straight into
// an interface field yields a NON-nil interface holding a nil pointer, which
// would defeat every `off == nil` check below. We convert explicitly so a
// missing official client really reads as nil.
func (c *Client) Broker() trading.Broker {
	var off officialOrderer
	if c.off != nil {
		off = c.off
	}
	return &hybridBroker{wts: c.Client, off: off, pol: c.pol, stderr: c.stderr}
}

// PlacePendingOrder routes a new order to exactly one backend. On official
// failure it does NOT fall back to WTS — the order may already be live at the
// brokerage, so a retry risks a duplicate. The wrapped error tells the user to
// verify acceptance via `tossctl orders` before retrying.
func (b *hybridBroker) PlacePendingOrder(ctx context.Context, intent orderintent.PlaceIntent) (trading.MutationResult, error) {
	if b.off == nil || b.pol.Prefer == "wts" || !officialEligiblePlace(intent) {
		return b.wts.PlacePendingOrder(ctx, intent)
	}
	res, err := b.off.PlaceOrder(ctx, intent)
	if err != nil {
		return trading.MutationResult{}, fmt.Errorf("공식 경로 주문 실패 — 중복 주문 방지를 위해 웹 세션으로 자동 재시도하지 않습니다. `tossctl orders` 로 접수 여부를 확인한 뒤 다시 시도하세요: %w", err)
	}
	return res, nil
}

// CancelPendingOrder routes by config only (no intent eligibility to judge).
// No cross-fallback on official failure.
func (b *hybridBroker) CancelPendingOrder(ctx context.Context, intent orderintent.CancelIntent) (trading.MutationResult, error) {
	if b.off == nil || b.pol.Prefer == "wts" {
		return b.wts.CancelPendingOrder(ctx, intent)
	}
	res, err := b.off.CancelOrder(ctx, intent.OrderID)
	if err != nil {
		return trading.MutationResult{}, fmt.Errorf("공식 경로 취소 실패 — 중복 처리 방지를 위해 웹 세션으로 자동 재시도하지 않습니다. `tossctl orders` 로 상태를 확인한 뒤 다시 시도하세요: %w", err)
	}
	return res, nil
}

// AmendPendingOrder routes by config only. No cross-fallback on official failure.
func (b *hybridBroker) AmendPendingOrder(ctx context.Context, intent orderintent.AmendIntent) (trading.MutationResult, error) {
	if b.off == nil || b.pol.Prefer == "wts" {
		return b.wts.AmendPendingOrder(ctx, intent)
	}
	res, err := b.off.ModifyOrder(ctx, intent)
	if err != nil {
		return trading.MutationResult{}, fmt.Errorf("공식 경로 정정 실패 — 중복 처리 방지를 위해 웹 세션으로 자동 재시도하지 않습니다. `tossctl orders` 로 상태를 확인한 뒤 다시 시도하세요: %w", err)
	}
	return res, nil
}

// GetOrderAvailableActions ALWAYS uses WTS: the official Open API exposes no
// equivalent pre-check, so there is nothing to route.
func (b *hybridBroker) GetOrderAvailableActions(ctx context.Context, orderID string) (map[string]any, error) {
	return b.wts.GetOrderAvailableActions(ctx, orderID)
}

// officialEligiblePlace reports whether the official Open API can serve this
// place intent. It returns false (→ WTS) only for orders the official path
// cannot do: KRW-settlement FRACTIONAL orders are a WTS-unique capability — the
// official fractional buy endpoint takes a USD orderAmount, not a KRW amount.
// Everything else — regular limit/market in KR or US, and USD-settled US
// fractional orders — is eligible. When genuinely unsure, the safe default is
// false (WTS is the proven path).
func officialEligiblePlace(intent orderintent.PlaceIntent) bool {
	if intent.Fractional && intent.CurrencyMode == "KRW" {
		return false
	}
	return true
}
