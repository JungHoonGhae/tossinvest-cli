package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/JungHoonGhae/tossinvest-cli/internal/hiddenholding"
	"github.com/JungHoonGhae/tossinvest-cli/internal/hybrid"
	"github.com/JungHoonGhae/tossinvest-cli/internal/jsoninput"
	"github.com/JungHoonGhae/tossinvest-cli/internal/official"
	"github.com/JungHoonGhae/tossinvest-cli/internal/openapiip"
	"github.com/JungHoonGhae/tossinvest-cli/internal/papertrading"
	"github.com/JungHoonGhae/tossinvest-cli/internal/pricealert"
	"github.com/JungHoonGhae/tossinvest-cli/internal/trading"
	watchlistservice "github.com/JungHoonGhae/tossinvest-cli/internal/watchlist"
)

// protocolVersion is the MCP protocol version this server defaults to when the
// client does not specify one during initialize.
const protocolVersion = "2025-06-18"

// Server is a minimal stdio MCP server exposing the catalog tool surface over
// an authenticated official.Client.
//
// ponytail: the MCP stdio transport is newline-delimited JSON-RPC 2.0 with a
// tiny method set (initialize, tools/list, tools/call). It is hand-rolled here
// to avoid a new dependency for three tools; swap in an MCP SDK if the surface
// grows materially.
type Server struct {
	catalog      *Catalog
	deps         *Deps
	name         string
	version      string
	instructions string
}

// Services are stateful workflows assembled by the process composition root.
// Keeping construction outside the transport makes network policy and test
// doubles explicit instead of hiding them in NewServer.
type Services struct {
	Trading        *trading.Service
	OpenAPIIP      *openapiip.Service
	PriceAlerts    *pricealert.Service
	HiddenHoldings *hiddenholding.Service
	Watchlists     *watchlistservice.Service
	Paper          *papertrading.Service
	Experiments    []string
}

// baseInstructions is returned in the initialize response so the host/model
// knows how to drive the 3-tool catalog and which auth each backend needs.
const baseInstructions = "Toss Securities via a 3-tool catalog. Call list_operations first " +
	"(optionally with a query) to find an operation id, then describe_operation for its parameter " +
	"schema, then call_operation to run it. Operations with backend \"wts\" need a Toss web session " +
	"(`tossctl auth login`); those with backend \"auto\" work with either credential (official first, " +
	"web-session fallback); the rest need official Open API credentials (`tossctl openapi login`). " +
	"Every write returns a preview. Live and preference writes require execute + confirm token. Inspect mutation policy before execution: it declares risk, " +
	"reversibility, opt-in, irreversible-acknowledgement, and verification requirements. Order writes additionally require trading config opt-in; " +
	"non-trading settings writes use the same two-step confirmation boundary, and destructive writes may require acknowledge_irreversible=true. " +
	"Operation domain describes the product area; backend describes the credential channel. A missing web UI does not make an API unavailable, " +
	"but general Banking/MyData mobile APIs are not callable through the current WTS connector because their app session and cipher envelope are not implemented."

// NewServer constructs a Server over the given backends. official serves the
// official-only Open API operations (and, via tradingSvc, gated order
// mutations); routed is the hybrid router that serves the WTS reads and the
// "auto" reads, giving agents the same official→WTS fallback the CLI has.
//
// official may be nil when its credentials are absent. routed is expected to
// be non-nil whenever any credential is present; because it is built even
// without a web session, Catalog.Call gates WTS operations on the auth
// snapshot (see SetAuthStatus) rather than on nilness. Operations whose
// backend is unavailable return a clear "run login" error. Services.Trading
// must use an OfficialBroker so order writes never touch a WTS session.
func NewServer(official *official.Client, routed *hybrid.Client, services Services, name, version string) *Server {
	instructions := baseInstructions
	if len(services.Experiments) > 0 {
		instructions += " Enabled experimental operations are labeled experimental in discovery and may change without notice. Isolated paper writes declare simulation_execute and need execute=true without a live confirmation token."
	}
	return &Server{
		catalog: NewCatalog(services.Experiments...),
		deps: &Deps{
			Client: official, WTS: routed, Trading: services.Trading,
			OpenAPIIP: services.OpenAPIIP, PriceAlerts: services.PriceAlerts,
			HiddenHoldings: services.HiddenHoldings,
			Watchlists:     services.Watchlists,
			Paper:          services.Paper,
		},
		name:         name,
		version:      version,
		instructions: instructions,
	}
}

// SetAuthStatus records the read-only auth snapshot returned by the auth_status
// operation (connected + expiry per backend; no secrets).
func (s *Server) SetAuthStatus(a AuthStatus) {
	s.deps.Auth = a
}

// AppendInstructions adds a line to the initialize `instructions` text (e.g. an
// "update available" notice). No-op for empty text.
func (s *Server) AppendInstructions(text string) {
	if text == "" {
		return
	}
	s.instructions += "\n\n" + text
}

// --- JSON-RPC 2.0 wire types ------------------------------------------------

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"` // absent => notification
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

const (
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
)

// Serve reads newline-delimited JSON-RPC messages from in and writes responses
// to out until in reaches EOF. Notifications (requests without an id) are
// handled without producing a response, per the JSON-RPC spec.
func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	r := bufio.NewReader(in)
	enc := json.NewEncoder(out) // Encode appends '\n', giving newline framing.
	for {
		line, readErr := r.ReadBytes('\n')
		if trimmed := bytes.TrimSpace(line); len(trimmed) > 0 {
			if resp, ok := s.handle(ctx, trimmed); ok {
				if err := enc.Encode(resp); err != nil {
					return err
				}
			}
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

// handle processes one raw JSON-RPC message. It returns (response, true) for
// requests and (_, false) for notifications, which take no response.
func (s *Server) handle(ctx context.Context, raw []byte) (rpcResponse, bool) {
	var req rpcRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		// Malformed frame with no recoverable id: drop it silently rather than
		// guessing an id, matching lenient MCP host behaviour.
		return rpcResponse{}, false
	}
	isNotification := len(req.ID) == 0

	result, rerr := s.dispatch(ctx, req.Method, req.Params)

	if isNotification {
		return rpcResponse{}, false
	}
	resp := rpcResponse{JSONRPC: "2.0", ID: req.ID}
	if rerr != nil {
		resp.Error = rerr
	} else {
		resp.Result = result
	}
	return resp, true
}

// dispatch routes a method to its handler, returning either a result or an error.
func (s *Server) dispatch(ctx context.Context, method string, params json.RawMessage) (any, *rpcError) {
	switch method {
	case "initialize":
		return s.handleInitialize(params), nil
	case "notifications/initialized", "notifications/cancelled":
		return nil, nil // notifications: ignored
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return s.handleToolsList(), nil
	case "tools/call":
		return s.handleToolsCall(ctx, params)
	default:
		return nil, &rpcError{Code: codeMethodNotFound, Message: "method not found: " + method}
	}
}

func (s *Server) handleInitialize(params json.RawMessage) any {
	// Echo the client's requested protocol version when present.
	version := protocolVersion
	var p struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if len(params) > 0 && json.Unmarshal(params, &p) == nil && p.ProtocolVersion != "" {
		version = p.ProtocolVersion
	}
	res := map[string]any{
		"protocolVersion": version,
		"capabilities":    map[string]any{"tools": map[string]any{}},
		"serverInfo":      map[string]any{"name": s.name, "version": s.version},
	}
	if s.instructions != "" {
		res["instructions"] = s.instructions
	}
	return res
}

// --- tools ------------------------------------------------------------------

type toolDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"inputSchema"`
}

func (s *Server) handleToolsList() any {
	obj := func(props map[string]any, required ...string) map[string]any {
		schema := map[string]any{"type": "object", "properties": props}
		if len(required) > 0 {
			schema["required"] = required
		}
		return schema
	}
	tools := []toolDef{
		{
			Name:        "list_operations",
			Description: "List available Toss operations — the official Open API plus WTS reads and safely gated writes. Each compact item shows id, product domain, execution environment, experimental gate, category, summary, write flag, backend, and required parameter names. Method, path, full parameters, and mutation policy live in describe_operation; always describe a write before calling it. Optionally filter with a case-insensitive query. Call this first to discover operation ids.",
			InputSchema: obj(map[string]any{
				"query": map[string]any{"type": "string", "description": "case-insensitive substring filter over canonical id/aliases/path/domain/category/summary"},
				"limit": map[string]any{"type": "integer", "description": "max results (default 200)"},
			}),
		},
		{
			Name:        "describe_operation",
			Description: "Get the full parameter schema for one operation id (from list_operations).",
			InputSchema: obj(map[string]any{
				"operation": map[string]any{"type": "string", "description": "operation id"},
			}, "operation"),
		},
		{
			Name:        "call_operation",
			Description: "Call a Toss Securities operation by id with its parameters. WTS operations (backend \"wts\") need a web session (`tossctl auth login`); official operations need official credentials (`tossctl openapi login`). Every write previews by default. Live-order and preference writes require execute=true plus confirm=<token>; isolated paper writes declare simulation_execute and require execute=true without a confirmation token. A paper execution never authorizes a live order. Read the operation's mutation policy first: financial writes require config opt-in and irreversible writes may additionally require acknowledge_irreversible=true. Every write declares its verification or unknown-outcome policy: WTS preference and supported paper writes re-read state, while official order transport errors require state inspection before retrying.",
			InputSchema: obj(map[string]any{
				"operation": map[string]any{"type": "string", "description": "operation id"},
				"params":    map[string]any{"type": "object", "description": "operation parameters (see describe_operation)"},
			}, "operation"),
		},
	}
	return map[string]any{"tools": tools}
}

// maxResultBytes caps the JSON text a single tools/call result may carry. Some
// reads (news_briefing, sectors) return tens of thousands of characters, which
// blows the MCP client's token budget — the client then drops the whole result,
// so the model gets nothing. Trimming to a cap beats returning nothing.
const maxResultBytes = 30000

// toolResult builds an MCP tools/call result carrying JSON-encoded text content.
//
// ponytail: the size cap lives here because every tool (list/describe/call)
// routes through this one function; per-operation limits would repeat the guard
// once per handler. Bump maxResultBytes or add per-op paging if a caller needs
// the full payload.
func toolResult(payload any, isError bool) (any, *rpcError) {
	text, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, &rpcError{Code: codeInvalidParams, Message: "encoding result: " + err.Error()}
	}
	if len(text) > maxResultBytes {
		var v any
		if jsoninput.Decode(text, &v) == nil {
			if trimmed, terr := json.MarshalIndent(shrinkToCap(v), "", "  "); terr == nil {
				text = trimmed
			}
		}
	}
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": string(text)}},
		"isError": isError,
	}, nil
}

// --- result size cap ---------------------------------------------------------

const (
	// omittedKey marks the placeholder element shrinkToCap appends to a trimmed
	// array, so a truncated list can never be mistaken for a complete one.
	omittedKey = "_omitted_items"
	// omittedResultKey marks a payload that cannot be reduced by dropping array
	// rows (for example one oversized scalar string).
	omittedResultKey = "_omitted_result"
)

// omittedItems is a private in-memory marker so an upstream response whose
// last row happens to contain an `_omitted_items` field is never mistaken for
// a marker created by this transport. It still marshals to the documented JSON
// shape.
type omittedItems struct {
	Count int    `json:"_omitted_items"`
	Note  string `json:"_note"`
}

// shrinkToCap repeatedly trims the largest array in a decoded JSON value until
// the re-encoded value fits maxResultBytes, appending an explicit placeholder to
// every array it trims. When no array can be reduced, it replaces the payload
// with an explicit omission object so the transport limit remains an invariant.
func shrinkToCap(v any) any {
	holder := map[string]any{"v": v} // gives the root a setter
	for {
		text, err := json.MarshalIndent(holder["v"], "", "  ")
		if err != nil || len(text) <= maxResultBytes {
			return holder["v"]
		}
		var (
			best     []any
			bestSet  func(any)
			bestSize int
		)
		walkSlices(holder, nil, func(s []any, set func(any)) {
			if keepable(s) == 0 {
				return
			}
			enc, err := json.MarshalIndent(s, "", "  ")
			if err != nil || len(enc) <= bestSize {
				return
			}
			best, bestSet, bestSize = s, set, len(enc)
		})
		if bestSet == nil {
			return map[string]any{
				omittedResultKey:  true,
				"_original_bytes": len(text),
				"_note": fmt.Sprintf(
					"result omitted: payload exceeded the %d-byte MCP limit and contained no array rows that could be trimmed. Narrow the request or use the CLI for the full result.",
					maxResultBytes,
				),
			}
		}
		bestSet(trim(best, bestSize, len(text)-maxResultBytes))
	}
}

// keepable reports how many real (non-placeholder) elements an array holds.
func keepable(s []any) int {
	if n := len(s); n > 0 {
		if _, ok := s[n-1].(omittedItems); ok {
			return n - 1
		}
	}
	return len(s)
}

// trim drops elements from the tail of s — roughly enough to shed `excess` bytes
// given the array encodes to `size` bytes — and records the total number dropped
// so far in a trailing placeholder. It always drops at least one element, so
// shrinkToCap's loop terminates; overshooting the cap only costs another pass.
func trim(s []any, size, excess int) []any {
	n, dropped := keepable(s), 0
	if n < len(s) {
		dropped = s[len(s)-1].(omittedItems).Count
	}
	// Estimate how many average-sized rows must be removed. This avoids the
	// overflowing n*(size-excess) calculation flagged by CodeQL while remaining
	// deliberately conservative; shrinkToCap simply takes another pass if the
	// placeholder overhead leaves the result above the cap.
	bytesPerItem := 1
	if n > 0 && size > n {
		bytesPerItem = size / n
	}
	drop := 1
	if excess > 0 {
		drop = excess / bytesPerItem
		if excess%bytesPerItem != 0 {
			drop++
		}
		if drop < 1 {
			drop = 1
		}
	}
	if drop > n {
		drop = n
	}
	keep := n - drop
	dropped += drop
	// drop is always at least one, so len(s) has room for the kept rows plus
	// the omission marker. Using the existing length as capacity avoids an
	// attacker-controlled addition in the allocation-size expression.
	out := make([]any, 0, len(s))
	out = append(out, s[:keep]...)
	return append(out, omittedItems{
		Count: dropped,
		Note:  fmt.Sprintf("%d items omitted: result exceeded the %d-byte MCP limit. Narrow the request (see describe_operation params) or use the CLI for the full list.", dropped, maxResultBytes),
	})
}

// walkSlices visits every array in a decoded JSON value, passing a setter that
// replaces it in its parent.
func walkSlices(v any, set func(any), visit func([]any, func(any))) {
	switch t := v.(type) {
	case []any:
		if set != nil {
			visit(t, set)
		}
		for i := range t {
			walkSlices(t[i], func(nv any) { t[i] = nv }, visit)
		}
	case map[string]any:
		for k := range t {
			k := k
			walkSlices(t[k], func(nv any) { t[k] = nv }, visit)
		}
	}
}

// toolError builds an isError result with a plain message so the model sees it.
func toolError(format string, a ...any) (any, *rpcError) {
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": fmt.Sprintf(format, a...)}},
		"isError": true,
	}, nil
}

func (s *Server) handleToolsCall(ctx context.Context, params json.RawMessage) (any, *rpcError) {
	var call struct {
		Name      string `json:"name"`
		Arguments struct {
			Query     json.RawMessage `json:"query"`
			Limit     json.RawMessage `json:"limit"`
			Operation string          `json:"operation"`
			Params    json.RawMessage `json:"params"`
		} `json:"arguments"`
	}
	if len(params) > 0 {
		if err := jsoninput.Decode(params, &call); err != nil {
			return nil, &rpcError{Code: codeInvalidParams, Message: "invalid tools/call params: " + err.Error()}
		}
	}
	switch call.Name {
	case "list_operations":
		query := ""
		if len(call.Arguments.Query) > 0 {
			var decoded any
			if err := jsoninput.Decode(call.Arguments.Query, &decoded); err != nil {
				return nil, &rpcError{Code: codeInvalidParams, Message: "list_operations 'query' must be a string: " + err.Error()}
			}
			value, ok := decoded.(string)
			if !ok {
				return nil, &rpcError{Code: codeInvalidParams, Message: "list_operations 'query' must be a string"}
			}
			query = value
		}
		limit := 0
		if len(call.Arguments.Limit) > 0 {
			var decoded any
			if err := jsoninput.Decode(call.Arguments.Limit, &decoded); err != nil {
				return nil, &rpcError{Code: codeInvalidParams, Message: "list_operations 'limit' must be an integer: " + err.Error()}
			}
			number, ok := decoded.(json.Number)
			if !ok {
				return nil, &rpcError{Code: codeInvalidParams, Message: "list_operations 'limit' must be an integer"}
			}
			value, err := jsoninput.Int(number, strconv.IntSize)
			if err != nil {
				return nil, &rpcError{Code: codeInvalidParams, Message: "list_operations 'limit' must be an integer: " + err.Error()}
			}
			limit = int(value)
		}
		return compactToolResult(s.listOperationsPayload(query, limit), false)
	case "describe_operation":
		id := call.Arguments.Operation
		if id == "" {
			return toolError("describe_operation requires the 'operation' parameter")
		}
		op, ok := s.catalog.Get(id)
		if !ok {
			return toolError("unknown operation %q (use list_operations)", id)
		}
		return toolResult(op, false)
	case "call_operation":
		id := call.Arguments.Operation
		if id == "" {
			return toolError("call_operation requires the 'operation' parameter")
		}
		var opArgs map[string]any
		if len(call.Arguments.Params) > 0 {
			if err := jsoninput.Decode(call.Arguments.Params, &opArgs); err != nil {
				return toolError("call_operation 'params' must be a JSON object: %s", err.Error())
			}
			if opArgs == nil {
				return toolError("call_operation 'params' must be a JSON object: got null")
			}
		}
		result, err := s.catalog.Call(ctx, s.deps, id, opArgs)
		if err != nil {
			return toolError("%s", err.Error())
		}
		return toolResult(result, false)
	default:
		return nil, &rpcError{Code: codeMethodNotFound, Message: "unknown tool: " + call.Name}
	}
}

func (s *Server) listOperationsPayload(query string, limit int) any {
	items := s.catalog.ListItems(query, limit)
	return map[string]any{"count": len(items), "operations": items}
}

// compactToolResult keeps the discovery index below the MCP result cap without
// removing routing summaries. Human-facing `tossctl ops list` remains indented;
// only the model-consumed MCP index uses compact JSON.
func compactToolResult(payload any, isError bool) (any, *rpcError) {
	text, err := json.Marshal(payload)
	if err != nil {
		return nil, &rpcError{Code: codeInvalidParams, Message: "encoding result: " + err.Error()}
	}
	if len(text) > maxResultBytes {
		var v any
		if jsoninput.Decode(text, &v) == nil {
			if trimmed, terr := json.Marshal(shrinkToCap(v)); terr == nil {
				text = trimmed
			}
		}
	}
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": string(text)}},
		"isError": isError,
	}, nil
}
