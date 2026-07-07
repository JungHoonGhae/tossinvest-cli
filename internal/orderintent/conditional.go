package orderintent

import (
	"fmt"
	"strconv"
)

// ConditionLeg is one watch condition of a conditional order request.
// OrderSide: BUY|SELL. TriggerPrice: price that triggers the leg. OrderPrice:
// limit price (0 = MARKET / unset).
type ConditionLeg struct {
	OrderSide    string
	TriggerPrice float64
	OrderPrice   float64
}

// ConditionalPlaceIntent is a request to create a conditional order.
type ConditionalPlaceIntent struct {
	Symbol           string
	Type             string // SINGLE|OCO|OTO
	OrderType        string // LIMIT|MARKET
	ExpireDate       string
	Quantity         float64
	First            ConditionLeg
	Second           *ConditionLeg // OCO/OTO
	ClientOrderID    string
	ConfirmHighValue bool
}

// ConditionalCancelIntent is a request to cancel a conditional order by id.
type ConditionalCancelIntent struct{ ID string }

// ConditionalModifyIntent is a request to modify an existing conditional order.
type ConditionalModifyIntent struct {
	ID               string
	Type             string
	OrderType        string
	ExpireDate       string
	Quantity         float64
	First            ConditionLeg
	Second           *ConditionLeg
	ConfirmHighValue bool
}

func fmtFloat(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func legCanonical(l ConditionLeg) string {
	return fmt.Sprintf("%s:%s:%s", l.OrderSide, fmtFloat(l.TriggerPrice), fmtFloat(l.OrderPrice))
}

func secondCanonical(s *ConditionLeg) string {
	if s == nil {
		return "-"
	}
	return legCanonical(*s)
}

// CanonicalConditionalPlace builds a deterministic string for confirm-token hashing.
func CanonicalConditionalPlace(i ConditionalPlaceIntent) string {
	return fmt.Sprintf("cplace|%s|%s|%s|%s|%s|%s|%s|%t",
		i.Symbol, i.Type, i.OrderType, i.ExpireDate, fmtFloat(i.Quantity),
		legCanonical(i.First), secondCanonical(i.Second), i.ConfirmHighValue)
}

// CanonicalConditionalCancel builds a deterministic string for a cancel.
func CanonicalConditionalCancel(i ConditionalCancelIntent) string {
	return "ccancel|" + i.ID
}

// CanonicalConditionalModify builds a deterministic string for a modify.
func CanonicalConditionalModify(i ConditionalModifyIntent) string {
	return fmt.Sprintf("cmodify|%s|%s|%s|%s|%s|%s|%s|%t",
		i.ID, i.Type, i.OrderType, i.ExpireDate, fmtFloat(i.Quantity),
		legCanonical(i.First), secondCanonical(i.Second), i.ConfirmHighValue)
}
