package orderintent

import (
	"fmt"
	"math"
	"strings"
)

// OptionPlaceIntent is the target-independent order intent shared by paper
// execution and the later live-order preview. Exchange is transport metadata
// resolved from the contract; it is deliberately not the live environment's
// market selector (which is always "us").
type OptionPlaceIntent struct {
	Symbol       string  `json:"symbol"`
	Exchange     string  `json:"exchange"`
	CurrencyMode string  `json:"currency_mode"`
	Side         string  `json:"side"`
	OrderType    string  `json:"order_type"`
	Price        float64 `json:"price"`
	Quantity     int     `json:"quantity"`
}

type OptionPlaceInput struct {
	Symbol       string
	Exchange     string
	CurrencyMode string
	Side         string
	OrderType    string
	Price        float64
	Quantity     int
}

func NormalizeOptionPlace(input OptionPlaceInput) (OptionPlaceIntent, error) {
	symbol := strings.ToUpper(strings.TrimSpace(input.Symbol))
	if !strings.HasPrefix(symbol, "OPT_") {
		return OptionPlaceIntent{}, fmt.Errorf("US options require an OPT_ contract code")
	}
	exchange := strings.ToUpper(strings.TrimSpace(input.Exchange))
	currency := strings.ToUpper(strings.TrimSpace(input.CurrencyMode))
	if currency == "" {
		currency = "USD"
	}
	if currency != "USD" && currency != "KRW" {
		return OptionPlaceIntent{}, fmt.Errorf("option currency mode must be USD or KRW")
	}
	side := strings.ToLower(strings.TrimSpace(input.Side))
	if side != "buy" && side != "sell" {
		return OptionPlaceIntent{}, fmt.Errorf("option order side must be buy or sell")
	}
	orderType := strings.ToLower(strings.TrimSpace(input.OrderType))
	if orderType == "" {
		orderType = "limit"
	}
	price := input.Price
	switch orderType {
	case "limit":
		if price <= 0 || math.IsNaN(price) || math.IsInf(price, 0) {
			return OptionPlaceIntent{}, fmt.Errorf("option limit price must be greater than zero")
		}
	case "market":
		price = 0
	default:
		return OptionPlaceIntent{}, fmt.Errorf("option order type must be limit or market")
	}
	if input.Quantity <= 0 {
		return OptionPlaceIntent{}, fmt.Errorf("option quantity must be a positive whole number")
	}
	return OptionPlaceIntent{
		Symbol: symbol, Exchange: exchange, CurrencyMode: currency,
		Side: side, OrderType: orderType, Price: price, Quantity: input.Quantity,
	}, nil
}

// LiveIntent changes only the execution environment. The economic intent is
// preserved and then re-enters the normal live normalization/guard pipeline.
func (i OptionPlaceIntent) LiveIntent() (PlaceIntent, error) {
	return NormalizePlace(PlaceInput{
		Symbol: i.Symbol, Market: "us", Side: i.Side, OrderType: i.OrderType,
		Quantity: float64(i.Quantity), Price: i.Price, CurrencyMode: i.CurrencyMode,
	})
}

// PortableCanonical is intentionally identical to the live canonical intent.
// A paper execution never authorizes live execution; the live service must
// issue its own confirmation token after applying live config and risk checks.
func (i OptionPlaceIntent) PortableCanonical() string {
	live, err := i.LiveIntent()
	if err != nil {
		return ""
	}
	return CanonicalPlace(live)
}
