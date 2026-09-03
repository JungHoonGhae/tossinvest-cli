package ops

import (
	"context"
	"fmt"

	"github.com/JungHoonGhae/tossinvest-cli/internal/featuregate"
	"github.com/JungHoonGhae/tossinvest-cli/internal/orderintent"
	"github.com/JungHoonGhae/tossinvest-cli/internal/papertrading"
)

const paperCertAPI = "https://wts-cert-api.tossinvest.com"

var paperExecuteParams = []Param{
	{Name: "execute", Type: "boolean", Desc: "false/omitted = server-validated simulation preview; true = apply only to the isolated paper ledger"},
}

// paperOperations is the machine surface for the dedicated /paper/ ledger.
// These operations never call the live-order service and intentionally use a
// different authorization mode from financial mutations.
func paperOperations() []Operation {
	operations := []Operation{
		{
			ID: "get_paper_trading_status", Method: "GET", Path: "wts:/api/v1/paper/cash-balance + /api/v1/paper/education/summary",
			Domain: "securities", Environment: "paper", Category: "paper-trading", Backend: "wts",
			Summary: "Read simulated US-options cash and server-side eligibility/progress flags.",
			Probe: &ProbeSpec{
				Name: "paper-cash-balance", Method: "GET", URL: paperCertAPI + "/api/v1/paper/cash-balance",
				Check: paperStatusAndPath("result.orderableAmount", "number"),
			},
			ExtraProbes: []ProbeSpec{{
				Name: "paper-education-summary", Method: "GET", URL: paperCertAPI + "/api/v1/paper/education/summary",
				Check: paperStatusAndPath("result.allCompleted", "bool"),
			}},
			handler: func(ctx context.Context, d *Deps, _ map[string]any) (any, error) {
				if err := requirePaperService(d); err != nil {
					return nil, err
				}
				return d.Paper.Status(ctx)
			},
		},
		{
			ID: "list_pending_paper_orders", Method: "GET", Path: "wts:/api/v1/paper/trading/orders/histories/all/pending",
			Domain: "securities", Environment: "paper", Category: "paper-trading", Backend: "wts",
			Summary: "List pending simulated US-options orders.",
			Probe: &ProbeSpec{
				Name: "paper-pending-orders", Method: "GET", URL: paperCertAPI + "/api/v1/paper/trading/orders/histories/all/pending",
				Check: paperStatusAndPath("result", "array"),
			},
			handler: func(ctx context.Context, d *Deps, _ map[string]any) (any, error) {
				if err := requirePaperService(d); err != nil {
					return nil, err
				}
				return d.Paper.PendingOrders(ctx)
			},
		},
		{
			ID: "list_completed_paper_orders", Method: "GET", Path: "wts:/api/v2/paper/trading/my-orders/markets/us-opt/by-date/completed",
			Domain: "securities", Environment: "paper", Category: "paper-trading", Backend: "wts",
			Summary: "List completed or cancelled simulated US-options orders.",
			Probe: &ProbeSpec{
				Name: "paper-completed-orders", Method: "GET", URL: paperCertAPI + "/api/v2/paper/trading/my-orders/markets/us-opt/by-date/completed",
				Check: paperStatusAndPath("result.body", "array"),
			},
			handler: func(ctx context.Context, d *Deps, _ map[string]any) (any, error) {
				if err := requirePaperService(d); err != nil {
					return nil, err
				}
				return d.Paper.CompletedOrders(ctx)
			},
		},
		{
			ID: "initialize_paper_trading", Method: "POST", Path: "wts:/api/v1/paper/init",
			Domain: "securities", Environment: "paper", Category: "paper-trading", Backend: "wts",
			Summary: "Initialize/apply for the server's paper-options environment; preview unless execute=true.", Write: true,
			Mutation: simulationMutation(MutationUnknown, "re-read paper status; current server may return an opaque 500 when rollout initialization is unavailable"),
			Params:   paperExecuteParams,
			handler:  paperInitializeHandler,
		},
		{
			ID: "deposit_paper_cash", Method: "POST", Path: "wts:/api/v1/paper/deposit",
			Domain: "securities", Environment: "paper", Category: "paper-trading", Backend: "wts",
			Summary: "Add simulated cash to the isolated paper ledger; preview unless execute=true.", Write: true,
			Mutation: simulationMutation(MutationUnknown, "automatically re-read paper cash balance after execution; the observed API exposes no matching withdrawal endpoint"),
			Params:   append([]Param{{Name: "amount", Type: "integer", Required: true, Desc: "positive whole simulated-cash amount"}}, paperExecuteParams...),
			handler:  paperDepositHandler,
		},
		{
			ID: "place_paper_order", Method: "POST", Path: "wts:/api/v2/paper/trading/order/prepare -> /api/v2/paper/trading/order/create",
			Domain: "securities", Environment: "paper", Category: "paper-trading", Backend: "wts",
			Summary: "Preview or place an isolated simulated US-options order; never reaches a live account.", Write: true,
			Mutation: simulationMutation(MutationIrreversible, "inspect pending and completed paper orders before retrying after an unknown transport outcome"),
			Params: append([]Param{
				{Name: "stock_code", Type: "string", Required: true, Desc: "OPT_ contract code from quote options"},
				{Name: "market", Type: "string", Desc: "exchange code override; normally resolved from the contract"},
				{Name: "currency_mode", Type: "string", Desc: "USD (default) or KRW"},
				{Name: "side", Type: "string", Required: true, Desc: "buy or sell"},
				{Name: "order_type", Type: "string", Desc: "limit (default) or market"},
				{Name: "price", Type: "number", Desc: "required and positive for limit orders"},
				{Name: "quantity", Type: "integer", Required: true, Desc: "positive whole contract count"},
			}, paperExecuteParams...),
			handler: paperPlaceHandler,
		},
		{
			ID: "cancel_paper_order", Method: "POST", Path: "wts:/api/v2/paper/trading/order/cancel/prepare/{date}/{orderNo} -> /api/v3/paper/trading/order/cancel/{date}/{orderNo}",
			Domain: "securities", Environment: "paper", Category: "paper-trading", Backend: "wts",
			Summary: "Preview or cancel one pending simulated order.", Write: true,
			Mutation: simulationMutation(MutationIrreversible, "automatically verify absence from pending orders after execution; inspect completed orders before retrying an unknown outcome"),
			Params:   append([]Param{{Name: "order_id", Type: "string", Required: true}}, paperExecuteParams...),
			handler:  paperCancelHandler,
		},
		{
			ID: "cancel_all_paper_orders", Method: "POST", Path: "wts:/api/v3/paper/trading/order/bulk-cancel/prepare -> /api/v3/paper/trading/order/bulk-cancel",
			Domain: "securities", Environment: "paper", Category: "paper-trading", Backend: "wts",
			Summary: "Preview or cancel all matching pending simulated orders.", Write: true,
			Mutation: simulationMutation(MutationIrreversible, "automatically compare remaining pending orders with failed_count after execution"),
			Params:   append([]Param{{Name: "side", Type: "string", Desc: "optional buy or sell filter"}}, paperExecuteParams...),
			handler:  paperBulkCancelHandler,
		},
	}
	for i := range operations {
		operations[i].Experimental = featuregate.PaperTrading
	}
	return operations
}

func requirePaperService(d *Deps) error {
	if d == nil || d.Paper == nil {
		return fmt.Errorf("paper trading service is not configured")
	}
	return nil
}

func paperInitializeHandler(ctx context.Context, d *Deps, args map[string]any) (any, error) {
	if err := requirePaperService(d); err != nil {
		return nil, err
	}
	execute, err := argBool(args, "execute")
	if err != nil {
		return nil, err
	}
	return d.Paper.Initialize(ctx, papertrading.ExecuteOptions{Execute: execute})
}

func paperDepositHandler(ctx context.Context, d *Deps, args map[string]any) (any, error) {
	if err := requirePaperService(d); err != nil {
		return nil, err
	}
	amount, err := argInt(args, "amount")
	if err != nil {
		return nil, err
	}
	execute, err := argBool(args, "execute")
	if err != nil {
		return nil, err
	}
	return d.Paper.Deposit(ctx, int64(amount), papertrading.ExecuteOptions{Execute: execute})
}

func paperPlaceHandler(ctx context.Context, d *Deps, args map[string]any) (any, error) {
	if err := requirePaperService(d); err != nil {
		return nil, err
	}
	stockCode, err := argString(args, "stock_code")
	if err != nil {
		return nil, err
	}
	market, err := argString(args, "market")
	if err != nil {
		return nil, err
	}
	currency, err := argString(args, "currency_mode")
	if err != nil {
		return nil, err
	}
	side, err := argString(args, "side")
	if err != nil {
		return nil, err
	}
	orderType, err := argString(args, "order_type")
	if err != nil {
		return nil, err
	}
	price, err := argFloat(args, "price")
	if err != nil {
		return nil, err
	}
	quantity, err := argInt(args, "quantity")
	if err != nil {
		return nil, err
	}
	execute, err := argBool(args, "execute")
	if err != nil {
		return nil, err
	}
	return d.Paper.Place(ctx, orderintent.OptionPlaceIntent{
		Symbol: stockCode, Exchange: market, CurrencyMode: currency,
		Side: side, OrderType: orderType, Price: price, Quantity: quantity,
	}, papertrading.ExecuteOptions{Execute: execute})
}

func paperCancelHandler(ctx context.Context, d *Deps, args map[string]any) (any, error) {
	if err := requirePaperService(d); err != nil {
		return nil, err
	}
	orderID, err := argString(args, "order_id")
	if err != nil {
		return nil, err
	}
	execute, err := argBool(args, "execute")
	if err != nil {
		return nil, err
	}
	return d.Paper.Cancel(ctx, orderID, papertrading.ExecuteOptions{Execute: execute})
}

func paperBulkCancelHandler(ctx context.Context, d *Deps, args map[string]any) (any, error) {
	if err := requirePaperService(d); err != nil {
		return nil, err
	}
	side, err := argString(args, "side")
	if err != nil {
		return nil, err
	}
	execute, err := argBool(args, "execute")
	if err != nil {
		return nil, err
	}
	return d.Paper.BulkCancel(ctx, side, papertrading.ExecuteOptions{Execute: execute})
}

func paperStatusAndPath(path, kind string) func(int, []byte) error {
	return func(status int, body []byte) error {
		if err := ExpectStatus(status, 200); err != nil {
			return err
		}
		return ExpectPath(body, path, kind)
	}
}
