package main

import (
	"fmt"

	"github.com/JungHoonGhae/tossinvest-cli/internal/hiddenholding"
	"github.com/JungHoonGhae/tossinvest-cli/internal/jsoninput"
	"github.com/JungHoonGhae/tossinvest-cli/internal/openapiip"
	"github.com/JungHoonGhae/tossinvest-cli/internal/ops"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/JungHoonGhae/tossinvest-cli/internal/pricealert"
	"github.com/JungHoonGhae/tossinvest-cli/internal/trading"
	watchlistservice "github.com/JungHoonGhae/tossinvest-cli/internal/watchlist"
	"github.com/spf13/cobra"
)

// newOpsCmd builds `tossctl ops`: the operation registry (internal/ops) as a
// terminal surface — the registry's third consumer after the MCP server and the
// monitor probes.
//
// It exists because the typed commands cannot enumerate themselves. An agent
// driving tossctl can read `--help`, but nothing tells it that 100+ operations
// exist, what each takes, or how to call one it has never seen. The MCP server
// answers exactly that, and agents increasingly prefer a CLI to an MCP server
// (see docs/research/2026-07-25-cli-mcp-single-declaration.md), so the same
// three verbs are worth having here.
//
// Deliberately *not* a second set of typed commands: `ops` is isomorphic to the
// MCP tools (same ids, same JSON params, same JSON out), so anything learned on
// one surface transfers to the other. ADR 0001's argument for typed, masked,
// human-shaped commands still governs `tossctl account`, `order`, and friends —
// this is the machine door, and it is the only one.
func newOpsCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ops",
		Short: "Discover and call API operations directly (machine surface)",
		Long: "Browse and invoke the operation registry that backs the MCP server. " +
			"`ops list` finds an operation, `ops describe` shows its parameters, and " +
			"`ops call` runs it. Output is always JSON, never a table.\n\n" +
			"Errors follow shell convention rather than the MCP one: stdout carries JSON " +
			"only on success, and a failure prints a plain message to stderr and exits " +
			"non-zero. Check the exit status, do not parse stdout for an error object.\n\n" +
			"For agents and scripts. Humans want the typed commands (`tossctl account`, " +
			"`tossctl order`, ...), which format for reading and mask account numbers.",
	}
	cmd.AddCommand(newOpsListCmd(), newOpsDescribeCmd(), newOpsCallCmd(opts))
	return cmd
}

func newOpsListCmd() *cobra.Command {
	var query string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available API operations, optionally filtered",
		Long: "List the operations in the registry as JSON. Filter with --query, which " +
			"matches the canonical id, compatibility aliases, path, product domain, category, and summary.\n\n" +
			"The list is a compact discovery index. Run `ops describe <id>` for the HTTP method/path, " +
			"full parameter schema, and mutation policy before calling an operation.\n\n" +
			"Needs no credentials: the catalog is a local declaration, not an API call. " +
			"Operations you cannot yet run are listed too — `backend` tells you which " +
			"login each one needs.",
		Annotations:  map[string]string{"source": "local"},
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// No --limit: the whole catalog is under Catalog.List's 200-item
			// cap, so a limit flag could not currently bind.
			items := ops.NewCatalog().ListItems(query, 0)
			return output.WriteJSON(cmd.OutOrStdout(), map[string]any{
				"count": len(items), "operations": items,
			})
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Filter by id, alias, path, product domain, category, or summary")
	return cmd
}

func newOpsDescribeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "describe <operation>",
		Short: "Show one operation's parameter schema",
		Long: "Print an operation's full declaration as JSON: method, path, backend, " +
			"whether it writes, its mutation risk and approval policy, and every parameter with its type and whether it is " +
			"required. This is what you need to build the --params object for `ops call`.\n\n" +
			"Needs no credentials.",
		Annotations:  map[string]string{"source": "local"},
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			op, ok := ops.NewCatalog().Get(args[0])
			if !ok {
				return fmt.Errorf("unknown operation %q; run `tossctl ops list` to see the available ids", args[0])
			}
			return output.WriteJSON(cmd.OutOrStdout(), op)
		},
	}
}

func newOpsCallCmd(opts *rootOptions) *cobra.Command {
	var params string
	cmd := &cobra.Command{
		Use:   "call <operation>",
		Short: "Call an operation with JSON parameters",
		Long: "Run one operation and print its result as JSON. Parameters go in a single " +
			"JSON object: `--params '{\"scope\":\"watchlist\"}'`. Use `ops describe` to see " +
			"what an operation accepts.\n\n" +
			"Output is raw — account numbers and real names appear unmasked, unlike the " +
			"typed commands. Do not paste it into an issue or a chat without checking it.\n\n" +
			"Every write defaults to a dry-run preview carrying a confirm token. Execution " +
			"requires execute + that preview token in --params. Inspect `ops describe` for " +
			"the operation's risk, reversibility, config opt-in, irreversible acknowledgement, " +
			"and verification policy before executing it.",
		// Marked mutating because the write operations reachable here (place,
		// cancel, modify) are the same ones `tossctl order` exposes — the gate
		// lives in trading.Service, not in the command, so this door is no more
		// permissive, but it is a door.
		Annotations: map[string]string{
			"source": "both", "mutating": "true", "writes_state": "possible",
			"mutation_risk": "operation-defined", "reversibility": "operation-defined",
		},
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			catalog := ops.NewCatalog()
			if _, ok := catalog.Get(args[0]); !ok {
				return fmt.Errorf("unknown operation %q; run `tossctl ops list` to see the available ids", args[0])
			}
			// Decoded before touching credentials so a typo in the JSON is
			// reported as a typo, not as a login problem.
			callArgs := map[string]any{}
			if cmd.Flags().Changed("params") {
				if err := jsoninput.Decode([]byte(params), &callArgs); err != nil {
					return fmt.Errorf("--params is not a JSON object: %w", err)
				}
				if callArgs == nil {
					return fmt.Errorf("--params is not a JSON object: got null")
				}
			}

			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			officialClient := app.client.Official()
			// The operation catalog declares regular and conditional orders as
			// official-only. Keep this machine surface identical to MCP even though
			// the human-oriented `order` command retains its legacy hybrid broker.
			officialTrading := trading.NewService(app.config.Trading, ops.OfficialBroker{Client: officialClient}).
				WithConditionalBroker(app.client).
				WithLineage(app.lineageService)
			deps := &ops.Deps{
				Client:         officialClient,
				WTS:            app.client,
				Trading:        officialTrading,
				OpenAPIIP:      openapiip.NewService(app.client, openapiip.NewHTTPResolver(nil, "")),
				PriceAlerts:    pricealert.NewService(app.client),
				HiddenHoldings: hiddenholding.NewService(app.client),
				Watchlists:     watchlistservice.NewService(app.client),
				Auth:           authSnapshot(app.session, app.client.Official(), app.tokenFile),
			}
			result, err := catalog.Call(cmd.Context(), deps, args[0], callArgs)
			if err != nil {
				return err
			}
			return output.WriteJSON(cmd.OutOrStdout(), result)
		},
	}
	cmd.Flags().StringVar(&params, "params", "", "Operation parameters as a JSON object")
	return cmd
}
