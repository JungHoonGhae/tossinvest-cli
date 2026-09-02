package ops

import (
	"context"
	"io"
	"testing"

	tossclient "github.com/JungHoonGhae/tossinvest-cli/internal/client"
	"github.com/JungHoonGhae/tossinvest-cli/internal/hybrid"
	"github.com/JungHoonGhae/tossinvest-cli/internal/openapiip"
	"github.com/JungHoonGhae/tossinvest-cli/internal/routing"
)

type opsAllowlistClient struct {
	ips []string
}

func (c *opsAllowlistClient) OpenAPIAllowedIPs(context.Context) ([]string, error) {
	return append([]string(nil), c.ips...), nil
}

func (c *opsAllowlistClient) AddOpenAPIAllowedIP(_ context.Context, ip string) error {
	c.ips = append(c.ips, ip)
	return nil
}

func (c *opsAllowlistClient) DeleteOpenAPIAllowedIP(_ context.Context, ip string) error {
	for i, existing := range c.ips {
		if existing == ip {
			c.ips = append(c.ips[:i], c.ips[i+1:]...)
			break
		}
	}
	return nil
}

type opsIPResolver string

func (r opsIPResolver) CurrentPublicIP(context.Context) (string, error) { return string(r), nil }

func TestOpenAPIIPOperationsExposeSafeMCPFlow(t *testing.T) {
	t.Parallel()
	catalog := NewCatalog()
	op, ok := catalog.Get("openapi_ip_replace_current")
	if !ok {
		t.Fatal("openapi_ip_replace_current operation missing")
	}
	if !op.Write || op.Backend != "wts" {
		t.Fatalf("unexpected operation metadata: %#v", op)
	}

	allowlist := &opsAllowlistClient{ips: []string{"203.0.113.7"}}
	manager := openapiip.NewService(
		allowlist,
		opsIPResolver("198.51.100.9"),
	)
	routed := hybrid.New(tossclient.New(tossclient.Config{}), nil, hybrid.Policy{Prefer: routing.WTS}, io.Discard)
	deps := &Deps{
		WTS:       routed,
		OpenAPIIP: manager,
		Auth:      AuthStatus{WTS: BackendStatus{Connected: true}},
	}

	result, err := catalog.Call(context.Background(), deps, op.ID, nil)
	if err != nil {
		t.Fatalf("preview call: %v", err)
	}
	plan, ok := result.(openapiip.Plan)
	if !ok || plan.Applied || plan.ConfirmToken == "" {
		t.Fatalf("expected preview with confirm token, got %#v", result)
	}

	allowlist.ips = []string{"192.0.2.44"}
	if _, err := catalog.Call(context.Background(), deps, op.ID, map[string]any{"execute": true, "confirm": plan.ConfirmToken}); err == nil {
		t.Fatal("stale confirmation token must be rejected")
	}
	if len(allowlist.ips) != 1 || allowlist.ips[0] != "192.0.2.44" {
		t.Fatalf("stale token mutated allowlist: %v", allowlist.ips)
	}

	result, err = catalog.Call(context.Background(), deps, op.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	fresh := result.(openapiip.Plan)
	result, err = catalog.Call(context.Background(), deps, op.ID, map[string]any{"execute": true, "confirm": fresh.ConfirmToken})
	if err != nil {
		t.Fatal(err)
	}
	applied := result.(openapiip.Plan)
	if !applied.Applied || len(allowlist.ips) != 1 || allowlist.ips[0] != "198.51.100.9" {
		t.Fatalf("fresh execution = %#v, allowlist = %v", applied, allowlist.ips)
	}
}
