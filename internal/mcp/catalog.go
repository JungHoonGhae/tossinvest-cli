// Package mcp implements a minimal stdio Model Context Protocol server that
// fronts the operation registry (internal/ops) with a catalog-style tool
// surface.
//
// Rather than registering one MCP tool per API operation (which forces every
// operation's schema into the model's context permanently), the server exposes
// three fixed meta-tools — list_operations, describe_operation, call_operation —
// backed by the shared registry. This keeps the always-on tool context small
// (three schemas) while still making every operation callable.
//
// The design mirrors the catalog mode of the KIS_MCP_Server reference
// (list-kis-api-specs / get-kis-api-spec / call-kis-api).
//
// The operations themselves — what exists, parameters, typed handlers, and
// their monitoring probes — live in internal/ops. This package is only the
// MCP protocol adapter over that registry; internal/monitor is the other
// consumer.
package mcp

import "github.com/JungHoonGhae/tossinvest-cli/internal/ops"

// Re-exported registry types — the MCP server and its callers (cmd/tossctl)
// use these names; the definitions moved to internal/ops so the registry can
// feed both the MCP catalog and the monitor probes.
type (
	Deps           = ops.Deps
	AuthStatus     = ops.AuthStatus
	BackendStatus  = ops.BackendStatus
	Param          = ops.Param
	Operation      = ops.Operation
	MutationPolicy = ops.MutationPolicy
	Catalog        = ops.Catalog
	OfficialBroker = ops.OfficialBroker
)

// NewCatalog builds the shared operation catalog.
func NewCatalog() *Catalog { return ops.NewCatalog() }
