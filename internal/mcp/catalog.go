// Package mcp implements a minimal stdio Model Context Protocol server that
// fronts the official Toss Open API with a catalog-style tool surface.
//
// Rather than registering one MCP tool per API operation (which forces every
// operation's schema into the model's context permanently), the server exposes
// three fixed meta-tools — list_operations, describe_operation, call_operation —
// backed by an internal registry of operations. This keeps the always-on tool
// context small (three schemas) while still making every operation callable.
//
// The design mirrors the catalog mode of the KIS_MCP_Server reference
// (list-kis-api-specs / get-kis-api-spec / call-kis-api).
//
// Read operations are thin dispatchers over the tested official.Client typed
// methods. Write operations (order place/cancel/modify) go through the shared
// trading.Service — the same config gate, dry-run preview, and execute/confirm
// token flow the `tossctl order` CLI uses — bound to an official-only broker so
// no WTS web session is ever involved.
//
// ponytail: operations are hand-registered. When the official OpenAPI surface
// stabilises further, this registry is the natural seam to generate directly
// from the spec (see docs/migration; project goal "discovery-based dynamic
// commands").
package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/JungHoonGhae/tossinvest-cli/internal/official"
	"github.com/JungHoonGhae/tossinvest-cli/internal/trading"
)

// Deps carries the backends a handler may need. Read operations use Client;
// write (order-mutation) operations go through Trading, which applies the
// config gate, dry-run preview, and confirm-token flow — the same policy the
// `tossctl order` CLI enforces. Trading routes to an official-only broker, so
// no WTS web session is involved.
type Deps struct {
	Client  *official.Client
	Trading *trading.Service
}

// Param describes a single input parameter of an operation.
type Param struct {
	Name     string `json:"name"`
	Type     string `json:"type"` // "string" | "integer" | "boolean" | "string[]"
	Required bool   `json:"required"`
	Desc     string `json:"description,omitempty"`
}

// Operation is one callable API operation in the catalog.
type Operation struct {
	ID       string `json:"id"`
	Method   string `json:"method"`
	Path     string `json:"path"`
	Category string `json:"category"`
	Summary  string `json:"summary"`
	// Write marks state-changing operations (order place/cancel/modify). They
	// are gated by config and require an explicit execute + confirm token.
	Write  bool    `json:"write"`
	Params []Param `json:"params"`
	// handler executes the operation against the given backends.
	handler func(ctx context.Context, d *Deps, args map[string]any) (any, error)
}

// requiredNames returns the names of the operation's required parameters.
func (o Operation) requiredNames() []string {
	var out []string
	for _, p := range o.Params {
		if p.Required {
			out = append(out, p.Name)
		}
	}
	return out
}

// Catalog is the immutable registry of operations, indexed by ID.
type Catalog struct {
	ops  []Operation
	byID map[string]Operation
}

// NewCatalog builds the operation catalog over the official API (reads plus
// gated order-mutation writes).
func NewCatalog() *Catalog {
	ops := append(readOperations(), writeOperations()...)
	byID := make(map[string]Operation, len(ops))
	for _, o := range ops {
		byID[o.ID] = o
	}
	return &Catalog{ops: ops, byID: byID}
}

// List returns operations whose searchable text contains query (case-insensitive).
// An empty query returns all operations. The result is capped at limit (<=0 means
// the default of 200).
func (c *Catalog) List(query string, limit int) []Operation {
	if limit <= 0 || limit > 200 {
		limit = 200
	}
	q := strings.ToLower(strings.TrimSpace(query))
	out := make([]Operation, 0, len(c.ops))
	for _, o := range c.ops {
		if q != "" {
			hay := strings.ToLower(o.ID + " " + o.Path + " " + o.Category + " " + o.Summary)
			if !strings.Contains(hay, q) {
				continue
			}
		}
		out = append(out, o)
		if len(out) >= limit {
			break
		}
	}
	return out
}

// Get returns the operation with the given ID.
func (c *Catalog) Get(id string) (Operation, bool) {
	o, ok := c.byID[id]
	return o, ok
}

// Call validates the arguments against the operation's required parameters and
// dispatches to its handler. It returns the operation's result payload.
func (c *Catalog) Call(ctx context.Context, deps *Deps, id string, args map[string]any) (any, error) {
	op, ok := c.byID[id]
	if !ok {
		return nil, fmt.Errorf("unknown operation %q (use list_operations to discover valid ids)", id)
	}
	if args == nil {
		args = map[string]any{}
	}
	var missing []string
	for _, p := range op.Params {
		if !p.Required {
			continue
		}
		if _, ok := args[p.Name]; !ok {
			missing = append(missing, p.Name)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("operation %q is missing required parameter(s): %s", id, strings.Join(missing, ", "))
	}
	return op.handler(ctx, deps, args)
}
