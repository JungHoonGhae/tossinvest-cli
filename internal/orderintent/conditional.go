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

type ConditionalType string

const (
	ConditionalSingle ConditionalType = "SINGLE"
	ConditionalOCO    ConditionalType = "OCO"
	ConditionalOTO    ConditionalType = "OTO"
)

func ParseConditionalType(value string) (ConditionalType, error) {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return ConditionalSingle, nil
	}
	kind := ConditionalType(value)
	switch kind {
	case ConditionalSingle, ConditionalOCO, ConditionalOTO:
		return kind, nil
	default:
		return "", fmt.Errorf("unsupported conditional order type %q; expected SINGLE, OCO or OTO", value)
	}
}

func (t ConditionalType) RequiresSecondLeg() bool {
	return t == ConditionalOCO || t == ConditionalOTO
}

type ConditionalOrderType string

const (
	ConditionalLimit  ConditionalOrderType = "LIMIT"
	ConditionalMarket ConditionalOrderType = "MARKET"
)

// ConditionalShape is the shared economic shape of create and modify requests.
// Keeping it as one value prevents those two flows from drifting as the
// official conditional-order schema evolves.
type ConditionalShape struct {
	Type       ConditionalType
	OrderType  ConditionalOrderType
	ExpireDate string
	Quantity   float64
	First      ConditionLeg
	Second     *ConditionLeg // OCO/OTO
}

// ConditionalPlaceIntent is a request to create a conditional order.
type ConditionalPlaceIntent struct {
	Symbol string
	ConditionalShape
	ClientOrderID    string
	ConfirmHighValue bool
}

// ConditionalCancelIntent is a request to cancel a conditional order by id.
type ConditionalCancelIntent struct{ ID string }

// ConditionalModifyIntent is a request to modify an existing conditional order.
type ConditionalModifyIntent struct {
	ID string
	ConditionalShape
	ConfirmHighValue bool
}

func NormalizeConditionalPlace(intent ConditionalPlaceIntent) (ConditionalPlaceIntent, error) {
	intent.Symbol = strings.ToUpper(strings.TrimSpace(intent.Symbol))
	if intent.Symbol == "" {
		return ConditionalPlaceIntent{}, fmt.Errorf("symbol is required")
	}
	intent.ClientOrderID = strings.TrimSpace(intent.ClientOrderID)
	shape, err := NormalizeConditionalShape(intent.ConditionalShape)
	if err != nil {
		return ConditionalPlaceIntent{}, err
	}
	intent.ConditionalShape = shape
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
	shape, err := NormalizeConditionalShape(intent.ConditionalShape)
	if err != nil {
		return ConditionalModifyIntent{}, err
	}
	intent.ConditionalShape = shape
	return intent, nil
}

func NormalizeConditionalShape(shape ConditionalShape) (ConditionalShape, error) {
	kind, err := ParseConditionalType(string(shape.Type))
	if err != nil {
		return ConditionalShape{}, err
	}
	shape.Type = kind

	orderType := strings.ToUpper(strings.TrimSpace(string(shape.OrderType)))
	if orderType == "" {
		orderType = string(ConditionalLimit)
	}
	shape.OrderType = ConditionalOrderType(orderType)
	if shape.OrderType != ConditionalLimit && shape.OrderType != ConditionalMarket {
		return ConditionalShape{}, fmt.Errorf("unsupported order type %q; expected LIMIT or MARKET", orderType)
	}
	if shape.Type.RequiresSecondLeg() && shape.OrderType != ConditionalLimit {
		return ConditionalShape{}, fmt.Errorf("conditional order type %s requires LIMIT order type", shape.Type)
	}

	shape.ExpireDate = strings.TrimSpace(shape.ExpireDate)
	if _, err := time.Parse("2006-01-02", shape.ExpireDate); err != nil {
		return ConditionalShape{}, fmt.Errorf("expire date must use YYYY-MM-DD")
	}
	if shape.Quantity <= 0 {
		return ConditionalShape{}, fmt.Errorf("quantity must be greater than zero")
	}
	if err := normalizeConditionLeg(&shape.First, shape.OrderType, "first"); err != nil {
		return ConditionalShape{}, err
	}

	if !shape.Type.RequiresSecondLeg() {
		if shape.Second != nil {
			return ConditionalShape{}, fmt.Errorf("second condition must be omitted for SINGLE orders")
		}
		return shape, nil
	}
	if shape.Second == nil {
		return ConditionalShape{}, fmt.Errorf("second condition is required for %s orders", shape.Type)
	}
	leg := *shape.Second
	if err := normalizeConditionLeg(&leg, shape.OrderType, "second"); err != nil {
		return ConditionalShape{}, err
	}
	shape.Second = &leg
	return shape, nil
}

func normalizeConditionLeg(leg *ConditionLeg, orderType ConditionalOrderType, name string) error {
	leg.OrderSide = strings.ToUpper(strings.TrimSpace(leg.OrderSide))
	if leg.OrderSide != "BUY" && leg.OrderSide != "SELL" {
		return fmt.Errorf("%s side must be BUY or SELL", name)
	}
	if leg.TriggerPrice <= 0 {
		return fmt.Errorf("%s trigger price must be greater than zero", name)
	}
	if orderType == ConditionalLimit && leg.OrderPrice <= 0 {
		return fmt.Errorf("%s order price must be greater than zero for LIMIT orders", name)
	}
	if orderType == ConditionalMarket && leg.OrderPrice != 0 {
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
	return fmt.Sprintf("cplace|%s|%s|%s|%s|%s|%s|%s|%s|%t",
		i.Symbol, i.Type, i.OrderType, i.ExpireDate, fmtFloat(i.Quantity),
		legCanonical(i.First), secondCanonical(i.Second), i.ClientOrderID, i.ConfirmHighValue)
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
