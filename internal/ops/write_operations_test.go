package ops

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/config"
	"github.com/JungHoonGhae/tossinvest-cli/internal/official"
	"github.com/JungHoonGhae/tossinvest-cli/internal/orderintent"
	"github.com/JungHoonGhae/tossinvest-cli/internal/trading"
)

// 에이전트(MCP)가 주문을 내는 진입부다. 게이트 본체는 internal/trading 에서 덮여
// 있지만, **에이전트의 인자가 그 게이트까지 도달하는 경로**는 지금까지 검증되지
// 않았다. 여기서 실수하면 의도하지 않은 주문이 나간다.

// recordingBroker 는 호출 여부만 기록한다. 게이트가 열렸는지를 "브로커가 불렸는가"
// 로 판정하기 위한 것이라 응답 내용은 의미 없다.
type recordingBroker struct {
	placed    int
	canceled  int
	amended   int
	actionsOf []string
}

func (b *recordingBroker) PlacePendingOrder(context.Context, orderintent.PlaceIntent) (trading.MutationResult, error) {
	b.placed++
	return trading.MutationResult{}, nil
}

func (b *recordingBroker) GetOrderAvailableActions(_ context.Context, id string) (map[string]any, error) {
	b.actionsOf = append(b.actionsOf, id)
	return map[string]any{}, nil
}

func (b *recordingBroker) CancelPendingOrder(context.Context, orderintent.CancelIntent) (trading.MutationResult, error) {
	b.canceled++
	return trading.MutationResult{}, nil
}

func (b *recordingBroker) AmendPendingOrder(context.Context, orderintent.AmendIntent) (trading.MutationResult, error) {
	b.amended++
	return trading.MutationResult{}, nil
}

func (b *recordingBroker) total() int { return b.placed + b.canceled + b.amended }

// writeDeps 는 쓰기 오퍼레이션을 부를 수 있는 최소 Deps 를 만든다.
//
// Client 는 핸들러가 쓰지 않지만 Catalog.Call 의 사전조건이 non-nil 을 요구한다
// (ops.go 의 official 분기). 그래서 빈 값을 넣는다.
func writeDeps(policy config.Trading) (*Deps, *recordingBroker) {
	b := &recordingBroker{}
	return &Deps{
		Client:  &official.Client{},
		Trading: trading.NewService(policy, b),
		Auth:    AuthStatus{Official: BackendStatus{Connected: true}},
	}, b
}

func allowAll() config.Trading {
	return config.Trading{
		Place: true, Cancel: true, Amend: true, Sell: true,
		AllowLiveOrderActions: true,
	}
}

func placeArgs() map[string]any {
	return map[string]any{
		"symbol": "005930", "market": "kr", "side": "buy",
		"order_type": "limit", "quantity": 1.0, "price": 70000.0,
	}
}

// 1) execute 를 주지 않으면 미리보기여야 하고 브로커까지 내려가면 안 된다.
func TestWriteOpsWithoutExecuteReturnPreview(t *testing.T) {
	cases := []struct {
		id   string
		args map[string]any
	}{
		{"place_order", placeArgs()},
		{"cancel_order", map[string]any{"order_id": "A1", "symbol": "005930"}},
		{"modify_order", map[string]any{"order_id": "A1", "quantity": 2.0}},
	}
	for _, c := range cases {
		t.Run(c.id, func(t *testing.T) {
			deps, broker := writeDeps(allowAll())
			res, err := NewCatalog().Call(context.Background(), deps, c.id, c.args)
			if err != nil {
				t.Fatalf("Call: %v", err)
			}
			if _, ok := res.(trading.Preview); !ok {
				t.Errorf("expected a Preview, got %T", res)
			}
			if broker.total() != 0 {
				t.Errorf("execute 없이 브로커가 호출됐다 (%d회)", broker.total())
			}
		})
	}
}

//  2. execute=true 인데 토큰이 틀리면 거절돼야 한다. 미리보기 토큰을 모르는 채로
//     execute 만 켜서 주문이 나가면 2단계 확인이 무의미해진다.
func TestWriteOpsRejectWrongConfirmToken(t *testing.T) {
	for _, id := range []string{"place_order", "cancel_order", "modify_order"} {
		t.Run(id, func(t *testing.T) {
			deps, broker := writeDeps(allowAll())
			args := map[string]any{"execute": true, "confirm": "not-the-token"}
			switch id {
			case "place_order":
				for k, v := range placeArgs() {
					args[k] = v
				}
			case "cancel_order":
				args["order_id"], args["symbol"] = "A1", "005930"
			case "modify_order":
				args["order_id"], args["quantity"] = "A1", 2.0
			}
			_, err := NewCatalog().Call(context.Background(), deps, id, args)
			if !errors.Is(err, trading.ErrConfirmMismatch) {
				t.Fatalf("expected ErrConfirmMismatch, got %v", err)
			}
			if broker.total() != 0 {
				t.Errorf("토큰 불일치인데 브로커가 호출됐다 (%d회)", broker.total())
			}
		})
	}
}

//  3. 올바른 토큰이라도 config 가 막고 있으면 나가면 안 된다. 토큰은 "의도 확인"
//     이고 config 는 "권한" 이라 둘은 독립적으로 걸려야 한다.
func TestWriteOpsRespectConfigGate(t *testing.T) {
	deps, broker := writeDeps(config.Trading{Place: true, AllowLiveOrderActions: false})
	intent, err := orderintent.NormalizePlace(orderintent.PlaceInput{
		Symbol: "005930", Market: "kr", Side: "buy", OrderType: "limit", Quantity: 1, Price: 70000,
	})
	if err != nil {
		t.Fatal(err)
	}
	args := placeArgs()
	args["execute"] = true
	args["confirm"] = orderintent.ConfirmToken(orderintent.CanonicalPlace(intent))

	_, callErr := NewCatalog().Call(context.Background(), deps, "place_order", args)
	if !errors.Is(callErr, trading.ErrLiveActionsDisabled) {
		t.Fatalf("expected ErrLiveActionsDisabled, got %v", callErr)
	}
	if broker.total() != 0 {
		t.Errorf("config 가 막았는데 브로커가 호출됐다 (%d회)", broker.total())
	}
}

//  4. 올바른 토큰 + 허용된 config 면 실제로 나가야 한다. 위 세 테스트가 "항상
//     거절" 로도 통과해버리는 것을 막는 대조군이다.
func TestWriteOpsExecuteWithValidTokenReachesBroker(t *testing.T) {
	deps, broker := writeDeps(allowAll())
	intent, err := orderintent.NormalizePlace(orderintent.PlaceInput{
		Symbol: "005930", Market: "kr", Side: "buy", OrderType: "limit", Quantity: 1, Price: 70000,
	})
	if err != nil {
		t.Fatal(err)
	}
	args := placeArgs()
	args["execute"] = true
	args["confirm"] = orderintent.ConfirmToken(orderintent.CanonicalPlace(intent))

	if _, err := NewCatalog().Call(context.Background(), deps, "place_order", args); err != nil {
		t.Fatalf("Call: %v", err)
	}
	if broker.placed != 1 {
		t.Errorf("정상 경로인데 주문이 나가지 않았다 (placed=%d)", broker.placed)
	}
}

//  5. 수량·가격은 돈이 실리는 인자다. MCP 로 오면 JSON 숫자라 늘 float64 지만
//     Go 쪽 호출자는 int 를 넘긴다 — argInt 와 같은 관용도를 유지한다. 숫자가
//     아닌 값이 조용히 0 이 되면 "1주" 가 "0주" 로 나가므로 반드시 거절해야 한다.
func TestWriteOpNumericArgs(t *testing.T) {
	for _, q := range []any{2.0, 2} {
		deps, broker := writeDeps(allowAll())
		args := placeArgs()
		args["quantity"] = q
		if _, err := NewCatalog().Call(context.Background(), deps, "place_order", args); err != nil {
			t.Errorf("quantity=%v(%T) 를 받아야 한다: %v", q, q, err)
		}
		if broker.total() != 0 {
			t.Error("미리보기여야 하는데 브로커가 호출됐다")
		}
	}

	deps, _ := writeDeps(allowAll())
	for _, bad := range []any{"많이", true, nil} {
		args := placeArgs()
		args["quantity"] = bad
		_, err := NewCatalog().Call(context.Background(), deps, "place_order", args)
		if err == nil {
			t.Errorf("quantity=%v(%T) 는 거절돼야 한다", bad, bad)
			continue
		}
		if !strings.Contains(err.Error(), "quantity") {
			t.Errorf("어느 인자가 문제인지 알려줘야 한다: %v", err)
		}
	}
}
