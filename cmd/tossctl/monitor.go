package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/JungHoonGhae/tossinvest-cli/internal/monitor"
	"github.com/spf13/cobra"
)

func newMonitorCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "monitor",
		Short: "Periodic health checks against Toss read-only endpoints",
	}

	apiCmd := &cobra.Command{
		Use:         "api",
		Short:       "Run schema-invariant probes; exit 1 on any failure",
		Annotations: map[string]string{"source": "wts"},
		Long: `Run schema-invariant probes against the read-only Toss endpoints the
CLI depends on. Designed for cron / launchd.

Exits 0 when every probe passes, 1 if any probe fails. Compose alert
channels (Discord, Slack, ntfy, mail, …) in the cron line via "||".
See AGENTS.md for recipes.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			if app.session == nil {
				return errors.New("no active session; run `tossctl auth login` first")
			}

			results := monitor.Run(cmd.Context(), app.session)
			printResults(cmd.OutOrStdout(), cmd.OutOrStderr(), results, monitorQuiet)
			for _, r := range results {
				if !r.OK {
					os.Exit(1)
				}
			}
			return nil
		},
	}
	apiCmd.Flags().BoolVar(&monitorQuiet, "quiet", false, "Only print failed probes")

	cmd.AddCommand(apiCmd)
	return cmd
}

var monitorQuiet bool

func printResults(stdout, stderr io.Writer, results []monitor.Result, quiet bool) {
	pass, fail := 0, 0
	for _, r := range results {
		if r.OK {
			pass++
		} else {
			fail++
		}
	}
	if !quiet {
		for _, r := range results {
			if r.OK {
				fmt.Fprintf(stdout, "  ✓ %s — status=%d (%dms)\n", r.Probe.Name, r.Status, r.Duration.Milliseconds())
			}
		}
	}
	for _, r := range results {
		if !r.OK {
			fmt.Fprintf(stderr, "  ✗ %s — status=%d: %s\n", r.Probe.Name, r.Status, r.Detail)
		}
	}
	fmt.Fprintf(stdout, "\n%d passed, %d failed\n", pass, fail)
}
