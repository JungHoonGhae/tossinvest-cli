package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	tossclient "github.com/JungHoonGhae/tossinvest-cli/internal/client"
	"github.com/JungHoonGhae/tossinvest-cli/internal/config"
	"github.com/JungHoonGhae/tossinvest-cli/internal/hiddenholding"
	"github.com/JungHoonGhae/tossinvest-cli/internal/hybrid"
	"github.com/JungHoonGhae/tossinvest-cli/internal/mcp"
	"github.com/JungHoonGhae/tossinvest-cli/internal/official"
	"github.com/JungHoonGhae/tossinvest-cli/internal/openapiip"
	"github.com/JungHoonGhae/tossinvest-cli/internal/orderlineage"
	"github.com/JungHoonGhae/tossinvest-cli/internal/papertrading"
	"github.com/JungHoonGhae/tossinvest-cli/internal/pricealert"
	"github.com/JungHoonGhae/tossinvest-cli/internal/routing"
	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
	"github.com/JungHoonGhae/tossinvest-cli/internal/trading"
	"github.com/JungHoonGhae/tossinvest-cli/internal/updatecheck"
	"github.com/JungHoonGhae/tossinvest-cli/internal/version"
	watchlistservice "github.com/JungHoonGhae/tossinvest-cli/internal/watchlist"
	"github.com/spf13/cobra"
)

func mcpOfficialBackends(cfg config.File, creds *official.Credentials, tokenFile, lineageFile string, prefer routing.Preference) (*official.Client, *trading.Service) {
	if creds == nil || !cfg.OpenAPI.Enabled || prefer == routing.WTS {
		return nil, nil
	}
	officialClient := official.New(*creds, tokenFile)
	tradingSvc := trading.NewService(cfg.Trading, mcp.OfficialBroker{Client: officialClient}).
		WithLineage(orderlineage.NewService(lineageFile))
	return officialClient, tradingSvc
}

// newMCPCmd builds the `tossctl mcp` command: a stdio Model Context Protocol
// server exposing official, WTS, and enabled experimental operations through a catalog tool surface
// (list_operations / describe_operation / call_operation).
//
// It speaks JSON-RPC 2.0 over stdin/stdout and is meant to be launched by an
// MCP host (Claude Code, Claude Desktop, Codex, etc.), not run interactively.
func newMCPCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Run a stdio MCP server over the Toss official Open API + WTS (catalog surface)",
		Long: "Run a Model Context Protocol server on stdin/stdout that exposes Toss " +
			"Securities as a catalog of operations. Configure it in an MCP host as the " +
			"command `tossctl mcp`. It covers the official Open API (reads plus gated " +
			"order place/cancel/modify) and, when a web session is present, the WTS-only " +
			"reads (rankings, flows, AI signals, screener, sectors, earnings, briefing, " +
			"community, dividends, Prime, transactions), plus safely gated account-setting " +
			"writes. Opted-in paper-trading operations are labeled experimental and target " +
			"only the isolated paper environment. Every write publishes its risk, reversibility, " +
			"approval, and verification policy. Live order mutations follow the same config " +
			"gate and execute/confirm flow as `tossctl order` and use the official " +
			"API only (no WTS). Needs at least one credential: `tossctl openapi login` " +
			"(official) and/or `tossctl auth login` (WTS web session).",
		Annotations:  map[string]string{"source": "both"},
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := loadConfig(opts)
			if err != nil {
				return err
			}
			prefer, err := resolveBackend(cfg.OpenAPI, opts.backend)
			if err != nil {
				return err
			}

			// Official Open API client (optional): serves official reads + order
			// writes when credentials are present.
			credFile, tokenFile, err := resolveOpenAPIPaths(opts)
			if err != nil {
				return err
			}
			creds, err := official.LoadCredentials(os.Getenv, credFile)
			if err != nil {
				return err
			}
			// Only the effective routing policy may enable official reads and
			// order writes. A saved credential must not bypass openapi.enabled=false
			// or an explicit --backend wts pin.
			officialClient, tradingSvc := mcpOfficialBackends(
				cfg, creds, tokenFile, resolveLineageFile(opts), prefer,
			)

			// WTS web-session client (optional): serves the WTS-only reads.
			store := session.NewFileStore(resolveSessionFile(opts))
			sess, err := store.Load(context.Background())
			if err != nil && !errors.Is(err, session.ErrNoSession) {
				return err
			}
			var wtsClient *tossclient.Client
			if sess != nil {
				wtsClient = tossclient.New(tossclient.Config{Session: sess, TradingPolicy: cfg.Trading})
			}

			if officialClient == nil && wtsClient == nil {
				return fmt.Errorf("no credentials found; run `tossctl openapi login` (official API) and/or `tossctl auth login` (WTS web session) first")
			}

			// Hybrid router — the same official→WTS fallback the CLI applies, so
			// agents and humans resolve a read the same way. hybrid embeds the WTS
			// client, so it needs a non-nil one even when no session exists; the
			// sessionless client is never reached, because Catalog.Call gates WTS
			// operations on the auth snapshot set below.
			routedWTS := wtsClient
			if routedWTS == nil {
				routedWTS = tossclient.New(tossclient.Config{TradingPolicy: cfg.Trading})
			}
			routed := hybrid.New(routedWTS, officialClient,
				hybrid.Policy{Prefer: prefer, Fallback: cfg.OpenAPI.Fallback}, os.Stderr)
			if tradingSvc != nil {
				tradingSvc.WithConditionalBroker(routed)
			}

			var ipManager *openapiip.Service
			if sess != nil {
				ipManager = openapiip.NewService(routed, openapiip.NewHTTPResolver(nil, ""))
			}
			server := mcp.NewServer(officialClient, routed, mcp.Services{
				Trading:        tradingSvc,
				OpenAPIIP:      ipManager,
				PriceAlerts:    pricealert.NewService(routed),
				HiddenHoldings: hiddenholding.NewService(routed),
				Watchlists:     watchlistservice.NewService(routed),
				Paper:          papertrading.NewService(routed),
				Experiments:    enabledExperiments(cfg),
			}, "tossinvest-cli", version.Current().Version)

			// Read-only auth snapshot for the auth_status operation (no secrets —
			// only connected flags + expiry timestamps).
			server.SetAuthStatus(authSnapshot(sess, officialClient, tokenFile))

			// MCP-only users never see the CLI's stderr update notices, so surface
			// "update available" through the initialize `instructions` (the agent can
			// relay it). Bounded + cached; a network failure is silent.
			if cachePath, perr := resolveUpdateCachePath(opts); perr == nil {
				checkCtx, cancel := context.WithTimeout(cmd.Context(), 2*time.Second)
				latest := updatecheck.New(cachePath).LatestStable(checkCtx)
				cancel()
				cur := version.Current().Version
				if updatecheck.IsNewer(latest, cur) {
					server.AppendInstructions(fmt.Sprintf(
						"Update available: tossctl v%s (this server runs v%s). Tell the user they can update with `brew upgrade tossctl` or `tossctl update`, then restart this MCP server to pick it up.",
						latest, cur))
				}
			}

			// Serve blocks until stdin reaches EOF (host closed the pipe).
			return server.Serve(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}
}
