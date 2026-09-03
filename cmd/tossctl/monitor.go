package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/monitor"
	"github.com/spf13/cobra"
)

func newMonitorCmd(opts *rootOptions) *cobra.Command {
	var quiet bool
	cmd := &cobra.Command{
		Use:   "monitor",
		Short: i18n.T("monitor.short"),
	}

	apiCmd := &cobra.Command{
		Use:         "api",
		Short:       i18n.T("monitor.api.short"),
		Annotations: map[string]string{"source": "wts"},
		Long:        i18n.T("monitor.api.long"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			if app.session == nil {
				return errors.New("no active session; run `tossctl auth login` first")
			}

			results := monitor.Run(cmd.Context(), app.session, enabledExperiments(app.config)...)
			printResults(cmd.OutOrStdout(), cmd.OutOrStderr(), results, quiet)
			for _, r := range results {
				if !r.OK && !r.Skipped {
					os.Exit(1)
				}
			}
			return nil
		},
	}
	apiCmd.Flags().BoolVar(&quiet, "quiet", false, "Only print failed probes")

	cmd.AddCommand(apiCmd)
	return cmd
}

func printResults(stdout, stderr io.Writer, results []monitor.Result, quiet bool) {
	pass, fail, skipped := 0, 0, 0
	for _, r := range results {
		if r.Skipped {
			skipped++
		} else if r.OK {
			pass++
		} else {
			fail++
		}
	}
	if !quiet {
		for _, r := range results {
			if r.OK {
				fmt.Fprintf(stdout, "  ✓ %s — status=%d (%dms)\n", r.Probe.Name, r.Status, r.Duration.Milliseconds())
			} else if r.Skipped {
				fmt.Fprintf(stdout, "  - %s — %s\n", r.Probe.Name, r.Detail)
			}
		}
	}
	authFailures := 0
	for _, r := range results {
		if !r.OK && !r.Skipped {
			fmt.Fprintf(stderr, "  ✗ %s — status=%d: %s\n", r.Probe.Name, r.Status, r.Detail)
			if r.Status == http.StatusUnauthorized || r.Status == http.StatusForbidden {
				authFailures++
			}
		}
	}
	fmt.Fprintf(stdout, "\n%d passed, %d failed, %d skipped\n", pass, fail, skipped)

	// One expired session knocks out every account-scoped probe at once. Without
	// this line the output is N separate 401s, which reads as "Toss broke N
	// endpoints" — the opposite of the truth, and an expensive thing to chase.
	// The typed clients say this via internal/client's auth-error mapping, but
	// probes deliberately bypass the typed client, so nothing else says it here.
	if authFailures > 0 {
		fmt.Fprintf(stderr,
			"\n⚠ %d of %d failures are 401/403 — likely one expired session, not %d broken endpoints.\n"+
				"  Check with `tossctl auth status`; renew with `tossctl auth extend` or `tossctl auth login`.\n",
			authFailures, fail, authFailures)
	}
}
