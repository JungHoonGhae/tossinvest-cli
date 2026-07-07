package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/JungHoonGhae/tossinvest-cli/internal/official"
	"github.com/JungHoonGhae/tossinvest-cli/internal/trading"
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
	catalog *Catalog
	deps    *Deps
	name    string
	version string
}

// NewServer constructs a Server over the given authenticated client and trading
// service. tradingSvc drives gated order-mutation operations; pass one built on
// an OfficialBroker so writes never touch a WTS session.
func NewServer(client *official.Client, tradingSvc *trading.Service, name, version string) *Server {
	return &Server{
		catalog: NewCatalog(),
		deps:    &Deps{Client: client, Trading: tradingSvc},
		name:    name,
		version: version,
	}
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
	return map[string]any{
		"protocolVersion": version,
		"capabilities":    map[string]any{"tools": map[string]any{}},
		"serverInfo":      map[string]any{"name": s.name, "version": s.version},
	}
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
			Description: "List available Toss official API operations (id, method, path, summary). Optionally filter with a case-insensitive query. Call this first to discover operation ids.",
			InputSchema: obj(map[string]any{
				"query": map[string]any{"type": "string", "description": "case-insensitive substring filter over id/path/category/summary"},
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
			Description: "Call a Toss official API operation by id with its parameters. Reads return the JSON payload. Write operations (place/cancel/modify order) are gated: without execute=true they return a dry-run preview with a confirm_token; pass execute=true plus confirm=<token> to submit (also requires config to enable trading).",
			InputSchema: obj(map[string]any{
				"operation": map[string]any{"type": "string", "description": "operation id"},
				"params":    map[string]any{"type": "object", "description": "operation parameters (see describe_operation)"},
			}, "operation"),
		},
	}
	return map[string]any{"tools": tools}
}

// toolResult builds an MCP tools/call result carrying JSON-encoded text content.
func toolResult(payload any, isError bool) (any, *rpcError) {
	text, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, &rpcError{Code: codeInvalidParams, Message: "encoding result: " + err.Error()}
	}
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": string(text)}},
		"isError": isError,
	}, nil
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
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if len(params) > 0 {
		if err := json.Unmarshal(params, &call); err != nil {
			return nil, &rpcError{Code: codeInvalidParams, Message: "invalid tools/call params: " + err.Error()}
		}
	}
	switch call.Name {
	case "list_operations":
		query, _ := call.Arguments["query"].(string)
		limit := 0
		if v, ok := call.Arguments["limit"].(float64); ok {
			limit = int(v)
		}
		return toolResult(s.listOperationsPayload(query, limit), false)
	case "describe_operation":
		id, _ := call.Arguments["operation"].(string)
		if id == "" {
			return toolError("describe_operation requires the 'operation' parameter")
		}
		op, ok := s.catalog.Get(id)
		if !ok {
			return toolError("unknown operation %q (use list_operations)", id)
		}
		return toolResult(op, false)
	case "call_operation":
		id, _ := call.Arguments["operation"].(string)
		if id == "" {
			return toolError("call_operation requires the 'operation' parameter")
		}
		opArgs, _ := call.Arguments["params"].(map[string]any)
		result, err := s.catalog.Call(ctx, s.deps, id, opArgs)
		if err != nil {
			return toolError("%s", err.Error())
		}
		return toolResult(result, false)
	default:
		return nil, &rpcError{Code: codeMethodNotFound, Message: "unknown tool: " + call.Name}
	}
}

// listItem is the compact per-operation shape returned by list_operations.
type listItem struct {
	ID       string   `json:"id"`
	Method   string   `json:"method"`
	Path     string   `json:"path"`
	Category string   `json:"category"`
	Summary  string   `json:"summary"`
	Write    bool     `json:"write,omitempty"`
	Required []string `json:"required,omitempty"`
}

func (s *Server) listOperationsPayload(query string, limit int) any {
	ops := s.catalog.List(query, limit)
	items := make([]listItem, 0, len(ops))
	for _, o := range ops {
		items = append(items, listItem{
			ID: o.ID, Method: o.Method, Path: o.Path,
			Category: o.Category, Summary: o.Summary, Write: o.Write, Required: o.requiredNames(),
		})
	}
	return map[string]any{"count": len(items), "operations": items}
}
