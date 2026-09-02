package orderintent

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/JungHoonGhae/tossinvest-cli/internal/confirmation"
)

type PlaceIntent struct {
	Symbol       string  `json:"symbol"`
	Market       string  `json:"market"`
	Side         string  `json:"side"`
	OrderType    string  `json:"order_type"`
	Quantity     float64 `json:"quantity"`
	Price        float64 `json:"price,omitempty"`
	Amount       float64 `json:"amount,omitempty"`
	CurrencyMode string  `json:"currency_mode"`
	Fractional   bool    `json:"fractional"`
	// TimeInForce is the order's validity condition. Empty means DAY (the
	// server default) and is what every order used before this field existed.
	TimeInForce string `json:"time_in_force,omitempty"`
}

type CancelIntent struct {
	OrderID string `json:"order_id"`
	Symbol  string `json:"symbol"`
}

type AmendIntent struct {
	OrderID  string   `json:"order_id"`
	Quantity *float64 `json:"quantity,omitempty"`
	Price    *float64 `json:"price,omitempty"`
}

func (i AmendIntent) GetOrderID() string    { return i.OrderID }
func (i AmendIntent) GetQuantity() *float64 { return i.Quantity }
func (i AmendIntent) GetPrice() *float64    { return i.Price }

type PlaceInput struct {
	Symbol       string
	Market       string
	Side         string
	OrderType    string
	Quantity     float64
	Price        float64
	Amount       float64
	CurrencyMode string
	Fractional   bool
	TimeInForce  string
}

func NormalizePlace(input PlaceInput) (PlaceIntent, error) {
	symbol := strings.TrimSpace(strings.ToUpper(input.Symbol))
	if symbol == "" {
		return PlaceIntent{}, fmt.Errorf("symbol is required")
	}
	side := strings.TrimSpace(strings.ToLower(input.Side))
	if side == "" {
		return PlaceIntent{}, fmt.Errorf("side is required")
	}

	orderType := normalizeDefault(input.OrderType, "limit")
	fractional := input.Fractional

	// Fractional orders must be market orders
	if fractional && orderType == "limit" {
		orderType = "market"
	}

	intent := PlaceIntent{
		Symbol:       symbol,
		Market:       normalizeDefault(input.Market, "us"),
		Side:         side,
		OrderType:    orderType,
		Quantity:     input.Quantity,
		Price:        input.Price,
		Amount:       input.Amount,
		CurrencyMode: normalizeCurrencyMode(input.CurrencyMode),
		Fractional:   fractional,
		TimeInForce:  strings.ToUpper(strings.TrimSpace(input.TimeInForce)),
	}

	// A Korean stock code (6 digits, optionally A-prefixed) is never a valid US
	// ticker, so when the symbol clearly looks Korean we route it to the KR
	// market automatically instead of rejecting. The default --market is "us",
	// and forcing users to also pass --market kr for an obviously-Korean code
	// was pure friction. An explicit non-us market is left untouched.
	if intent.Market == "us" && looksLikeKRSymbol(intent.Symbol) {
		intent.Market = "kr"
	}
	if intent.Fractional && intent.Market != "us" {
		return PlaceIntent{}, fmt.Errorf("fractional orders are only supported for US stocks")
	}
	if intent.Side != "buy" && intent.Side != "sell" {
		return PlaceIntent{}, fmt.Errorf("unsupported side %q; expected buy or sell", input.Side)
	}
	if intent.OrderType != "limit" && intent.OrderType != "market" {
		return PlaceIntent{}, fmt.Errorf("unsupported order type %q; expected limit or market", input.OrderType)
	}
	if err := validateTimeInForce(&intent); err != nil {
		return PlaceIntent{}, err
	}
	if intent.Fractional {
		// Fractional orders are US market orders. BUY is amount-based
		// (`orderAmount`); SELL is quantity-based (a decimal share count) —
		// matching the official Open API (≥1.1.5): a decimal `quantity` is
		// accepted only for US MARKET SELL, up to 6 decimal places.
		intent.Price = 0
		if intent.Side == "sell" {
			if intent.Quantity <= 0 {
				return PlaceIntent{}, fmt.Errorf("quantity must be greater than zero for fractional sell orders")
			}
			if decimalPlaces(intent.Quantity) > 6 {
				return PlaceIntent{}, fmt.Errorf("fractional quantity supports up to 6 decimal places")
			}
			intent.Amount = 0
		} else {
			if intent.Amount <= 0 {
				return PlaceIntent{}, fmt.Errorf("amount must be greater than zero for fractional buy orders")
			}
			intent.Quantity = 0
		}
	} else {
		if intent.Quantity <= 0 {
			return PlaceIntent{}, fmt.Errorf("quantity must be greater than zero")
		}
		if intent.OrderType == "limit" && intent.Price <= 0 {
			return PlaceIntent{}, fmt.Errorf("price must be greater than zero for limit orders")
		}
		if intent.OrderType == "market" {
			intent.Price = 0
		}
	}

	return intent, nil
}

func NormalizeCancel(orderID, symbol string) (CancelIntent, error) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return CancelIntent{}, fmt.Errorf("order-id is required")
	}
	symbol = strings.TrimSpace(strings.ToUpper(symbol))
	if symbol == "" {
		return CancelIntent{}, fmt.Errorf("symbol is required")
	}

	return CancelIntent{OrderID: orderID, Symbol: symbol}, nil
}

func NormalizeAmend(orderID string, quantity *float64, price *float64) (AmendIntent, error) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return AmendIntent{}, fmt.Errorf("order-id is required")
	}
	intent := CancelIntent{OrderID: orderID}

	if quantity == nil && price == nil {
		return AmendIntent{}, fmt.Errorf("at least one of quantity or price must be set")
	}
	if quantity != nil && *quantity <= 0 {
		return AmendIntent{}, fmt.Errorf("quantity must be greater than zero when provided")
	}
	if price != nil && *price <= 0 {
		return AmendIntent{}, fmt.Errorf("price must be greater than zero when provided")
	}

	return AmendIntent{
		OrderID:  intent.OrderID,
		Quantity: quantity,
		Price:    price,
	}, nil
}

// validateTimeInForce enforces the official spec's combination rules. They are
// asymmetric — CLS and OPG belong to different markets — so a single "is it a
// known code" check would let through orders the ledger rejects.
//
//	DAY  every market, every order type (the default when unset)
//	CLS  US only, LIMIT only          → "LOC" (limit on close)
//	OPG  KR only, LIMIT or MARKET     → 국내 시가단일가
//
// Rejecting here rather than at the API boundary keeps the failure in the same
// place as the other order-shape rules, and means the preview a user confirms
// is already known to be well-formed.
func validateTimeInForce(intent *PlaceIntent) error {
	switch intent.TimeInForce {
	case "", "DAY":
		return nil
	case "CLS":
		if intent.Market != "us" {
			return fmt.Errorf("time-in-force CLS is only supported for US stocks")
		}
		if intent.OrderType != "limit" {
			return fmt.Errorf("time-in-force CLS requires a limit order")
		}
	case "OPG":
		if intent.Market != "kr" {
			return fmt.Errorf("time-in-force OPG is only supported for KR stocks")
		}
	default:
		return fmt.Errorf("unsupported time-in-force %q; expected DAY, CLS or OPG", intent.TimeInForce)
	}
	// 소수점 주문은 금액 기반(orderAmount) 변형으로 나가는데 그 스키마에는
	// timeInForce 자체가 없다. 조용히 무시되면 사용자가 지정한 조건이 사라진 채
	// 체결되므로 거절한다.
	if intent.Fractional {
		return fmt.Errorf("time-in-force %s is not supported for fractional orders", intent.TimeInForce)
	}
	return nil
}

func CanonicalPlace(intent PlaceIntent) string {
	fields := map[string]string{
		"currency_mode": intent.CurrencyMode,
		"fractional":    strconv.FormatBool(intent.Fractional),
		"market":        intent.Market,
		"order_type":    intent.OrderType,
		"price":         formatFloat(intent.Price),
		"amount":        formatFloat(intent.Amount),
		"quantity":      formatFloat(intent.Quantity),
		"side":          intent.Side,
		"symbol":        intent.Symbol,
	}
	// 값이 있을 때만 넣는다. 무조건 넣으면 기존에 발급된 DAY 주문 토큰이 전부
	// 무효가 된다. 비어 있을 때 생략하면 예전 토큰은 그대로 유효하고, OPG/CLS 는
	// 자기만의 토큰을 받는다 — DAY 로 미리보기해 얻은 토큰으로 OPG 를 낼 수 없다.
	if intent.TimeInForce != "" && intent.TimeInForce != "DAY" {
		fields["time_in_force"] = intent.TimeInForce
	}
	return canonicalString("place", fields)
}

func CanonicalCancel(intent CancelIntent) string {
	return canonicalString("cancel", map[string]string{
		"order_id": intent.OrderID,
		"symbol":   intent.Symbol,
	})
}

func CanonicalAmend(intent AmendIntent) string {
	fields := map[string]string{"order_id": intent.OrderID}
	if intent.Quantity != nil {
		fields["quantity"] = formatFloat(*intent.Quantity)
	}
	if intent.Price != nil {
		fields["price"] = formatFloat(*intent.Price)
	}
	return canonicalString("amend", fields)
}

func ConfirmToken(canonical string) string {
	return confirmation.Token(canonical)
}

func normalizeDefault(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = fallback
	}
	return strings.ToLower(value)
}

func normalizeCurrencyMode(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "KRW"
	}
	return strings.ToUpper(value)
}

func canonicalString(kind string, fields map[string]string) string {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys)+1)
	parts = append(parts, "kind="+kind)
	for _, key := range keys {
		parts = append(parts, key+"="+fields[key])
	}
	return strings.Join(parts, "|")
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

// decimalPlaces returns the number of fractional digits in value's shortest
// decimal representation (e.g. 0.5 → 1, 1.000001 → 6, 2 → 0).
func decimalPlaces(value float64) int {
	s := formatFloat(value)
	if i := strings.IndexByte(s, '.'); i >= 0 {
		return len(s) - i - 1
	}
	return 0
}

var krSymbolPattern = regexp.MustCompile(`^\d{6}$`)

func looksLikeKRSymbol(symbol string) bool {
	return krSymbolPattern.MatchString(symbol)
}

// InferMarketFromStockCode infers the market from a stock code pattern.
// Korean stock codes start with "A" followed by digits.
func InferMarketFromStockCode(code string) string {
	if strings.HasPrefix(code, "A") && len(code) == 7 {
		rest := code[1:]
		if krSymbolPattern.MatchString(rest) {
			return "kr"
		}
	}
	return "us"
}
