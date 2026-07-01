package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/auth"
	"github.com/JungHoonGhae/tossinvest-cli/internal/doctor"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
	"github.com/spf13/cobra"
)

func newAuthCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage Toss Securities session state",
	}

	var (
		loginHeadless bool
		loginQROutput string
	)

	loginCmd := &cobra.Command{
		Use:         "login",
		Short:       "Start browser-assisted login",
		Annotations: map[string]string{"source": "wts"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}

			cfg := app.loginConfig
			cfg.Headless = loginHeadless
			if loginQROutput != "" {
				cfg.QROutputPath = loginQROutput
			}

			sess, err := app.authService.LoginWith(cmd.Context(), cfg)
			if err != nil {
				return err
			}

			return writeImportResult(cmd.OutOrStdout(), app.format, app.paths.SessionFile, sess)
		},
	}
	loginCmd.Flags().BoolVar(&loginHeadless, "headless", false, "Run Chrome headless (required for remote/CLI-only login)")
	loginCmd.Flags().StringVar(&loginQROutput, "qr-output", "", "Path to write the current QR PNG (forward to phone via messenger)")

	var extendTimeout time.Duration
	extendCmd := &cobra.Command{
		Use:         "extend",
		Short:       "Extend session via Toss app push approval",
		Annotations: map[string]string{"source": "wts"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}

			ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer cancel()

			fmt.Fprintln(cmd.ErrOrStderr(), "Waiting for approval in the Toss app on your phone...")
			stop := startSpinner(cmd.ErrOrStderr(), "Waiting for approval", isTerminal(cmd.ErrOrStderr()))
			defer stop()

			result, err := app.authService.Extend(ctx, extendTimeout)
			stop()
			if err != nil {
				return userFacingCommandError(err)
			}
			return writeExtendResult(cmd.OutOrStdout(), result)
		},
	}
	extendCmd.Flags().DurationVar(&extendTimeout, "timeout", 120*time.Second, "Maximum time to wait for phone approval")

	cmd.AddCommand(
		loginCmd,
		extendCmd,
		&cobra.Command{
			Use:         "import-playwright-state <path>",
			Short:       "Import Playwright storage state into tossctl session storage",
			Args:        cobra.ExactArgs(1),
			Annotations: map[string]string{"source": "local"},
			RunE: func(cmd *cobra.Command, args []string) error {
				app, err := newAppContext(opts)
				if err != nil {
					return err
				}

				sess, err := app.authService.ImportPlaywrightState(cmd.Context(), args[0])
				if err != nil {
					return err
				}

				return writeImportResult(cmd.OutOrStdout(), app.format, app.paths.SessionFile, sess)
			},
		},
		&cobra.Command{
			Use:         "status",
			Short:       "Inspect the stored session state",
			Annotations: map[string]string{"source": "local"},
			RunE: func(cmd *cobra.Command, _ []string) error {
				app, err := newAppContext(opts)
				if err != nil {
					return err
				}

				status, err := app.authService.Status(cmd.Context())
				if err != nil {
					return err
				}

				return writeAuthStatus(cmd.OutOrStdout(), app.format, status)
			},
		},
		&cobra.Command{
			Use:         "logout",
			Short:       "Clear the stored session state",
			Annotations: map[string]string{"source": "local"},
			RunE: func(cmd *cobra.Command, _ []string) error {
				app, err := newAppContext(opts)
				if err != nil {
					return err
				}

				cleared, err := app.authService.Logout(cmd.Context())
				if err != nil {
					return err
				}

				return writeLogoutResult(cmd.OutOrStdout(), app.format, app.paths.SessionFile, cleared)
			},
		},
		&cobra.Command{
			Use:         "doctor",
			Short:       "Check whether auth login prerequisites are ready",
			Annotations: map[string]string{"source": "local"},
			RunE: func(cmd *cobra.Command, _ []string) error {
				app, err := newAppContext(opts)
				if err != nil {
					return err
				}
				configStatus, err := app.configService.Status(cmd.Context())
				if err != nil {
					return err
				}

				report, err := doctor.NewService(
					app.paths,
					configStatus,
					app.loginConfig,
					app.authService,
				).RunAuth(cmd.Context())
				if err != nil {
					return err
				}

				return output.WriteAuthDoctorReport(cmd.OutOrStdout(), app.format, report)
			},
		},
	)

	return cmd
}

func writeAuthStatus(w io.Writer, format output.Format, status auth.Status) error {
	switch format {
	case output.FormatJSON:
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(status)
	case output.FormatCSV:
		return fmt.Errorf("csv output is not supported for auth status")
	case output.FormatTable:
		if !status.Active {
			_, err := fmt.Fprintf(w, "No active session\nSession file: %s\n", status.SessionFile)
			return err
		}

		state := "active"
		if status.Expired {
			state = "expired"
		}

		liveCheck := "not checked"
		if status.Validated {
			liveCheck = "valid"
			if !status.Valid {
				liveCheck = "invalid"
			}
		}

		persistence := "session-scoped cookie (≈1h idle timeout — re-login and confirm 'keep me signed in on this device' for long-lived session)"
		if status.ExpiresAt != nil {
			persistence = fmt.Sprintf("persistent cookie (expires %s)", status.ExpiresAt.Format("2006-01-02 15:04:05Z07:00"))
		}

		if _, err := fmt.Fprintf(w, "Session: %s\nProvider: %s\nPersistence: %s\n", state, status.Provider, persistence); err != nil {
			return err
		}
		if status.ServerExpiresAt != nil {
			if _, err := fmt.Fprintf(w, "Server Expiry: %s\n", formatKST(*status.ServerExpiresAt)); err != nil {
				return err
			}
		}
		_, err := fmt.Fprintf(
			w,
			"Live Check: %s\nRetrieved At: %s\nSession File: %s\n",
			liveCheck,
			status.RetrievedAt.Format("2006-01-02 15:04:05Z07:00"),
			status.SessionFile,
		)
		if err != nil {
			return err
		}
		if status.Validated && status.CheckedAt != nil {
			if _, err := fmt.Fprintf(w, "Checked At: %s\n", status.CheckedAt.Format("2006-01-02 15:04:05Z07:00")); err != nil {
				return err
			}
		}
		if status.ValidationError != "" {
			_, err = fmt.Fprintf(w, "Validation Error: %s\n", status.ValidationError)
		}
		return err
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

func writeImportResult(w io.Writer, format output.Format, sessionFile string, sess *session.Session) error {
	payload := map[string]any{
		"provider":     sess.Provider,
		"cookie_count": len(sess.Cookies),
		"storage_keys": len(sess.Storage),
		"session_file": sessionFile,
		"retrieved_at": sess.RetrievedAt,
		"expires_at":   sess.ExpiresAt,
		"persistent":   sess.ExpiresAt != nil,
		"has_xsrf":     sess.Headers["X-XSRF-TOKEN"] != "",
	}

	switch format {
	case output.FormatJSON:
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(payload)
	case output.FormatCSV:
		return fmt.Errorf("csv output is not supported for auth import")
	case output.FormatTable:
		persistence := "session-scoped cookie (≈1h idle timeout)"
		if sess.ExpiresAt != nil {
			persistence = fmt.Sprintf("persistent cookie (expires %s)", sess.ExpiresAt.Format("2006-01-02 15:04:05Z07:00"))
		}
		_, err := fmt.Fprintf(
			w,
			"Imported session\nProvider: %s\nPersistence: %s\nCookies: %d\nStorage Keys: %d\nSession File: %s\n",
			sess.Provider,
			persistence,
			len(sess.Cookies),
			len(sess.Storage),
			sessionFile,
		)
		return err
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

func writeLogoutResult(w io.Writer, format output.Format, sessionFile string, cleared bool) error {
	payload := map[string]any{
		"cleared":      cleared,
		"session_file": sessionFile,
	}

	switch format {
	case output.FormatJSON:
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(payload)
	case output.FormatCSV:
		return fmt.Errorf("csv output is not supported for auth logout")
	case output.FormatTable:
		if cleared {
			_, err := fmt.Fprintf(w, "Cleared session file: %s\n", sessionFile)
			return err
		}

		_, err := fmt.Fprintf(w, "No stored session found at: %s\n", sessionFile)
		return err
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

// kstZone is the named time.Location used for displaying server-side expiry
// times. /api/v1/session/expired-at returns RFC3339Nano with a numeric +09:00
// offset, which time.Parse stores as an anonymous FixedZone (empty name) —
// formatting that with "MST" would print "+0900" instead of "KST". Pinning to
// a named zone before formatting guarantees the human-readable label.
var kstZone = time.FixedZone("KST", 9*3600)

// formatKST renders a time as "2026-MM-DD HH:MM KST". Centralized so the
// auth status / extend result outputs stay byte-identical and a future format
// tweak is one edit.
func formatKST(t time.Time) string {
	return t.In(kstZone).Format("2006-01-02 15:04 MST")
}

func writeExtendResult(w io.Writer, result *auth.ExtendResult) error {
	_, err := fmt.Fprintf(
		w,
		"✓ Extension complete. New expiry: %s (took %s)\n",
		formatKST(result.ServerExpiresAt),
		result.Elapsed.Round(time.Second),
	)
	return err
}

// startSpinner returns a stop function. In non-TTY mode it writes nothing.
// Safe to call stop() more than once.
func startSpinner(w io.Writer, label string, tty bool) func() {
	if !tty {
		return func() {}
	}
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	stopCh := make(chan struct{})
	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		i := 0
		for {
			select {
			case <-stopCh:
				fmt.Fprint(w, "\r\033[K")
				return
			case <-ticker.C:
				fmt.Fprintf(w, "\r%s %s", frames[i%len(frames)], label)
				i++
			}
		}
	}()
	stopped := false
	return func() {
		if stopped {
			return
		}
		stopped = true
		close(stopCh)
		<-doneCh
	}
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
