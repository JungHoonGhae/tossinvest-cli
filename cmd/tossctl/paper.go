package main

import (
	"fmt"
	"strconv"

	"github.com/JungHoonGhae/tossinvest-cli/internal/featuregate"
	"github.com/JungHoonGhae/tossinvest-cli/internal/orderintent"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/JungHoonGhae/tossinvest-cli/internal/papertrading"
	"github.com/spf13/cobra"
)

func paperReadAnnotations() map[string]string {
	return map[string]string{"source": "wts", "domain": "securities", "environment": "paper", "experimental": featuregate.PaperTrading}
}

func paperMutationAnnotations(reversibility string) map[string]string {
	a := mutationAnnotations("wts", "securities", "simulation", reversibility)
	a["environment"] = "paper"
	a["authorization"] = "simulation_execute"
	a["experimental"] = featuregate.PaperTrading
	return a
}

func newPaperCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use: "paper", Short: "Use the isolated US-options paper-trading environment", Hidden: true,
		Annotations: map[string]string{"experimental": featuregate.PaperTrading},
	}
	cmd.AddCommand(
		newPaperStatusCmd(opts), newPaperInitCmd(opts), newPaperDepositCmd(opts),
		newPaperOrderCmd(opts), newPaperOrdersCmd(opts),
	)
	return cmd
}

func newPaperStatusCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use: "status", Short: "Show simulated cash and prerequisite progress",
		Annotations: paperReadAnnotations(), Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			status, err := papertrading.NewService(app.client).Status(cmd.Context())
			if err != nil {
				return userFacingCommandError(err)
			}
			return writePaperValue(cmd, app.format, status)
		},
	}
}

func newPaperInitCmd(opts *rootOptions) *cobra.Command {
	var execute bool
	cmd := &cobra.Command{
		Use: "init", Short: "Initialize or apply for the paper-options environment",
		Annotations: paperMutationAnnotations("unknown"), Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			plan, err := papertrading.NewService(app.client).Initialize(cmd.Context(), papertrading.ExecuteOptions{Execute: execute})
			if err != nil {
				return userFacingCommandError(err)
			}
			return writePaperValue(cmd, app.format, plan)
		},
	}
	cmd.Flags().BoolVar(&execute, "execute", false, "Apply the simulation-only change (otherwise preview)")
	return cmd
}

func newPaperDepositCmd(opts *rootOptions) *cobra.Command {
	var execute bool
	cmd := &cobra.Command{
		Use: "deposit <amount>", Short: "Add simulated cash to the paper account",
		Annotations: paperMutationAnnotations("unknown"), Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			amount, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("amount must be a whole number")
			}
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			plan, err := papertrading.NewService(app.client).Deposit(cmd.Context(), amount, papertrading.ExecuteOptions{Execute: execute})
			if err != nil {
				return userFacingCommandError(err)
			}
			return writePaperValue(cmd, app.format, plan)
		},
	}
	cmd.Flags().BoolVar(&execute, "execute", false, "Apply the simulation-only change (otherwise preview)")
	return cmd
}

func newPaperOrderCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "order", Short: "Place or cancel one simulated option order"}
	cmd.AddCommand(newPaperOrderPlaceCmd(opts), newPaperOrderLivePreviewCmd(opts), newPaperOrderCancelCmd(opts))
	return cmd
}

type paperOptionFlags struct {
	side, orderType, currency, exchange string
	price                               float64
	quantity                            int
}

func (f paperOptionFlags) intent(symbol string) (orderintent.OptionPlaceIntent, error) {
	return orderintent.NormalizeOptionPlace(orderintent.OptionPlaceInput{
		Symbol: symbol, Exchange: f.exchange, CurrencyMode: f.currency,
		Side: f.side, OrderType: f.orderType, Price: f.price, Quantity: f.quantity,
	})
}

func bindPaperOptionFlags(cmd *cobra.Command, flags *paperOptionFlags) {
	cmd.Flags().StringVar(&flags.side, "side", "buy", "Order side: buy|sell")
	cmd.Flags().StringVar(&flags.orderType, "type", "limit", "Order type: limit|market")
	cmd.Flags().StringVar(&flags.currency, "currency-mode", "USD", "Currency mode: USD|KRW")
	cmd.Flags().StringVar(&flags.exchange, "market", "", "Option exchange code override (normally resolved automatically)")
	cmd.Flags().Float64Var(&flags.price, "price", 0, "Limit price per share in the selected currency")
	cmd.Flags().IntVar(&flags.quantity, "quantity", 1, "Whole option-contract quantity")
}

func newPaperOrderPlaceCmd(opts *rootOptions) *cobra.Command {
	flags := &paperOptionFlags{}
	var execute bool
	cmd := &cobra.Command{
		Use: "place <option-code>", Short: "Place a simulated US-options order",
		Annotations: paperMutationAnnotations("irreversible-in-simulation"), Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			intent, err := flags.intent(args[0])
			if err != nil {
				return err
			}
			plan, err := papertrading.NewService(app.client).Place(cmd.Context(), intent, papertrading.ExecuteOptions{Execute: execute})
			if err != nil {
				return userFacingCommandError(err)
			}
			return writePaperValue(cmd, app.format, plan)
		},
	}
	bindPaperOptionFlags(cmd, flags)
	cmd.Flags().BoolVar(&execute, "execute", false, "Place the simulation-only order (otherwise preview)")
	return cmd
}

func newPaperOrderLivePreviewCmd(opts *rootOptions) *cobra.Command {
	flags := &paperOptionFlags{}
	cmd := &cobra.Command{
		Use: "live-preview <option-code>", Aliases: []string{"promote"},
		Short: "Run the same option intent through the live-order guardrails",
		Annotations: map[string]string{
			"source": "local", "domain": "securities", "environment": "live", "transition_from": "paper",
			"experimental": featuregate.PaperTrading,
		},
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			paperIntent, err := flags.intent(args[0])
			if err != nil {
				return err
			}
			liveIntent, err := papertrading.LivePreviewIntent(paperIntent)
			if err != nil {
				return err
			}
			return output.WriteTradingPreview(cmd.OutOrStdout(), app.format, app.tradingService.PreviewPlace(liveIntent))
		},
	}
	bindPaperOptionFlags(cmd, flags)
	return cmd
}

func newPaperOrderCancelCmd(opts *rootOptions) *cobra.Command {
	var execute bool
	cmd := &cobra.Command{
		Use: "cancel <order-id>", Short: "Cancel one pending simulated order",
		Annotations: paperMutationAnnotations("irreversible-in-simulation"), Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			plan, err := papertrading.NewService(app.client).Cancel(cmd.Context(), args[0], papertrading.ExecuteOptions{Execute: execute})
			if err != nil {
				return userFacingCommandError(err)
			}
			return writePaperValue(cmd, app.format, plan)
		},
	}
	cmd.Flags().BoolVar(&execute, "execute", false, "Cancel the simulation-only order (otherwise preview)")
	return cmd
}

func newPaperOrdersCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "orders", Short: "Inspect or bulk-cancel simulated orders"}
	cmd.AddCommand(
		&cobra.Command{
			Use: "pending", Short: "List pending simulated orders", Annotations: paperReadAnnotations(), Args: cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				app, err := newAppContext(opts)
				if err != nil {
					return err
				}
				orders, err := papertrading.NewService(app.client).PendingOrders(cmd.Context())
				if err != nil {
					return userFacingCommandError(err)
				}
				return writePaperValue(cmd, app.format, orders)
			},
		},
		&cobra.Command{
			Use: "completed", Short: "List completed or cancelled simulated orders", Annotations: paperReadAnnotations(), Args: cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				app, err := newAppContext(opts)
				if err != nil {
					return err
				}
				orders, err := papertrading.NewService(app.client).CompletedOrders(cmd.Context())
				if err != nil {
					return userFacingCommandError(err)
				}
				return writePaperValue(cmd, app.format, orders)
			},
		},
		newPaperBulkCancelCmd(opts),
	)
	return cmd
}

func newPaperBulkCancelCmd(opts *rootOptions) *cobra.Command {
	var side string
	var execute bool
	cmd := &cobra.Command{
		Use: "cancel-all", Short: "Cancel pending simulated orders in bulk",
		Annotations: paperMutationAnnotations("irreversible-in-simulation"), Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			plan, err := papertrading.NewService(app.client).BulkCancel(cmd.Context(), side, papertrading.ExecuteOptions{Execute: execute})
			if err != nil {
				return userFacingCommandError(err)
			}
			return writePaperValue(cmd, app.format, plan)
		},
	}
	cmd.Flags().StringVar(&side, "side", "", "Only cancel buy or sell orders")
	cmd.Flags().BoolVar(&execute, "execute", false, "Cancel the simulation-only orders (otherwise preview)")
	return cmd
}

func writePaperValue(cmd *cobra.Command, format output.Format, value any) error {
	if format == output.FormatCSV {
		return fmt.Errorf("csv output is not supported for paper trading; use --output json")
	}
	return output.WriteJSON(cmd.OutOrStdout(), value)
}
