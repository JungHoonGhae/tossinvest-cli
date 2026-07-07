package main

import (
	"fmt"
	"os"

	"github.com/JungHoonGhae/tossinvest-cli/internal/mcp"
	"github.com/JungHoonGhae/tossinvest-cli/internal/official"
	"github.com/JungHoonGhae/tossinvest-cli/internal/trading"
	"github.com/JungHoonGhae/tossinvest-cli/internal/version"
	"github.com/spf13/cobra"
)

// newMCPCmd builds the `tossctl mcp` command: a stdio Model Context Protocol
// server exposing the official Toss Open API through a catalog tool surface
// (list_operations / describe_operation / call_operation).
//
// It speaks JSON-RPC 2.0 over stdin/stdout and is meant to be launched by an
// MCP host (Claude Code, Claude Desktop, Codex, etc.), not run interactively.
func newMCPCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Run a stdio MCP server over the official Toss Open API (catalog surface)",
		Long: "Run a Model Context Protocol server on stdin/stdout that exposes the " +
			"official Toss Open API as a catalog of operations (reads plus gated order " +
			"place/cancel/modify). Configure it in an MCP host as the command " +
			"`tossctl mcp`. Order mutations follow the same config gate and " +
			"execute/confirm flow as `tossctl order` and use the official API only " +
			"(no WTS). Requires saved Open API credentials (`tossctl openapi login`).",
		Annotations:  map[string]string{"source": "official"},
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			credFile, tokenFile, err := resolveOpenAPIPaths(opts)
			if err != nil {
				return err
			}
			creds, err := official.LoadCredentials(os.Getenv, credFile)
			if err != nil {
				return err
			}
			if creds == nil {
				return fmt.Errorf("no Open API credentials found; run `tossctl openapi login --key K --secret S` first")
			}
			client := official.New(*creds, tokenFile)

			// Trading (order place/cancel/modify) is gated by config exactly as
			// the CLI's `tossctl order` is, and routed through an official-only
			// broker so writes never touch a WTS session.
			cfg, err := loadConfig(opts)
			if err != nil {
				return err
			}
			tradingSvc := trading.NewService(cfg.Trading, mcp.OfficialBroker{Client: client})

			server := mcp.NewServer(client, tradingSvc, "tossinvest-cli", version.Current().Version)
			// Serve blocks until stdin reaches EOF (host closed the pipe).
			return server.Serve(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}
}
