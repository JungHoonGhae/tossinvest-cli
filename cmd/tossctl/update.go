package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/JungHoonGhae/tossinvest-cli/internal/selfupdate"
	"github.com/JungHoonGhae/tossinvest-cli/internal/tui"
	"github.com/JungHoonGhae/tossinvest-cli/internal/updatecheck"
	"github.com/JungHoonGhae/tossinvest-cli/internal/version"
	"github.com/spf13/cobra"
)

func newUpdateCmd(opts *rootOptions) *cobra.Command {
	var assumeYes bool
	var checkOnly bool

	cmd := &cobra.Command{
		Use:         "update",
		Short:       i18n.T("update.short"),
		Long:        i18n.T("update.long"),
		Annotations: map[string]string{"source": "local"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			format, err := output.ParseFormat(opts.outputFormat)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()

			execPath, err := os.Executable()
			if err != nil {
				return fmt.Errorf("resolving executable path: %w", err)
			}
			if resolved, err := filepath.EvalSymlinks(execPath); err == nil {
				execPath = resolved
			}
			method := selfupdate.DetectInstallMethod(execPath, version.Version)

			if method == selfupdate.MethodDev {
				return selfupdate.ErrDevBuild
			}

			cachePath, err := resolveUpdateCachePath(opts)
			if err != nil {
				return fmt.Errorf("resolving update cache path: %w", err)
			}
			checker := updatecheck.New(cachePath)

			var latest string
			spinErr := tui.WithSpinner("Checking latest version", func() error {
				latest = checker.LatestStable(cmd.Context())
				return nil
			})
			if spinErr != nil {
				return spinErr
			}

			updateAvailable := updatecheck.IsNewer(latest, version.Version)

			if checkOnly {
				return writeUpdateCheckResult(out, format, version.Version, latest, updateAvailable, method)
			}

			if !updateAvailable {
				fmt.Fprintf(out, "Already up to date (v%s).\n", version.Version)
				return nil
			}

			enabled := output.ColorEnabled(out, format)
			if !assumeYes {
				title := fmt.Sprintf(
					"Update tossctl %s -> %s?",
					version.Version,
					output.StyleHighlight(latest, enabled),
				)
				confirmed, confirmErr := tui.Confirm(title)
				switch {
				case errors.Is(confirmErr, tui.ErrNotInteractive):
					assumeYes = true
				case confirmErr != nil:
					return confirmErr
				case !confirmed:
					fmt.Fprintln(out, "Update canceled.")
					return nil
				}
			}
			if assumeYes {
				fmt.Fprintf(out, "Updating %s -> %s...\n", version.Version, latest)
			}

			runErr := selfupdate.Run(cmd.Context(), selfupdate.Options{
				Method: method,
				Out:    out,
				ErrOut: cmd.ErrOrStderr(),
			})
			if runErr != nil {
				fmt.Fprintln(out, output.StyleFail(fmt.Sprintf("✗ Update failed: %v", runErr), enabled))
				return runErr
			}
			if method == selfupdate.MethodBinary && runtime.GOOS == "windows" {
				// install.ps1 is still running in the background at this
				// point (see selfupdate.Run's Windows branch) — it hasn't
				// necessarily finished replacing tossctl.exe yet, so we
				// don't claim success here.
				return nil
			}
			fmt.Fprintln(out, output.StyleSuccess(fmt.Sprintf("✓ Updated to v%s", latest), enabled))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&assumeYes, "yes", "y", false, "skip the confirmation prompt (also implied when stdin/stdout are not a TTY)")
	cmd.Flags().BoolVar(&checkOnly, "check", false, "only report whether an update is available; do not update")

	return cmd
}

func writeUpdateCheckResult(out io.Writer, format output.Format, current, latest string, available bool, method selfupdate.InstallMethod) error {
	if format == output.FormatJSON {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(map[string]any{
			"current":          current,
			"latest":           latest,
			"update_available": available,
			"install_method":   method.String(),
		})
	}
	if available {
		fmt.Fprintf(out, "Update available: %s -> %s (install method: %s)\n", current, latest, method.String())
	} else {
		fmt.Fprintf(out, "Already up to date (v%s).\n", current)
	}
	return nil
}
