package mcp

import (
	"context"
	"fmt"

	"github.com/JungHoonGhae/tossinvest-cli/internal/official"
	"github.com/JungHoonGhae/tossinvest-cli/internal/orderintent"
	"github.com/JungHoonGhae/tossinvest-cli/internal/trading"
)

// OfficialBroker adapts *official.Client to the trading.Broker interface using
// only the official Open API — no WTS web session. It lets the shared
// trading.Service (config gate + dry-run preview + confirm-token flow) drive
// order mutations over the official path.
//
// GetOrderAvailableActions has no official-API equivalent (the hybrid broker
// sources it from WTS), so it returns an empty set: the official cancel/modify
// endpoints validate cancellability server-side and reject if not allowed.
type OfficialBroker struct {
	Client *official.Client
}

func (b OfficialBroker) PlacePendingOrder(ctx context.Context, intent orderintent.PlaceIntent) (trading.MutationResult, error) {
	return b.Client.PlaceOrder(ctx, intent)
}

func (b OfficialBroker) CancelPendingOrder(ctx context.Context, intent orderintent.CancelIntent) (trading.MutationResult, error) {
	return b.Client.CancelOrder(ctx, intent.OrderID)
}

func (b OfficialBroker) AmendPendingOrder(ctx context.Context, intent orderintent.AmendIntent) (trading.MutationResult, error) {
	return b.Client.ModifyOrder(ctx, intent)
}

func (b OfficialBroker) GetOrderAvailableActions(_ context.Context, _ string) (map[string]any, error) {
	return map[string]any{}, nil
}

// argFloat coerces an optional numeric argument to float64.
func argFloat(args map[string]any, name string) (float64, error) {
	v, ok := args[name]
	if !ok {
		return 0, nil
	}
	f, ok := v.(float64)
	if !ok {
		return 0, fmt.Errorf("parameter %q must be a number", name)
	}
	return f, nil
}

// argFloatPtr returns a *float64 that is nil when the argument is absent,
// distinguishing "not provided" from "zero" (needed for partial amends).
func argFloatPtr(args map[string]any, name string) (*float64, error) {
	if _, ok := args[name]; !ok {
		return nil, nil
	}
	f, err := argFloat(args, name)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// executeArgs pulls the shared execute/confirm parameters common to all writes.
//
// The two-step flow mirrors `tossctl order … --execute --confirm <token>`:
//   - execute omitted/false → return a dry-run Preview (with confirm_token).
//   - execute true          → submit the mutation, requiring the confirm token
//     and the config gates (per-action + allow_live_order_actions).
func executeArgs(args map[string]any) (execute bool, confirm string, err error) {
	if execute, err = argBool(args, "execute"); err != nil {
		return false, "", err
	}
	if confirm, err = argString(args, "confirm"); err != nil {
		return false, "", err
	}
	return execute, confirm, nil
}

var executeParams = []Param{
	{Name: "execute", Type: "boolean", Desc: "false/omitted = dry-run preview; true = submit the live order"},
	{Name: "confirm", Type: "string", Desc: "confirm_token from the preview (required when execute=true)"},
}

// writeOperations returns the catalog of state-changing order operations.
// They are gated by config (trading.* toggles + allow_live_order_actions) and
// routed through the official Open API only.
func writeOperations() []Operation {
	return []Operation{
		{
			ID: "place_order", Method: "POST", Path: "/api/v1/orders",
			Category: "order", Summary: "Place a new order (US/KR, limit or US fractional market). Gated; preview unless execute=true.",
			Write: true,
			Params: append([]Param{
				{Name: "symbol", Type: "string", Required: true},
				{Name: "side", Type: "string", Required: true, Desc: `"buy" or "sell"`},
				{Name: "market", Type: "string", Desc: `"us" or "kr" (auto-routed from KR codes; default us)`},
				{Name: "order_type", Type: "string", Desc: `"limit" (default) or "market"`},
				{Name: "quantity", Type: "number", Desc: "share count (required unless fractional buy)"},
				{Name: "price", Type: "number", Desc: "limit price (required for limit orders)"},
				{Name: "amount", Type: "number", Desc: "order amount for fractional US buy"},
				{Name: "currency_mode", Type: "string", Desc: `"KRW" (default) or "USD" for US price input`},
				{Name: "fractional", Type: "boolean", Desc: "US fractional market order"},
			}, executeParams...),
			handler: placeHandler,
		},
		{
			ID: "cancel_order", Method: "POST", Path: "/api/v1/orders/{orderId}/cancel",
			Category: "order", Summary: "Cancel a pending order. Gated; preview unless execute=true.",
			Write: true,
			Params: append([]Param{
				{Name: "order_id", Type: "string", Required: true},
				{Name: "symbol", Type: "string", Required: true},
			}, executeParams...),
			handler: cancelHandler,
		},
		{
			ID: "modify_order", Method: "POST", Path: "/api/v1/orders/{orderId}/modify",
			Category: "order", Summary: "Modify a pending order's quantity/price. Gated; preview unless execute=true.",
			Write: true,
			Params: append([]Param{
				{Name: "order_id", Type: "string", Required: true},
				{Name: "quantity", Type: "number", Desc: "new quantity (optional)"},
				{Name: "price", Type: "number", Desc: "new limit price (optional; omit for market)"},
			}, executeParams...),
			handler: modifyHandler,
		},
	}
}

func placeHandler(ctx context.Context, d *Deps, args map[string]any) (any, error) {
	symbol, err := argString(args, "symbol")
	if err != nil {
		return nil, err
	}
	side, err := argString(args, "side")
	if err != nil {
		return nil, err
	}
	market, err := argString(args, "market")
	if err != nil {
		return nil, err
	}
	orderType, err := argString(args, "order_type")
	if err != nil {
		return nil, err
	}
	currencyMode, err := argString(args, "currency_mode")
	if err != nil {
		return nil, err
	}
	quantity, err := argFloat(args, "quantity")
	if err != nil {
		return nil, err
	}
	price, err := argFloat(args, "price")
	if err != nil {
		return nil, err
	}
	amount, err := argFloat(args, "amount")
	if err != nil {
		return nil, err
	}
	fractional, err := argBool(args, "fractional")
	if err != nil {
		return nil, err
	}
	intent, err := orderintent.NormalizePlace(orderintent.PlaceInput{
		Symbol:       symbol,
		Market:       market,
		Side:         side,
		OrderType:    orderType,
		Quantity:     quantity,
		Price:        price,
		Amount:       amount,
		CurrencyMode: currencyMode,
		Fractional:   fractional,
	})
	if err != nil {
		return nil, err
	}
	execute, confirm, err := executeArgs(args)
	if err != nil {
		return nil, err
	}
	if !execute {
		return d.Trading.PreviewPlace(intent), nil
	}
	return d.Trading.Place(ctx, intent, trading.ExecuteOptions{Execute: true, Confirm: confirm})
}

func cancelHandler(ctx context.Context, d *Deps, args map[string]any) (any, error) {
	orderID, err := argString(args, "order_id")
	if err != nil {
		return nil, err
	}
	symbol, err := argString(args, "symbol")
	if err != nil {
		return nil, err
	}
	intent, err := orderintent.NormalizeCancel(orderID, symbol)
	if err != nil {
		return nil, err
	}
	execute, confirm, err := executeArgs(args)
	if err != nil {
		return nil, err
	}
	if !execute {
		return d.Trading.PreviewCancel(intent), nil
	}
	return d.Trading.Cancel(ctx, intent, trading.ExecuteOptions{Execute: true, Confirm: confirm})
}

func modifyHandler(ctx context.Context, d *Deps, args map[string]any) (any, error) {
	orderID, err := argString(args, "order_id")
	if err != nil {
		return nil, err
	}
	quantity, err := argFloatPtr(args, "quantity")
	if err != nil {
		return nil, err
	}
	price, err := argFloatPtr(args, "price")
	if err != nil {
		return nil, err
	}
	intent, err := orderintent.NormalizeAmend(orderID, quantity, price)
	if err != nil {
		return nil, err
	}
	execute, confirm, err := executeArgs(args)
	if err != nil {
		return nil, err
	}
	if !execute {
		return d.Trading.PreviewAmend(intent), nil
	}
	return d.Trading.Amend(ctx, intent, trading.ExecuteOptions{Execute: true, Confirm: confirm})
}
