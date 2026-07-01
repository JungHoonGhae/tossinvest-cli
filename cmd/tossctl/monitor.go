package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/monitor"
	"github.com/spf13/cobra"
)

func newMonitorCmd(opts *rootOptions) *cobra.Command {
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
