package ops

import (
	"context"
	"fmt"

	"github.com/JungHoonGhae/tossinvest-cli/internal/openapiip"
)

// settingsOperations contains non-trading account-setting operations. They
// use the WTS session, but state changes still follow preview/confirm/execute.
func settingsOperations() []Operation {
	return []Operation{
		{
			ID:       "openapi_ip_list",
			Method:   "GET",
			Path:     "wts:GET /api/v1/openapi/client",
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
			Category: "settings",
			Summary:  "Replace the official Open API allowlist with this machine's current public IP. Preview by default; execution requires execute=true and the preview confirm_token. Verifies every mutation and reconciles the previous allowlist on failure.",
			Write:    true,
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
	}
}
