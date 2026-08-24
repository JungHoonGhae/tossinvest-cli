package orderintent

import "testing"

func base() PlaceInput {
	return PlaceInput{Symbol: "AAPL", Market: "us", Side: "buy", OrderType: "limit", Quantity: 1, Price: 10}
}

func TestTimeInForceCombinationRules(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*PlaceInput)
		wantErr bool
	}{
		{"기본값은 빈 값 (DAY)", func(i *PlaceInput) {}, false},
		{"DAY 명시", func(i *PlaceInput) { i.TimeInForce = "DAY" }, false},
		{"CLS + 미국 + 지정가", func(i *PlaceInput) { i.TimeInForce = "CLS" }, false},
		{"CLS + 시장가는 거절", func(i *PlaceInput) { i.TimeInForce = "CLS"; i.OrderType = "market" }, true},
		{"CLS + 국내는 거절", func(i *PlaceInput) {
			i.TimeInForce = "CLS"
			i.Symbol, i.Market = "005930", "kr"
		}, true},
		{"OPG + 국내 지정가", func(i *PlaceInput) {
			i.TimeInForce = "OPG"
			i.Symbol, i.Market = "005930", "kr"
		}, false},
		// spec: OPG 는 orderType LIMIT/MARKET 둘 다 지원한다 (CLS 와 다른 점).
		{"OPG + 국내 시장가도 허용", func(i *PlaceInput) {
			i.TimeInForce = "OPG"
			i.Symbol, i.Market, i.OrderType, i.Price = "005930", "kr", "market", 0
		}, false},
		{"OPG + 미국은 거절", func(i *PlaceInput) { i.TimeInForce = "OPG" }, true},
		{"모르는 코드는 거절", func(i *PlaceInput) { i.TimeInForce = "GTC" }, true},
		{"소문자도 받는다", func(i *PlaceInput) { i.TimeInForce = "cls" }, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := base()
			c.mutate(&in)
			_, err := NormalizePlace(in)
			if c.wantErr && err == nil {
				t.Errorf("expected an error, got none")
			}
			if !c.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// 소수점 주문은 금액 기반 스키마로 나가는데 거기엔 timeInForce 필드 자체가 없다.
// 조용히 버려지면 사용자가 지정한 조건 없이 체결된다.
func TestTimeInForceRejectedForFractional(t *testing.T) {
	in := base()
	in.OrderType, in.Fractional, in.Amount, in.Quantity, in.Price = "market", true, 10000, 0, 0
	in.TimeInForce = "CLS"
	if _, err := NormalizePlace(in); err == nil {
		t.Error("fractional order must reject an explicit time-in-force")
	}
}

// confirm 토큰은 intent 를 묶는다. TimeInForce 가 빠지면 DAY 로 미리보기해 받은
// 토큰으로 OPG 주문을 낼 수 있다.
func TestConfirmTokenBindsTimeInForce(t *testing.T) {
	day := base()
	day.Symbol, day.Market = "005930", "kr"
	dayIntent, err := NormalizePlace(day)
	if err != nil {
		t.Fatal(err)
	}
	opg := day
	opg.TimeInForce = "OPG"
	opgIntent, err := NormalizePlace(opg)
	if err != nil {
		t.Fatal(err)
	}
	if CanonicalPlace(dayIntent) == CanonicalPlace(opgIntent) {
		t.Error("a DAY preview token must not authorize an OPG order")
	}
}

// 반대로, 값을 안 준 주문의 토큰은 이 기능이 생기기 전과 같아야 한다 — 아니면
// 이미 발급된 토큰이 전부 무효가 된다.
func TestConfirmTokenUnchangedWhenUnset(t *testing.T) {
	in := base()
	intent, err := NormalizePlace(in)
	if err != nil {
		t.Fatal(err)
	}
	explicitDay := in
	explicitDay.TimeInForce = "DAY"
	dayIntent, err := NormalizePlace(explicitDay)
	if err != nil {
		t.Fatal(err)
	}
	if CanonicalPlace(intent) != CanonicalPlace(dayIntent) {
		t.Error("DAY is the default — its token must match an unset one")
	}
}
