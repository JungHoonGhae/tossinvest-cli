package ops

import (
	"context"
	"fmt"

	"github.com/JungHoonGhae/tossinvest-cli/internal/hiddenholding"
	"github.com/JungHoonGhae/tossinvest-cli/internal/openapiip"
	"github.com/JungHoonGhae/tossinvest-cli/internal/pricealert"
	watchlistservice "github.com/JungHoonGhae/tossinvest-cli/internal/watchlist"
)

// settingsOperations contains non-trading account-setting operations. They
// use the WTS session, but state changes still follow preview/confirm/execute.
func settingsOperations() []Operation {
	return []Operation{
		{
			ID:       "openapi_ip_list",
			Method:   "GET",
			Path:     "wts:GET /api/v1/openapi/client",
			Domain:   "system",
			Category: "settings",
			Summary:  "List IP addresses allowed to call the official Open API. Requires a WTS web session; returns no API key or secret.",
			Backend:  "wts",
			handler: func(ctx context.Context, d *Deps, _ map[string]any) (any, error) {
				if d.OpenAPIIP == nil {
					return nil, fmt.Errorf("Open API IP manager is not configured")
				}
				ips, err := d.OpenAPIIP.List(ctx)
				if err != nil {
					return nil, err
				}
				return map[string]any{"allowed_ips": ips}, nil
			},
		},
		{
			ID:       "openapi_ip_replace_current",
			Method:   "DELETE+POST",
			Path:     "wts:DELETE+POST /api/v1/openapi/client/allowed-ips",
			Domain:   "system",
			Category: "settings",
			Summary:  "Replace the official Open API allowlist with this machine's current public IP. Preview by default; execution requires execute=true and the preview confirm_token. Verifies every mutation and reconciles the previous allowlist on failure.",
			Write:    true,
			Mutation: compensatingPreferenceMutation("post-read each allowlist step; restore the previous allowlist on partial failure"),
			Backend:  "wts",
			Params: []Param{
				{Name: "execute", Type: "boolean", Desc: "false/omitted = preview; true = apply the replacement"},
				{Name: "confirm", Type: "string", Desc: "confirm_token from a fresh preview (required when execute=true)"},
			},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				if d.OpenAPIIP == nil {
					return nil, fmt.Errorf("Open API IP manager is not configured")
				}
				execute, err := argBool(args, "execute")
				if err != nil {
					return nil, err
				}
				confirm, err := argString(args, "confirm")
				if err != nil {
					return nil, err
				}
				return d.OpenAPIIP.ReplaceCurrent(ctx, openapiip.ExecuteOptions{Execute: execute, Confirm: confirm})
			},
		},
		{
			ID:       "price_alerts",
			Method:   "GET",
			Path:     "wts:GET /api/v1/user-price-alimy/{productCode}",
			Domain:   "securities",
			Category: "alerts",
			Summary:  "List target-price alerts for a Securities product.",
			Backend:  "wts",
			Params: []Param{
				{Name: "symbol", Type: "string", Required: true, Desc: "stock symbol, name, or product code"},
			},
			Probe: &ProbeSpec{
				Name: "price-alerts", Method: "GET", URL: probeAPI + "/api/v1/user-price-alimy/A005930",
				Check: func(status int, body []byte) error {
					if err := ExpectStatus(status, 200); err != nil {
						return err
					}
					return ExpectPath(body, "result", "array")
				},
			},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				if d.PriceAlerts == nil {
					return nil, fmt.Errorf("price alert manager is not configured")
				}
				symbol, _ := argString(args, "symbol")
				return d.PriceAlerts.List(ctx, symbol)
			},
		},
		priceAlertWriteOperation("price_alert_add", pricealert.ActionAdd),
		priceAlertWriteOperation("price_alert_remove", pricealert.ActionRemove),
		{
			ID:       "hidden_holdings",
			Method:   "GET",
			Path:     "wts:GET /api/v2/hidden-stocks",
			Domain:   "securities",
			Category: "portfolio",
			Summary:  "List holdings hidden from a Securities portfolio. Account identifiers are not returned.",
			Backend:  "wts",
			Params: []Param{
				{Name: "account", Type: "string", Desc: "Securities account key; primary account when omitted"},
			},
			Probe: &ProbeSpec{
				Name: "hidden-holdings", Method: "GET", URL: probeCert + "/api/v2/hidden-stocks",
				Check: func(status int, body []byte) error {
					if err := ExpectStatus(status, 200); err != nil {
						return err
					}
					return ExpectPath(body, "result.hiddenStocks", "array")
				},
			},
			handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
				if d.HiddenHoldings == nil {
					return nil, fmt.Errorf("hidden holding manager is not configured")
				}
				account, _ := argString(args, "account")
				return d.HiddenHoldings.List(ctx, account)
			},
		},
		hiddenHoldingWriteOperation("hidden_holding_hide", hiddenholding.ActionHide),
		hiddenHoldingWriteOperation("hidden_holding_show", hiddenholding.ActionShow),
		watchlistGroupWriteOperation("watchlist_group_create", watchlistservice.GroupCreate),
		watchlistGroupWriteOperation("watchlist_group_rename", watchlistservice.GroupRename),
		watchlistGroupWriteOperation("watchlist_group_delete", watchlistservice.GroupDelete),
		watchlistItemWriteOperation("watchlist_item_add", watchlistservice.ItemAdd),
		watchlistItemWriteOperation("watchlist_item_remove", watchlistservice.ItemRemove),
	}
}

func priceAlertWriteOperation(id string, action pricealert.Action) Operation {
	method := "POST"
	path := "POST /api/v1/user-price-alimy/{productCode}"
	if action == pricealert.ActionRemove {
		method = "DELETE"
		path = "DELETE /api/v1/user-price-alimy/{productCode}/{currency}/{targetPrice}"
	}
	return Operation{
		ID:        id,
		Method:    method,
		Path:      "wts:" + path,
		Domain:    "securities",
		Category:  "alerts",
		Summary:   fmt.Sprintf("Preview or %s a Securities target-price alert. Execution requires execute=true and the fresh confirm_token.", action),
		Write:     true,
		Mutation:  reversiblePreferenceMutation("post-read the exact target-price tuple"),
		Backend:   "wts",
		ProbeRefs: []string{"stock-search"},
		Params: []Param{
			{Name: "symbol", Type: "string", Required: true, Desc: "stock symbol, name, or product code"},
			{Name: "price", Type: "number", Required: true, Desc: "finite target price greater than zero"},
			{Name: "currency", Type: "string", Required: true, Desc: "KRW or USD"},
			{Name: "execute", Type: "boolean", Desc: "false/omitted = preview; true = apply"},
			{Name: "confirm", Type: "string", Desc: "confirm_token from a fresh preview"},
		},
		handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
			if d.PriceAlerts == nil {
				return nil, fmt.Errorf("price alert manager is not configured")
			}
			symbol, _ := argString(args, "symbol")
			price, err := argFloat(args, "price")
			if err != nil {
				return nil, err
			}
			currency, _ := argString(args, "currency")
			execute, _ := argBool(args, "execute")
			confirm, _ := argString(args, "confirm")
			return d.PriceAlerts.Change(ctx, action, symbol, price, currency, pricealert.ExecuteOptions{Execute: execute, Confirm: confirm})
		},
	}
}

func hiddenHoldingWriteOperation(id string, action hiddenholding.Action) Operation {
	return Operation{
		ID:        id,
		Method:    "POST",
		Path:      "wts:POST /api/v1/my-assets/hidden-stocks/" + string(action),
		Domain:    "securities",
		Category:  "portfolio",
		Summary:   fmt.Sprintf("Preview or %s a holding in the Securities portfolio. Execution requires execute=true and the fresh confirm_token.", action),
		Write:     true,
		Mutation:  reversiblePreferenceMutation("post-read the account-scoped hidden-holding set"),
		Backend:   "wts",
		ProbeRefs: []string{"stock-search"},
		Params: []Param{
			{Name: "symbol", Type: "string", Required: true, Desc: "stock symbol, name, or product code"},
			{Name: "account", Type: "string", Desc: "Securities account key; primary account when omitted"},
			{Name: "execute", Type: "boolean", Desc: "false/omitted = preview; true = apply"},
			{Name: "confirm", Type: "string", Desc: "confirm_token from a fresh preview"},
		},
		handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
			if d.HiddenHoldings == nil {
				return nil, fmt.Errorf("hidden holding manager is not configured")
			}
			symbol, _ := argString(args, "symbol")
			account, _ := argString(args, "account")
			execute, _ := argBool(args, "execute")
			confirm, _ := argString(args, "confirm")
			return d.HiddenHoldings.Change(ctx, action, symbol, account, hiddenholding.ExecuteOptions{Execute: execute, Confirm: confirm})
		},
	}
}

func watchlistGroupWriteOperation(id string, action watchlistservice.Action) Operation {
	method := "POST"
	path := "POST /api/v1/new-watchlists/groups"
	policy := reversiblePreferenceMutation("post-read the watchlist folder list and match the requested state")
	params := []Param{
		{Name: "execute", Type: "boolean", Desc: "false/omitted = preview; true = apply"},
		{Name: "confirm", Type: "string", Desc: "confirm_token from a fresh preview"},
	}
	if action != watchlistservice.GroupDelete {
		params = append([]Param{{Name: "name", Type: "string", Required: true, Desc: "new folder name"}}, params...)
	}
	if action != watchlistservice.GroupCreate {
		params = append([]Param{{Name: "group_id", Type: "integer", Required: true, Desc: "folder id from watchlist_groups"}}, params...)
	}
	if action == watchlistservice.GroupRename {
		method, path = "PATCH", "PATCH /api/v1/new-watchlists/groups/{groupId}"
	}
	if action == watchlistservice.GroupDelete {
		method, path = "DELETE", "DELETE /api/v1/new-watchlists/groups/{groupId}"
		policy = destructiveMutation("post-read absence; deleted folder identity, ordering, and membership cannot be restored exactly")
		params = append(params, Param{
			Name: "acknowledge_irreversible", Type: "boolean",
			Desc: "required with execute=true because deleting a folder cannot be undone exactly",
		})
	}
	return Operation{
		ID: id, Method: method, Path: "wts:" + path, Domain: "securities", Category: "watchlist",
		Summary: fmt.Sprintf("Preview or apply watchlist %s. Execution requires execute=true and a fresh confirm_token.%s", action, irreversibleSummary(action)),
		Write:   true, Mutation: policy, Backend: "wts", Params: params,
		ProbeRefs: []string{"watchlist-group"},
		handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
			if d.Watchlists == nil {
				return nil, fmt.Errorf("watchlist manager is not configured")
			}
			groupID, err := argInt(args, "group_id")
			if err != nil {
				return nil, err
			}
			name, _ := argString(args, "name")
			execute, _ := argBool(args, "execute")
			confirm, _ := argString(args, "confirm")
			acknowledge, _ := argBool(args, "acknowledge_irreversible")
			return d.Watchlists.ChangeGroup(ctx, action, int64(groupID), name, watchlistservice.ExecuteOptions{
				Execute: execute, Confirm: confirm, AcknowledgeIrreversible: acknowledge,
			})
		},
	}
}

func irreversibleSummary(action watchlistservice.Action) string {
	if action == watchlistservice.GroupDelete {
		return " Deletion is irreversible and additionally requires acknowledge_irreversible=true."
	}
	return ""
}

func watchlistItemWriteOperation(id string, action watchlistservice.Action) Operation {
	path := "POST /api/v1/new-watchlists/items"
	if action == watchlistservice.ItemRemove {
		path += "/remove"
	}
	return Operation{
		ID: id, Method: "POST", Path: "wts:" + path, Domain: "securities", Category: "watchlist",
		Summary: fmt.Sprintf("Preview or apply watchlist %s. Execution requires execute=true and a fresh confirm_token.", action),
		Write:   true, Mutation: reversiblePreferenceMutation("post-read the selected folder's exact product-code membership"), Backend: "wts",
		ProbeRefs: []string{"stock-search", "watchlist-group"},
		Params: []Param{
			{Name: "group_id", Type: "integer", Required: true, Desc: "folder id from watchlist_groups"},
			{Name: "symbol", Type: "string", Required: true, Desc: "stock symbol, name, or product code"},
			{Name: "execute", Type: "boolean", Desc: "false/omitted = preview; true = apply"},
			{Name: "confirm", Type: "string", Desc: "confirm_token from a fresh preview"},
		},
		handler: func(ctx context.Context, d *Deps, args map[string]any) (any, error) {
			if d.Watchlists == nil {
				return nil, fmt.Errorf("watchlist manager is not configured")
			}
			groupID, err := argInt(args, "group_id")
			if err != nil {
				return nil, err
			}
			symbol, _ := argString(args, "symbol")
			execute, _ := argBool(args, "execute")
			confirm, _ := argString(args, "confirm")
			return d.Watchlists.ChangeItem(ctx, action, int64(groupID), symbol, watchlistservice.ExecuteOptions{Execute: execute, Confirm: confirm})
		},
	}
}
