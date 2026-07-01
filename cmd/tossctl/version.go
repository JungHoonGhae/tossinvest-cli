package main

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/JungHoonGhae/tossinvest-cli/internal/updatecheck"
	"github.com/JungHoonGhae/tossinvest-cli/internal/version"
	"github.com/spf13/cobra"
)

func newVersionCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:         "version",
		Short:       i18n.T("version.short"),
		Annotations: map[string]string{"source": "local"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			info := version.Current()
			latest := lookupLatest(cmd.Context(), opts)
			updateAvailable := updatecheck.IsNewer(latest, info.Version)

			switch output.Format(opts.outputFormat) {
			case output.FormatJSON:
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(map[string]any{
					"version":          info.Version,
					"commit":           info.Commit,
					"date":             info.Date,
					"os":               runtime.GOOS,
					"arch":             runtime.GOARCH,
					"latest":           latest,
					"update_available": updateAvailable,
				})
			case output.FormatCSV:
				return fmt.Errorf("csv output is not supported for version")
			default:
				if _, err := fmt.Fprintf(
					cmd.OutOrStdout(),
					"tossctl %s\ncommit: %s\ndate: %s\nos/arch: %s/%s\n",
					info.Version,
					info.Commit,
					valueOrDefault(info.Date, "n/a"),
					runtime.GOOS,
					runtime.GOARCH,
				); err != nil {
					return err
				}
				if latest != "" {
					suffix := ""
					if updateAvailable {
						suffix = " (update available — `brew upgrade tossctl-cli` or " + version.ReleasesLatestURL + ")"
					}
					if _, err := fmt.Fprintf(cmd.OutOrStdout(), "latest: %s%s\n", latest, suffix); err != nil {
						return err
					}
				}
				return nil
			}
		},
	}
}

// lookupLatest returns the cached latest tag, falling back to a fresh fetch
// when the cache is stale. Failure paths return empty string so version
// output stays clean even when offline.
func lookupLatest(ctx context.Context, opts *rootOptions) string {
	cachePath, err := resolveUpdateCachePath(opts)
	if err != nil {
		return ""
	}
	lookupCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return updatecheck.New(cachePath).LatestStable(lookupCtx)
}

func valueOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
