package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/push"
	"github.com/spf13/cobra"
)

func newPushCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "push",
		Short: i18n.T("push.short"),
	}

	var retry bool

	listenCmd := &cobra.Command{
		Use:         "listen",
		Short:       i18n.T("push.listen.short"),
		Annotations: map[string]string{"source": "wts"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			if app.session == nil {
				return fmt.Errorf("no active session; run `tossctl auth login` first")
			}

			ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer cancel()

			enc := json.NewEncoder(cmd.OutOrStdout())
			handler := func(ev push.Event) {
				_ = enc.Encode(ev)
			}

			listener := push.NewListener(
				app.session,
				push.WithLogger(func(format string, args ...any) {
					fmt.Fprintf(cmd.ErrOrStderr(), format+"\n", args...)
				}),
			)

			if retry {
				err = listener.ListenWithRetry(ctx, handler)
			} else {
				err = listener.Listen(ctx, handler)
			}
			if ctx.Err() != nil {
				return nil
			}
			return err
		},
	}
	listenCmd.Flags().BoolVar(&retry, "retry", true, "Auto-reconnect with exponential backoff when the stream drops")

	cmd.AddCommand(listenCmd)
	return cmd
}
