package orderintent

import "testing"

func TestCanonicalConditionalPlaceDeterministic(t *testing.T) {
	i := ConditionalPlaceIntent{
		Symbol: "005930", Type: "SINGLE", OrderType: "LIMIT", ExpireDate: "2026-12-31",
		Quantity: 10, First: ConditionLeg{OrderSide: "SELL", TriggerPrice: 70000, OrderPrice: 69900},
	}
	a := CanonicalConditionalPlace(i)
	b := CanonicalConditionalPlace(i)
	if a != b {
		t.Fatalf("canonical not deterministic: %q vs %q", a, b)
	}
	if a == "" {
		t.Fatalf("canonical empty")
	}
	// SINGLE(no second) vs OCO(with second) must differ
	j := i
	sec := ConditionLeg{OrderSide: "BUY", TriggerPrice: 60000}
	j.Second = &sec
	j.Type = "OCO"
	if CanonicalConditionalPlace(j) == a {
		t.Fatalf("SINGLE and OCO canonical must differ")
	}
	if ConfirmToken(a) == "" {
		t.Fatalf("confirm token empty")
	}
}

func TestCanonicalConditionalCancelModify(t *testing.T) {
	if CanonicalConditionalCancel(ConditionalCancelIntent{ID: "co-1"}) == "" {
		t.Fatalf("cancel canonical empty")
	}
	m := ConditionalModifyIntent{ID: "co-1", Type: "SINGLE", OrderType: "MARKET", ExpireDate: "2026-12-31", Quantity: 5, First: ConditionLeg{OrderSide: "SELL", TriggerPrice: 68000}}
	if CanonicalConditionalModify(m) == "" {
		t.Fatalf("modify canonical empty")
	}
}
