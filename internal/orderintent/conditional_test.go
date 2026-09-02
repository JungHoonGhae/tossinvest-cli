package orderintent

import (
	"strings"
	"testing"
)

func TestNormalizeConditionalPlaceDefaultsAndCanonicalizes(t *testing.T) {
	got, err := NormalizeConditionalPlace(ConditionalPlaceIntent{
		Symbol: " aapl ",
		ConditionalShape: ConditionalShape{
			ExpireDate: " 2026-12-31 ", Quantity: 1,
			First: ConditionLeg{OrderSide: " buy ", TriggerPrice: 100, OrderPrice: 99},
		},
	})
	if err != nil {
		t.Fatalf("NormalizeConditionalPlace: %v", err)
	}
	if got.Symbol != "AAPL" || got.Type != "SINGLE" || got.OrderType != "LIMIT" || got.ExpireDate != "2026-12-31" || got.First.OrderSide != "BUY" {
		t.Fatalf("unexpected normalized intent: %+v", got)
	}
}

func TestNormalizeConditionalPlaceValidatesOfficialShape(t *testing.T) {
	base := ConditionalPlaceIntent{
		Symbol: "005930",
		ConditionalShape: ConditionalShape{
			Type: "SINGLE", OrderType: "LIMIT", ExpireDate: "2026-12-31", Quantity: 1,
			First: ConditionLeg{OrderSide: "BUY", TriggerPrice: 70000, OrderPrice: 69900},
		},
	}
	cases := []struct {
		name string
		edit func(*ConditionalPlaceIntent)
		want string
	}{
		{name: "missing symbol", edit: func(i *ConditionalPlaceIntent) { i.Symbol = "" }, want: "symbol"},
		{name: "bad type", edit: func(i *ConditionalPlaceIntent) { i.Type = "TRAILING" }, want: "type"},
		{name: "bad expiry", edit: func(i *ConditionalPlaceIntent) { i.ExpireDate = "tomorrow" }, want: "expire"},
		{name: "zero quantity", edit: func(i *ConditionalPlaceIntent) { i.Quantity = 0 }, want: "quantity"},
		{name: "bad side", edit: func(i *ConditionalPlaceIntent) { i.First.OrderSide = "HOLD" }, want: "side"},
		{name: "missing trigger", edit: func(i *ConditionalPlaceIntent) { i.First.TriggerPrice = 0 }, want: "trigger"},
		{name: "limit missing price", edit: func(i *ConditionalPlaceIntent) { i.First.OrderPrice = 0 }, want: "order price"},
		{name: "market with price", edit: func(i *ConditionalPlaceIntent) { i.OrderType = "MARKET" }, want: "order price"},
		{name: "oco missing second", edit: func(i *ConditionalPlaceIntent) { i.Type = "OCO" }, want: "second"},
		{name: "oco market", edit: func(i *ConditionalPlaceIntent) {
			i.Type = "OCO"
			i.OrderType = "MARKET"
			i.First.OrderPrice = 0
			i.Second = &ConditionLeg{OrderSide: "SELL", TriggerPrice: 80000}
		}, want: "LIMIT"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			in := base
			tt.edit(&in)
			_, err := NormalizeConditionalPlace(in)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got %v", tt.want, err)
			}
		})
	}
}

func TestNormalizeConditionalCancelAndModifyRequireIDs(t *testing.T) {
	if _, err := NormalizeConditionalCancel(ConditionalCancelIntent{}); err == nil {
		t.Fatal("expected empty cancel id to fail")
	}
	if _, err := NormalizeConditionalModify(ConditionalModifyIntent{}); err == nil {
		t.Fatal("expected empty modify id to fail")
	}
}

func TestCanonicalConditionalPlaceDeterministic(t *testing.T) {
	i := ConditionalPlaceIntent{
		Symbol: "005930",
		ConditionalShape: ConditionalShape{
			Type: "SINGLE", OrderType: "LIMIT", ExpireDate: "2026-12-31",
			Quantity: 10, First: ConditionLeg{OrderSide: "SELL", TriggerPrice: 70000, OrderPrice: 69900},
		},
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
	j = i
	j.ClientOrderID = "different-idempotency-key"
	if CanonicalConditionalPlace(j) == a {
		t.Fatal("client order id must be bound to the confirmation token")
	}
	if ConfirmToken(a) == "" {
		t.Fatalf("confirm token empty")
	}
}

func TestCanonicalConditionalCancelModify(t *testing.T) {
	if CanonicalConditionalCancel(ConditionalCancelIntent{ID: "co-1"}) == "" {
		t.Fatalf("cancel canonical empty")
	}
	m := ConditionalModifyIntent{
		ID: "co-1",
		ConditionalShape: ConditionalShape{
			Type: "SINGLE", OrderType: "MARKET", ExpireDate: "2026-12-31", Quantity: 5,
			First: ConditionLeg{OrderSide: "SELL", TriggerPrice: 68000},
		},
	}
	if CanonicalConditionalModify(m) == "" {
		t.Fatalf("modify canonical empty")
	}
}
