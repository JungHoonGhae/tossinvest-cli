// Package ops is the single registry of API operations — the one place where
// "this operation exists, takes these parameters, calls this typed client
// method, and is monitored by this probe" is declared.
//
// Two consumers derive from it:
//   - internal/mcp exposes the registry as an MCP catalog
//     (list_operations / describe_operation / call_operation);
//   - internal/monitor derives health probes from entries that declare one.
//
// A probe deliberately bypasses the typed client (raw method/URL/body plus a
// schema invariant) so that server-side contract changes are caught even when
// the client code is in lockstep — see the #29 regression. Probes are curated,
// not mandatory: one representative endpoint per CLI surface (Probe stays nil
// on the rest to keep the live-API cron cheap and quiet).
//
// ponytail: operations are hand-registered. When the official OpenAPI surface
// stabilises further, this registry is the natural seam to generate directly
// from the spec (see docs/migration; project goal "discovery-based dynamic
// commands").
package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/hybrid"
	"github.com/JungHoonGhae/tossinvest-cli/internal/official"
	"github.com/JungHoonGhae/tossinvest-cli/internal/trading"
)

// Deps carries the backends a handler may need. Official-only read operations
// use Client; WTS reads and "auto" reads use WTS; write (order-mutation)
// operations go through Trading, which applies the config gate, dry-run
// preview, and confirm-token flow — the same policy the `tossctl order` CLI
// enforces. Trading routes to an official-only broker, so order writes never
// touch a WTS session. Client and WTS are each optional (nil when that
// credential/session is absent); Catalog.Call checks the one an operation
// needs and returns a clear "run login" error when it is missing.
//
// WTS is the hybrid router rather than the bare web-session client, so agents
// get the same official→WTS fallback the CLI has always had (see
// internal/hybrid). With no official credentials the router degrades to a pure
// WTS passthrough, which is exactly the pre-hybrid behaviour.
type Deps struct {
	Client  *official.Client
	WTS     *hybrid.Client
	Trading *trading.Service
	Auth    AuthStatus
}

// BackendStatus reports whether a backend is connected and, if known, when its
// credential/session expires. It carries no secrets — only a boolean and a
// timestamp — so it is safe to return to an agent.
type BackendStatus struct {
	Connected bool       `json:"connected"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// AuthStatus is the read-only auth snapshot returned by the auth_status
// operation: which backends are usable and when they expire. No key/secret or
// cookie value is ever included.
type AuthStatus struct {
	WTS      BackendStatus `json:"wts"`
	Official BackendStatus `json:"official"`
}

// Param describes a single input parameter of an operation.
type Param struct {
	Name     string `json:"name"`
	Type     string `json:"type"` // "string" | "integer" | "number" | "boolean" | "string[]"
	Required bool   `json:"required"`
	Desc     string `json:"description,omitempty"`
}

// ProbeSpec declares the health probe for an operation: a raw HTTP request
// (bypassing the typed client on purpose) plus the smallest schema invariant
// that catches a contract change without false-positiving on unrelated fields.
type ProbeSpec struct {
	Name   string // probe name (stable; may differ from the operation ID)
	Method string
	URL    string
	Body   string
	Check  func(status int, body []byte) error
}

// Operation is one callable API operation in the registry.
type Operation struct {
	ID       string `json:"id"`
	Method   string `json:"method"`
	Path     string `json:"path"`
	Category string `json:"category"`
	Summary  string `json:"summary"`
	// Write marks state-changing operations (order place/cancel/modify). They
	// are gated by config and require an explicit execute + confirm token.
	Write bool `json:"write"`
	// Backend selects which authenticated client the operation needs: "" (default)
	// = the official Open API client; "wts" = the web-session client; "auto" =
	// either one, because the hybrid router serves it (tries official, falls back
	// to WTS). Catalog.Call verifies the matching client is present before
	// dispatching.
	Backend string  `json:"backend,omitempty"`
	Params  []Param `json:"params"`
	// Probe, when set, is the monitoring spec derived by internal/monitor.
	// Curated — nil on operations whose surface is already covered.
	Probe *ProbeSpec `json:"-"`
	// handler executes the operation against the given backends.
	handler func(ctx context.Context, d *Deps, args map[string]any) (any, error)
}

// Catalog is the immutable registry of operations, indexed by ID.
type Catalog struct {
	ops  []Operation
	byID map[string]Operation
}

// NewCatalog builds the operation catalog (official reads, gated order-mutation
// writes, and WTS-only reads).
func NewCatalog() *Catalog {
	ops := append(readOperations(), writeOperations()...)
	ops = append(ops, wtsOperations()...)
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

// Probes returns the probe specs declared by registry entries, in catalog order.
func (c *Catalog) Probes() []ProbeSpec {
	var out []ProbeSpec
	for _, o := range c.ops {
		if o.Probe != nil {
			out = append(out, *o.Probe)
		}
	}
	return out
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
	// Verify the operation's backend is authenticated before dispatching.
	switch op.Backend {
	case "none":
		// No auth required (e.g. auth_status) — always callable.
	case "wts":
		// WTS is the hybrid router, which is built whenever any credential is
		// present — so presence alone does not imply a web session. Auth is the
		// authoritative signal.
		if deps.WTS == nil || !deps.Auth.WTS.Connected {
			return nil, fmt.Errorf("operation %q needs a Toss web session; run `tossctl auth login`", id)
		}
	case "auto":
		// Served by the hybrid router (official first, WTS fallback), so either
		// credential is enough.
		if !deps.Auth.WTS.Connected && !deps.Auth.Official.Connected {
			return nil, fmt.Errorf("operation %q needs either official Open API credentials (`tossctl openapi login`) or a Toss web session (`tossctl auth login`)", id)
		}
		if deps.WTS == nil {
			return nil, fmt.Errorf("operation %q cannot run: no routed client was wired", id)
		}
	default: // official
		if deps.Client == nil {
			return nil, fmt.Errorf("operation %q needs official Open API credentials; run `tossctl openapi login`", id)
		}
	}
	return op.handler(ctx, deps, args)
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

// RequiredNames returns the names of the operation's required parameters.
// Exported for protocol adapters (internal/mcp) that render schemas.
func (o Operation) RequiredNames() []string { return o.requiredNames() }

// ---------------------------------------------------------------------------
// Probe check helpers — shared with internal/monitor's remaining CLI-surface
// probes. The assertions are deliberately narrow: status + one critical JSON
// path, so Toss adding unrelated fields does not trip alerts.
// ---------------------------------------------------------------------------

// ExpectStatus reports a status-code mismatch. Response bodies are not
// embedded in the error so downstream alert payloads stay bounded.
// (moved verbatim from internal/monitor.)
func ExpectStatus(got, want int) error {
	if got == want {
		return nil
	}
	return fmt.Errorf("status %d (want %d)", got, want)
}

// ExpectPath walks a dotted JSON path (a.b.c) and asserts the value's type.
// Supported types: "string", "number", "bool", "object", "array", "null".
// Array indexing not supported — for nested-array checks, use a custom Check.
// (moved verbatim from internal/monitor.)
func ExpectPath(body []byte, path, wantType string) error {
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return fmt.Errorf("decode body: %v", err)
	}
	current := v
	for _, segment := range strings.Split(path, ".") {
		// A numeric segment indexes an array. Endpoints that return a bare list
		// (crypto-prices, and others like it) otherwise can only be checked down
		// to "result is an array" — which an empty array passes, so a probe
		// couldn't tell a working call from one that matched nothing.
		if idx, err := strconv.Atoi(segment); err == nil {
			arr, ok := current.([]any)
			if !ok {
				return fmt.Errorf("path %q: expected array at %q, got %s", path, segment, jsonTypeOf(current))
			}
			if idx < 0 || idx >= len(arr) {
				return fmt.Errorf("path %q: index %d out of range (len %d)", path, idx, len(arr))
			}
			current = arr[idx]
			continue
		}
		obj, ok := current.(map[string]any)
		if !ok {
			return fmt.Errorf("path %q: expected object at %q, got %s", path, segment, jsonTypeOf(current))
		}
		next, found := obj[segment]
		if !found {
			return fmt.Errorf("path %q: key %q missing", path, segment)
		}
		current = next
	}
	got := jsonTypeOf(current)
	if got != wantType {
		return fmt.Errorf("path %q: expected %s, got %s", path, wantType, got)
	}
	return nil
}

func jsonTypeOf(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "bool"
	case float64:
		return "number"
	case string:
		return "string"
	case map[string]any:
		return "object"
	case []any:
		return "array"
	}
	return "unknown"
}
