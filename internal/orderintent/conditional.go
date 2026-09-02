package orderintent

import (
	"fmt"
	"strconv"
	"strings"
	"time"
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

func NormalizeConditionalPlace(intent ConditionalPlaceIntent) (ConditionalPlaceIntent, error) {
	intent.Symbol = strings.ToUpper(strings.TrimSpace(intent.Symbol))
	if intent.Symbol == "" {
		return ConditionalPlaceIntent{}, fmt.Errorf("symbol is required")
	}
	intent.ClientOrderID = strings.TrimSpace(intent.ClientOrderID)
	if err := normalizeConditionalShape(
		&intent.Type, &intent.OrderType, &intent.ExpireDate,
		intent.Quantity, &intent.First, &intent.Second,
	); err != nil {
		return ConditionalPlaceIntent{}, err
	}
	return intent, nil
}

func NormalizeConditionalCancel(intent ConditionalCancelIntent) (ConditionalCancelIntent, error) {
	intent.ID = strings.TrimSpace(intent.ID)
	if intent.ID == "" {
		return ConditionalCancelIntent{}, fmt.Errorf("conditional order id is required")
	}
	return intent, nil
}

func NormalizeConditionalModify(intent ConditionalModifyIntent) (ConditionalModifyIntent, error) {
	intent.ID = strings.TrimSpace(intent.ID)
	if intent.ID == "" {
		return ConditionalModifyIntent{}, fmt.Errorf("conditional order id is required")
	}
	if err := normalizeConditionalShape(
		&intent.Type, &intent.OrderType, &intent.ExpireDate,
		intent.Quantity, &intent.First, &intent.Second,
	); err != nil {
		return ConditionalModifyIntent{}, err
	}
	return intent, nil
}

func normalizeConditionalShape(conditionalType, orderType, expireDate *string, quantity float64, first *ConditionLeg, second **ConditionLeg) error {
	*conditionalType = strings.ToUpper(strings.TrimSpace(*conditionalType))
	if *conditionalType == "" {
		*conditionalType = "SINGLE"
	}
	switch *conditionalType {
	case "SINGLE", "OCO", "OTO":
	default:
		return fmt.Errorf("unsupported conditional order type %q; expected SINGLE, OCO or OTO", *conditionalType)
	}

	*orderType = strings.ToUpper(strings.TrimSpace(*orderType))
	if *orderType == "" {
		*orderType = "LIMIT"
	}
	if *orderType != "LIMIT" && *orderType != "MARKET" {
		return fmt.Errorf("unsupported order type %q; expected LIMIT or MARKET", *orderType)
	}
	if (*conditionalType == "OCO" || *conditionalType == "OTO") && *orderType != "LIMIT" {
		return fmt.Errorf("conditional order type %s requires LIMIT order type", *conditionalType)
	}

	*expireDate = strings.TrimSpace(*expireDate)
	if _, err := time.Parse("2006-01-02", *expireDate); err != nil {
		return fmt.Errorf("expire date must use YYYY-MM-DD")
	}
	if quantity <= 0 {
		return fmt.Errorf("quantity must be greater than zero")
	}
	if err := normalizeConditionLeg(first, *orderType, "first"); err != nil {
		return err
	}

	if *conditionalType == "SINGLE" {
		if *second != nil {
			return fmt.Errorf("second condition must be omitted for SINGLE orders")
		}
		return nil
	}
	if *second == nil {
		return fmt.Errorf("second condition is required for %s orders", *conditionalType)
	}
	leg := **second
	if err := normalizeConditionLeg(&leg, *orderType, "second"); err != nil {
		return err
	}
	*second = &leg
	return nil
}

func normalizeConditionLeg(leg *ConditionLeg, orderType, name string) error {
	leg.OrderSide = strings.ToUpper(strings.TrimSpace(leg.OrderSide))
	if leg.OrderSide != "BUY" && leg.OrderSide != "SELL" {
		return fmt.Errorf("%s side must be BUY or SELL", name)
	}
	if leg.TriggerPrice <= 0 {
		return fmt.Errorf("%s trigger price must be greater than zero", name)
	}
	if orderType == "LIMIT" && leg.OrderPrice <= 0 {
		return fmt.Errorf("%s order price must be greater than zero for LIMIT orders", name)
	}
	if orderType == "MARKET" && leg.OrderPrice != 0 {
		return fmt.Errorf("%s order price must be omitted for MARKET orders", name)
	}
	return nil
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
