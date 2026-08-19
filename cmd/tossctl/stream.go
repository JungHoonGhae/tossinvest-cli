package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/official"
	"github.com/spf13/cobra"
)

// 실시간 웹소켓 구독. 프로토콜이 **한 연결에 배열 하나(선언형 full-replace)** 라서
// 커맨드도 채널별로 쪼개지 않고 하나로 둔다 — 여러 채널을 한 번에 선언하면 연결·
// keepalive·재연결이 한 벌로 끝난다. 스펙: docs/migration/asyncapi.latest.json.
func newStreamCmd(opts *rootOptions) *cobra.Command {
	var (
		trade     []string
		orderbook []string
		order     bool
		retry     bool
	)

	cmd := &cobra.Command{
		Use:         "stream",
		Short:       i18n.T("stream.short"),
		Long:        i18n.T("stream.long"),
		Annotations: map[string]string{"source": "official"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			client := app.client.Official()
			if client == nil {
				return fmt.Errorf("stream requires official Open API credentials; run `tossctl openapi login` first")
			}

			var accounts []string
			if order {
				// 주문 채널의 code 는 계좌번호가 아니라 accountSeq 이고, 숫자를
				// 문자열로 넣어야 한다.
				list, err := client.Accounts(cmd.Context())
				if err != nil {
					return fmt.Errorf("resolving accounts for order stream: %w", err)
				}
				for _, a := range list {
					// domain.Account.ID 가 곧 accountSeq 다 (adaptAccounts 참고).
					accounts = append(accounts, a.ID)
				}
			}

			subs := official.StreamSubscriptions(trade, orderbook, accounts)
			if len(subs) == 0 {
				return fmt.Errorf("nothing to subscribe: pass --trade, --orderbook or --order")
			}

			ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer cancel()

			enc := json.NewEncoder(cmd.OutOrStdout())
			handler := func(f official.StreamFrame) { _ = enc.Encode(f) }
			logf := func(format string, args ...any) {
				fmt.Fprintf(cmd.ErrOrStderr(), format+"\n", args...)
			}

			if retry {
				err = client.StreamWithRetry(ctx, subs, handler, logf)
			} else {
				err = client.Stream(ctx, subs, handler)
			}
			if ctx.Err() != nil {
				return nil
			}
			return err
		},
	}

	cmd.Flags().StringSliceVar(&trade, "trade", nil, "Symbols to stream executions for (comma separated)")
	cmd.Flags().StringSliceVar(&orderbook, "orderbook", nil, "Symbols to stream the order book for (comma separated)")
	cmd.Flags().BoolVar(&order, "order", false, "Stream your own order events for every account")
	cmd.Flags().BoolVar(&retry, "retry", true, "Auto-reconnect with exponential backoff and re-declare subscriptions")

	return cmd
}
