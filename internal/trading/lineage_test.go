package trading

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/config"
	"github.com/JungHoonGhae/tossinvest-cli/internal/orderintent"
	"github.com/JungHoonGhae/tossinvest-cli/internal/orderlineage"
)

// lineage 기록은 이제 Service 의 책임이다. 예전에는 cobra 쪽에만 있어서 MCP 로 넣은
// 주문은 흔적이 남지 않았다 — 나중에 취소할 때 원래 주문 id 로 찾을 수 없었다.
// 이 테스트가 "모든 변이 경로가 동일하게 기록한다" 를 고정한다.

type recorderSpy struct {
	calls []string
	err   error
}

func (r *recorderSpy) Record(originalOrderID string, e orderlineage.Entry) error {
	r.calls = append(r.calls, originalOrderID+"→"+e.CurrentOrderID)
	return r.err
}

type mutatingBroker struct{ res MutationResult }

func (b mutatingBroker) PlacePendingOrder(context.Context, orderintent.PlaceIntent) (MutationResult, error) {
	return b.res, nil
}
func (b mutatingBroker) CancelPendingOrder(context.Context, orderintent.CancelIntent) (MutationResult, error) {
	return b.res, nil
}
func (b mutatingBroker) AmendPendingOrder(context.Context, orderintent.AmendIntent) (MutationResult, error) {
	return b.res, nil
}
func (b mutatingBroker) GetOrderAvailableActions(context.Context, string) (map[string]any, error) {
	return map[string]any{}, nil
}

func openPolicy() config.Trading {
	return config.Trading{
		Place: true, Sell: true, Fractional: true, Cancel: true, Amend: true,
		AllowLiveOrderActions: true,
	}
}

func serviceWithSpy(res MutationResult) (*Service, *recorderSpy) {
	spy := &recorderSpy{}
	return NewService(openPolicy(), mutatingBroker{res: res}).WithLineage(spy), spy
}

// 세 변이 경로가 전부 기록해야 한다. 하나라도 빠지면 그 표면의 주문은 추적 불가다.
func TestEveryMutationPathRecordsLineage(t *testing.T) {
	res := MutationResult{
		Kind: "amend", OriginalOrderID: "OLD-1", CurrentOrderID: "NEW-1",
		Symbol: "DUMMY", Market: "us", Quantity: 1, Price: 10,
	}

	t.Run("Place", func(t *testing.T) {
		s, spy := serviceWithSpy(res)
		intent := orderintent.PlaceIntent{Symbol: "DUMMY", Side: "buy", Market: "us",
			OrderType: "limit", Quantity: 1, Price: 10, CurrencyMode: "KRW"}
		if _, err := s.Place(context.Background(), intent, confirmed(s, ActionPlace, s.PreviewPlace(intent))); err != nil {
			t.Fatal(err)
		}
		if len(spy.calls) != 1 {
			t.Fatalf("Place 가 lineage 를 기록하지 않았다: %v", spy.calls)
		}
	})

	t.Run("Cancel", func(t *testing.T) {
		s, spy := serviceWithSpy(res)
		intent := orderintent.CancelIntent{OrderID: "OLD-1", Symbol: "DUMMY"}
		if _, err := s.Cancel(context.Background(), intent, confirmed(s, ActionCancel, s.PreviewCancel(intent))); err != nil {
			t.Fatal(err)
		}
		if len(spy.calls) != 1 {
			t.Fatalf("Cancel 이 lineage 를 기록하지 않았다: %v", spy.calls)
		}
	})

	t.Run("Amend", func(t *testing.T) {
		s, spy := serviceWithSpy(res)
		qty, price := 1.0, 11.0
		intent := orderintent.AmendIntent{OrderID: "OLD-1", Quantity: &qty, Price: &price}
		if _, err := s.Amend(context.Background(), intent, confirmed(s, ActionAmend, s.PreviewAmend(intent))); err != nil {
			t.Fatal(err)
		}
		if len(spy.calls) != 1 || spy.calls[0] != "OLD-1→NEW-1" {
			t.Fatalf("Amend 기록이 틀렸다: %v", spy.calls)
		}
	})
}

// 기록 실패는 주문 실패가 아니다 — 주문은 이미 나갔다. 경고로만 알린다.
func TestLineageFailureWarnsButDoesNotFailTheOrder(t *testing.T) {
	s, spy := serviceWithSpy(MutationResult{
		Kind: "place", OriginalOrderID: "OLD-1", CurrentOrderID: "NEW-1",
	})
	spy.err = errors.New("디스크 오류")

	intent := orderintent.CancelIntent{OrderID: "OLD-1", Symbol: "DUMMY"}
	got, err := s.Cancel(context.Background(), intent, confirmed(s, ActionCancel, s.PreviewCancel(intent)))
	if err != nil {
		t.Fatalf("기록 실패가 주문을 실패시켰다: %v", err)
	}
	if len(got.Warnings) == 0 || !strings.Contains(got.Warnings[0], "lineage") {
		t.Errorf("경고가 없다 — 조용히 삼키면 안 된다: %v", got.Warnings)
	}
}

// recorder 가 없으면 예전과 똑같이 동작한다 (읽기 전용 호출자 배려).
func TestNoRecorderIsHarmless(t *testing.T) {
	s := NewService(openPolicy(), mutatingBroker{res: MutationResult{OriginalOrderID: "OLD-1"}})
	intent := orderintent.CancelIntent{OrderID: "OLD-1", Symbol: "DUMMY"}
	if _, err := s.Cancel(context.Background(), intent, confirmed(s, ActionCancel, s.PreviewCancel(intent))); err != nil {
		t.Fatalf("recorder 없이 실패했다: %v", err)
	}
}

// confirmed builds ExecuteOptions carrying the preview's own confirm token.
func confirmed(s *Service, _ Action, p Preview) ExecuteOptions {
	return ExecuteOptions{Execute: true, Confirm: p.ConfirmToken}
}

// resolvingSpy records like recorderSpy and also resolves ids through a map.
// It proves cancel/amend follow the live order the tool itself recorded.
type resolvingSpy struct {
	recorderSpy
	mapping    map[string]string
	resolveErr error
}

func (r *resolvingSpy) Resolve(orderID string) (string, bool, error) {
	if r.resolveErr != nil {
		return "", false, r.resolveErr
	}
	live, ok := r.mapping[orderID]
	return live, ok, nil
}

// TestCancelFollowsLineageToLiveOrder guards issue #6: after an amend issues a
// replacement id, cancelling the original id must act on the live order — the
// preview token, the guard, and the broker call all see the resolved id.
func TestCancelFollowsLineageToLiveOrder(t *testing.T) {
	spy := &resolvingSpy{mapping: map[string]string{"OLD-1": "NEW-1"}}
	stub := &brokerStub{}
	s := NewService(openPolicy(), stub).WithLineage(spy)

	intent := orderintent.CancelIntent{OrderID: "OLD-1", Symbol: "DUMMY"}
	preview := s.PreviewCancel(intent)
	if !strings.Contains(preview.Canonical, "NEW-1") {
		t.Fatalf("preview must bind the live order, got %q", preview.Canonical)
	}
	if _, err := s.Cancel(context.Background(), intent, confirmed(s, ActionCancel, preview)); err != nil {
		t.Fatal(err)
	}
	if stub.lastOrderID != "NEW-1" {
		t.Fatalf("broker must receive the live order id, got %q", stub.lastOrderID)
	}
}

// TestLineageResolveErrorDoesNotStrandCancel: a corrupt lineage cache must not
// block cancelling a possibly-live order — proceed with the original id and warn.
func TestLineageResolveErrorDoesNotStrandCancel(t *testing.T) {
	spy := &resolvingSpy{resolveErr: errors.New("디스크 오류")}
	stub := &brokerStub{}
	s := NewService(openPolicy(), stub).WithLineage(spy)

	intent := orderintent.CancelIntent{OrderID: "OLD-1", Symbol: "DUMMY"}
	got, err := s.Cancel(context.Background(), intent, confirmed(s, ActionCancel, s.PreviewCancel(intent)))
	if err != nil {
		t.Fatalf("resolve failure stranded the cancel: %v", err)
	}
	if stub.lastOrderID != "OLD-1" {
		t.Fatalf("must fall back to the original id, got %q", stub.lastOrderID)
	}
	if len(got.Warnings) == 0 || !strings.Contains(got.Warnings[len(got.Warnings)-1], "lineage") {
		t.Fatalf("resolve failure must warn, got %v", got.Warnings)
	}
}
