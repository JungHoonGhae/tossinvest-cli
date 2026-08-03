package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/i18n"
	"github.com/JungHoonGhae/tossinvest-cli/internal/orderintent"
	"github.com/JungHoonGhae/tossinvest-cli/internal/orderlineage"
	"github.com/JungHoonGhae/tossinvest-cli/internal/output"
	"github.com/JungHoonGhae/tossinvest-cli/internal/trading"
	"github.com/JungHoonGhae/tossinvest-cli/internal/tui"
	"github.com/spf13/cobra"
)

// orderItems converts a slice of domain.Order into tui.Item entries for
// interactive pickers. It is a pure function with no side effects.
func orderItems(orders []domain.Order) []tui.Item {
	items := make([]tui.Item, len(orders))
	for i, o := range orders {
		name := o.Name
		if name == "" {
			name = o.Symbol
		}
		qty := strconv.FormatFloat(o.Quantity, 'f', -1, 64)
		price := strconv.FormatFloat(o.Price, 'f', -1, 64)
		items[i] = tui.Item{
			ID:    o.ID,
			Label: fmt.Sprintf("%s (%s) · %s · %s @ %s · %s", name, o.Symbol, o.Side, qty, price, o.OrderDate),
		}
	}
	return items
}

type placeFlags struct {
	symbol       string
	market       string
	side         string
	orderType    string
	quantity     float64
	price        float64
	amount       float64
	currencyMode string
	fractional   bool
}

type executeFlags struct {
	execute bool
	confirm string

	// deprecatedDangerAck backs the retired --dangerously-skip-permissions flag.
	// It is bound but no longer consulted; the live-mutation gate is now
	// `--execute` + `--confirm <token>`. Kept for one release so existing
	// scripts/agents don't break on an unknown flag.
	deprecatedDangerAck bool
}

type amendFlags struct {
	orderID  string
	quantity float64
	price    float64
}

func newOrderCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "order",
		Short: i18n.T("order.short"),
		Long:  i18n.T("order.long"),
	}

	cmd.AddCommand(
		newOrderShowCmd(opts),
		newOrderPreviewCmd(opts),
		newOrderPlaceCmd(opts),
		newOrderCancelCmd(opts),
		newOrderAmendCmd(opts),
		newOrderConditionalCmd(opts),
		newOrderAutoTradeCmd(opts),
	)

	return cmd
}

func newOrderShowCmd(opts *rootOptions) *cobra.Command {
	var market string

	cmd := &cobra.Command{
		Use:         "show [order-id]",
		Short:       i18n.T("order.show.short"),
		Args:        cobra.MaximumNArgs(1),
		Annotations: map[string]string{"source": "wts"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Early non-TTY guard — before expensive app context creation.
			if len(args) == 0 && !tui.IsInteractive(os.Stdin, os.Stdout) {
				return fmt.Errorf("specify an order-id argument, or run in an interactive terminal")
			}

			app, err := newAppContext(opts)
			if err != nil {
				return err
			}

			var orderArg string
			if len(args) == 1 {
				orderArg = args[0]
			} else {
				// Interactive: pick from pending orders; fall back to completed if empty.
				orders, err := app.client.ListPendingOrders(cmd.Context())
				if err != nil {
					return err
				}
				if len(orders) == 0 {
					orders, err = app.client.ListCompletedOrders(cmd.Context(), "all")
					if err != nil {
						return err
					}
				}
				id, err := tui.PickFromList("Select an order to look up", orderItems(orders))
				if err != nil {
					return err
				}
				orderArg = id
			}

			aliases := []string{}
			lineageHintKey := orderArg
			lineageErr := error(nil)
			if app.lineageService != nil {
				if currentOrderID, ok, err := app.lineageService.Resolve(orderArg); err != nil {
					lineageErr = err
				} else if ok {
					aliases = append(aliases, currentOrderID)
					lineageHintKey = currentOrderID
				}
			}

			order, err := app.client.FindOrderWithAliases(cmd.Context(), orderArg, market, aliases...)
			if err != nil {
				if recoveredOrder, recovered, recoveryErr := recoverOrderWithLineageHint(cmd.Context(), app, orderArg, lineageHintKey, market); recoveryErr != nil {
					if lineageErr != nil {
						return fmt.Errorf("%v; local lineage cache %s could not be read: %v", recoveryErr, app.paths.LineageFile, lineageErr)
					}
					return recoveryErr
				} else if recovered {
					return output.WriteOrder(cmd.OutOrStdout(), app.format, recoveredOrder)
				}
				if lineageErr != nil {
					return fmt.Errorf("%w; local lineage cache %s could not be read: %v", err, app.paths.LineageFile, lineageErr)
				}
				return userFacingCommandError(err)
			}

			return output.WriteOrder(cmd.OutOrStdout(), app.format, order)
		},
	}

	cmd.Flags().StringVar(&market, "market", "all", "Completed-history market filter used during lookup: all, us, kr")
	return cmd
}

func newOrderPreviewCmd(opts *rootOptions) *cobra.Command {
	flags := defaultPlaceFlags()

	cmd := &cobra.Command{
		Use:         "preview",
		Short:       i18n.T("order.preview.short"),
		Annotations: map[string]string{"source": "local"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}

			intent, err := orderintent.NormalizePlace(orderintent.PlaceInput{
				Symbol:       flags.symbol,
				Market:       flags.market,
				Side:         flags.side,
				OrderType:    flags.orderType,
				Quantity:     flags.quantity,
				Price:        flags.price,
				Amount:       flags.amount,
				CurrencyMode: flags.currencyMode,
				Fractional:   flags.fractional,
			})
			if err != nil {
				return err
			}

			return output.WriteTradingPreview(cmd.OutOrStdout(), app.format, app.tradingService.PreviewPlace(intent))
		},
	}

	bindPlaceFlags(cmd, flags)
	return cmd
}

func newOrderPlaceCmd(opts *rootOptions) *cobra.Command {
	place := defaultPlaceFlags()
	exec := &executeFlags{}

	cmd := &cobra.Command{
		Use:         "place",
		Short:       i18n.T("order.place.short"),
		Annotations: map[string]string{"source": "both", "mutating": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}

			intent, err := orderintent.NormalizePlace(orderintent.PlaceInput{
				Symbol:       place.symbol,
				Market:       place.market,
				Side:         place.side,
				OrderType:    place.orderType,
				Quantity:     place.quantity,
				Price:        place.price,
				Amount:       place.amount,
				CurrencyMode: place.currencyMode,
				Fractional:   place.fractional,
			})
			if err != nil {
				return err
			}

			result, err := app.tradingService.Place(cmd.Context(), intent, trading.ExecuteOptions{
				Execute: exec.execute,
				Confirm: exec.confirm,
			})
			if err != nil {
				return userFacingPlaceError(app.paths, err, &intent)
			}

			return output.WriteMutationResult(cmd.OutOrStdout(), app.format, result)
		},
	}

	bindPlaceFlags(cmd, place)
	bindExecuteFlags(cmd, exec)
	return cmd
}

func newOrderCancelCmd(opts *rootOptions) *cobra.Command {
	exec := &executeFlags{}
	var orderID string
	var symbol string

	cmd := &cobra.Command{
		Use:         "cancel",
		Short:       i18n.T("order.cancel.short"),
		Annotations: map[string]string{"source": "both", "mutating": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Early non-TTY guard — before expensive app context creation.
			if orderID == "" && !tui.IsInteractive(os.Stdin, os.Stdout) {
				return fmt.Errorf("specify --order-id, or run in an interactive terminal")
			}

			app, err := newAppContext(opts)
			if err != nil {
				return err
			}

			if orderID == "" {
				// Interactive: fetch pending orders and let the user pick.
				orders, err := app.client.ListPendingOrders(cmd.Context())
				if err != nil {
					return err
				}
				id, err := tui.PickFromList("Select an order to cancel", orderItems(orders))
				if err != nil {
					return err
				}
				orderID = id
				for _, o := range orders {
					if o.ID == id {
						symbol = o.Symbol
						break
					}
				}
			}

			intent, err := orderintent.NormalizeCancel(orderID, symbol)
			if err != nil {
				return err
			}

			preview := app.tradingService.PreviewCancel(intent)
			if !exec.execute {
				return output.WriteTradingPreview(cmd.OutOrStdout(), app.format, preview)
			}

			result, err := app.tradingService.Cancel(cmd.Context(), intent, trading.ExecuteOptions{
				Execute: exec.execute,
				Confirm: exec.confirm,
			})
			if err != nil {
				return userFacingTradingError(app.paths, err)
			}

			return output.WriteMutationResult(cmd.OutOrStdout(), app.format, result)
		},
	}

	cmd.Flags().StringVar(&orderID, "order-id", "", "Pending order identifier")
	cmd.Flags().StringVar(&symbol, "symbol", "", "Trading symbol for the pending order")
	bindExecuteFlags(cmd, exec)
	return cmd
}

func newOrderAmendCmd(opts *rootOptions) *cobra.Command {
	flags := &amendFlags{}
	exec := &executeFlags{}

	cmd := &cobra.Command{
		Use:         "amend",
		Short:       i18n.T("order.amend.short"),
		Annotations: map[string]string{"source": "both", "mutating": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Early non-TTY guard — before expensive app context creation.
			if flags.orderID == "" && !tui.IsInteractive(os.Stdin, os.Stdout) {
				return fmt.Errorf("specify --order-id, or run in an interactive terminal")
			}

			app, err := newAppContext(opts)
			if err != nil {
				return err
			}

			if flags.orderID == "" {
				// Interactive: fetch pending orders and let the user pick.
				orders, err := app.client.ListPendingOrders(cmd.Context())
				if err != nil {
					return err
				}
				id, err := tui.PickFromList("Select an order to amend", orderItems(orders))
				if err != nil {
					return err
				}
				flags.orderID = id
			}

			intent, err := orderintent.NormalizeAmend(flags.orderID, optionalFloat64(cmd, "quantity", flags.quantity), optionalFloat64(cmd, "price", flags.price))
			if err != nil {
				return err
			}

			preview := app.tradingService.PreviewAmend(intent)
			if !exec.execute {
				return output.WriteTradingPreview(cmd.OutOrStdout(), app.format, preview)
			}

			result, err := app.tradingService.Amend(cmd.Context(), intent, trading.ExecuteOptions{
				Execute: exec.execute,
				Confirm: exec.confirm,
			})
			if err != nil {
				return userFacingTradingError(app.paths, err)
			}
			return output.WriteMutationResult(cmd.OutOrStdout(), app.format, result)
		},
	}

	cmd.Flags().StringVar(&flags.orderID, "order-id", "", "Pending order identifier")
	cmd.Flags().Float64Var(&flags.quantity, "quantity", 0, "Updated quantity")
	cmd.Flags().Float64Var(&flags.price, "price", 0, "Updated limit price")
	bindExecuteFlags(cmd, exec)
	return cmd
}

func defaultPlaceFlags() *placeFlags {
	return &placeFlags{
		market:       "us",
		orderType:    "limit",
		currencyMode: "KRW",
	}
}

func recoverOrderWithLineageHint(ctx context.Context, app *appContext, requestedOrderID, lineageHintKey, market string) (domain.Order, bool, error) {
	if app == nil || app.lineageService == nil {
		return domain.Order{}, false, nil
	}

	entry, ok, err := app.lineageService.Lookup(lineageHintKey)
	if err != nil {
		return domain.Order{}, false, err
	}
	if !ok {
		return domain.Order{}, false, nil
	}

	order, recovered, err := app.client.FindCompletedOrderFromLineageHint(ctx, requestedOrderID, market, entry)
	if err != nil || !recovered {
		return order, recovered, err
	}

	if err := app.lineageService.Record(lineageHintKey, orderlineage.Entry{
		CurrentOrderID: order.ID,
		Kind:           entry.Kind,
		Symbol:         entry.Symbol,
		Market:         entry.Market,
		Quantity:       entry.Quantity,
		Price:          entry.Price,
		OrderDate:      entry.OrderDate,
	}); err != nil {
		return order, true, nil
	}

	return order, true, nil
}

func bindPlaceFlags(cmd *cobra.Command, flags *placeFlags) {
	cmd.Flags().StringVar(&flags.symbol, "symbol", "", "Trading symbol")
	cmd.Flags().StringVar(&flags.market, "market", flags.market, "Market identifier")
	cmd.Flags().StringVar(&flags.side, "side", "", "Order side: buy or sell")
	cmd.Flags().StringVar(&flags.orderType, "type", flags.orderType, "Order type: limit or market")
	cmd.Flags().Float64Var(&flags.quantity, "qty", 0, "Order quantity")
	cmd.Flags().Float64Var(&flags.price, "price", 0, "Order price for limit orders")
	cmd.Flags().Float64Var(&flags.amount, "amount", 0, "Order amount in KRW for fractional orders")
	cmd.Flags().StringVar(&flags.currencyMode, "currency-mode", flags.currencyMode, "Currency mode")
	cmd.Flags().BoolVar(&flags.fractional, "fractional", false, "Fractional US market order — buy is amount-based (--amount); sell is a decimal share count (--qty, ≤6 places)")
	if err := cmd.MarkFlagRequired("symbol"); err != nil {
		panic(err)
	}
	if err := cmd.MarkFlagRequired("side"); err != nil {
		panic(err)
	}
	// `--qty` is not statically required: fractional BUY is amount-based
	// (`--amount`, no qty), fractional SELL and all other orders use `--qty`.
	// Per-case validation lives in orderintent.NormalizePlace.
}

func bindExecuteFlags(cmd *cobra.Command, flags *executeFlags) {
	cmd.Flags().BoolVar(&flags.execute, "execute", false, "Perform the live mutation instead of a local preview")
	cmd.Flags().StringVar(&flags.confirm, "confirm", "", "Confirmation token from `tossctl order preview`")

	// Retired in v0.5.1: the live-mutation gate is now `--execute` + `--confirm <token>`.
	// The old danger-acknowledgement flag is accepted (and ignored) for one release so
	// existing scripts/agents keep working; cobra prints a deprecation notice on use and
	// hides it from help.
	cmd.Flags().BoolVar(&flags.deprecatedDangerAck, "dangerously-skip-permissions", false, "Deprecated no-op")
	if err := cmd.Flags().MarkDeprecated("dangerously-skip-permissions", "no longer required — `--execute` + `--confirm <token>` is sufficient"); err != nil {
		panic(err)
	}
}

func optionalFloat64(cmd *cobra.Command, name string, value float64) *float64 {
	if !cmd.Flags().Changed(name) {
		return nil
	}
	return &value
}

// newOrderAutoTradeCmd exposes the automated-trading rules the user armed in
// Toss's own UI. Read-only: tossctl does not arm or cancel them.
func newOrderAutoTradeCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:         "autotrade",
		Short:       i18n.T("order.autotrade.short"),
		Long:        i18n.T("order.autotrade.long"),
		Annotations: map[string]string{"source": "wts"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			list, err := app.client.ListAutoTrades(cmd.Context())
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteAutoTrades(cmd.OutOrStdout(), app.format, list)
		},
	}
}

func newOrderConditionalCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "conditional",
		Short: i18n.T("order.conditional.short"),
		Long:  i18n.T("order.conditional.long"),
	}

	var listStatus, listSymbol, listCursor string
	var listLimit int
	listCmd := &cobra.Command{
		Use:         "list",
		Short:       i18n.T("order.conditional.list.short"),
		Annotations: map[string]string{"source": "official"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			l, err := app.client.ConditionalOrders(cmd.Context(), listStatus, listSymbol, listCursor, listLimit)
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteConditionalOrders(cmd.OutOrStdout(), app.format, l)
		},
	}
	listCmd.Flags().StringVar(&listStatus, "status", "OPEN", "lifecycle filter, required by the API (OPEN = in-progress, CLOSED = finished)")
	listCmd.Flags().StringVar(&listSymbol, "symbol", "", "filter by symbol")
	listCmd.Flags().StringVar(&listCursor, "cursor", "", "pagination cursor (from previous next_cursor)")
	listCmd.Flags().IntVar(&listLimit, "limit", 0, "max rows (0 = API default)")

	getCmd := &cobra.Command{
		Use:         "get <conditional-order-id>",
		Short:       i18n.T("order.conditional.get.short"),
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"source": "official"},
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			o, err := app.client.ConditionalOrder(cmd.Context(), args[0])
			if err != nil {
				return userFacingCommandError(err)
			}
			return output.WriteConditionalOrder(cmd.OutOrStdout(), app.format, o)
		},
	}

	var cancelExec bool
	var cancelConfirm string
	cancelCmd := &cobra.Command{
		Use:         "cancel <conditional-order-id>",
		Short:       i18n.T("order.conditional.cancel.short"),
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"source": "official", "mutating": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			intent := orderintent.ConditionalCancelIntent{ID: args[0]}
			canonical := orderintent.CanonicalConditionalCancel(intent)
			gateErr := conditionalGate(app.config, canonical, cancelExec, cancelConfirm)
			if gateErr == errConditionalPreviewOnly {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n%s: %s\n",
					i18n.T("order.conditional.cancel.previewLine"),
					i18n.T("order.conditional.confirmToken"),
					orderintent.ConfirmToken(canonical))
				return nil
			}
			if gateErr != nil {
				return gateErr
			}
			if err := app.client.CancelConditionalOrder(cmd.Context(), intent); err != nil {
				return userFacingCommandError(err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T("order.conditional.cancel.done"))
			return nil
		},
	}
	cancelCmd.Flags().BoolVar(&cancelExec, "execute", false, "actually cancel (omit for preview + confirm token)")
	cancelCmd.Flags().StringVar(&cancelConfirm, "confirm", "", "confirm token from the preview")

	var (
		plSymbol, plType, plOrderType, plExpire, plClientID, plConfirm      string
		plFirstSide, plSecondSide                                           string
		plQty, plFirstTrigger, plFirstOrder, plSecondTrigger, plSecondOrder float64
		plConfirmHigh, plExec                                               bool
	)
	placeCmd := &cobra.Command{
		Use:         "place",
		Short:       i18n.T("order.conditional.place.short"),
		Long:        i18n.T("order.conditional.place.long"),
		Annotations: map[string]string{"source": "official", "mutating": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			intent := orderintent.ConditionalPlaceIntent{
				Symbol: plSymbol, Type: plType, OrderType: plOrderType, ExpireDate: plExpire,
				Quantity: plQty, ClientOrderID: plClientID, ConfirmHighValue: plConfirmHigh,
				First: orderintent.ConditionLeg{OrderSide: plFirstSide, TriggerPrice: plFirstTrigger, OrderPrice: plFirstOrder},
			}
			if plType == "OCO" || plType == "OTO" {
				if plSecondSide == "" {
					return fmt.Errorf("--second-side/--second-trigger required for %s", plType)
				}
				intent.Second = &orderintent.ConditionLeg{OrderSide: plSecondSide, TriggerPrice: plSecondTrigger, OrderPrice: plSecondOrder}
			}
			canonical := orderintent.CanonicalConditionalPlace(intent)
			gateErr := conditionalGate(app.config, canonical, plExec, plConfirm)
			if gateErr == errConditionalPreviewOnly {
				return output.WriteConditionalPlacePreview(cmd.OutOrStdout(), intent, orderintent.ConfirmToken(canonical))
			}
			if gateErr != nil {
				return gateErr
			}
			ref, err := app.client.CreateConditionalOrder(cmd.Context(), intent)
			if err != nil {
				return userFacingCommandError(err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", i18n.T("order.conditional.place.done"), ref.ID)
			return nil
		},
	}
	placeCmd.Flags().StringVar(&plSymbol, "symbol", "", "symbol (required)")
	placeCmd.Flags().StringVar(&plType, "type", "SINGLE", "SINGLE|OCO|OTO")
	placeCmd.Flags().Float64Var(&plQty, "qty", 0, "quantity (required)")
	placeCmd.Flags().StringVar(&plOrderType, "order-type", "LIMIT", "LIMIT|MARKET")
	placeCmd.Flags().StringVar(&plExpire, "expire", "", "expire date YYYY-MM-DD (required)")
	placeCmd.Flags().StringVar(&plFirstSide, "first-side", "", "first leg side BUY|SELL (required)")
	placeCmd.Flags().Float64Var(&plFirstTrigger, "first-trigger", 0, "first leg trigger price (required)")
	placeCmd.Flags().Float64Var(&plFirstOrder, "first-order-price", 0, "first leg limit price (LIMIT)")
	placeCmd.Flags().StringVar(&plSecondSide, "second-side", "", "second leg side (OCO/OTO)")
	placeCmd.Flags().Float64Var(&plSecondTrigger, "second-trigger", 0, "second leg trigger price (OCO/OTO)")
	placeCmd.Flags().Float64Var(&plSecondOrder, "second-order-price", 0, "second leg limit price")
	placeCmd.Flags().StringVar(&plClientID, "client-order-id", "", "idempotency key")
	placeCmd.Flags().BoolVar(&plConfirmHigh, "confirm-high-value", false, "consent for orders >= 1억원")
	placeCmd.Flags().BoolVar(&plExec, "execute", false, "actually place (omit for preview + confirm token)")
	placeCmd.Flags().StringVar(&plConfirm, "confirm", "", "confirm token from the preview")

	var (
		mdType, mdOrderType, mdExpire, mdConfirm, mdFirstSide, mdSecondSide string
		mdQty, mdFirstTrigger, mdFirstOrder, mdSecondTrigger, mdSecondOrder float64
		mdConfirmHigh, mdExec                                               bool
	)
	modifyCmd := &cobra.Command{
		Use:         "modify <conditional-order-id>",
		Short:       i18n.T("order.conditional.modify.short"),
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"source": "official", "mutating": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(opts)
			if err != nil {
				return err
			}
			intent := orderintent.ConditionalModifyIntent{
				ID: args[0], Type: mdType, OrderType: mdOrderType, ExpireDate: mdExpire,
				Quantity: mdQty, ConfirmHighValue: mdConfirmHigh,
				First: orderintent.ConditionLeg{OrderSide: mdFirstSide, TriggerPrice: mdFirstTrigger, OrderPrice: mdFirstOrder},
			}
			if mdType == "OCO" || mdType == "OTO" {
				if mdSecondSide == "" {
					return fmt.Errorf("--second-side/--second-trigger required for %s", mdType)
				}
				intent.Second = &orderintent.ConditionLeg{OrderSide: mdSecondSide, TriggerPrice: mdSecondTrigger, OrderPrice: mdSecondOrder}
			}
			canonical := orderintent.CanonicalConditionalModify(intent)
			gateErr := conditionalGate(app.config, canonical, mdExec, mdConfirm)
			if gateErr == errConditionalPreviewOnly {
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s x%s\n%s: %s\n",
					intent.ID, intent.Type, strconv.FormatFloat(intent.Quantity, 'f', -1, 64),
					i18n.T("order.conditional.confirmToken"), orderintent.ConfirmToken(canonical))
				return nil
			}
			if gateErr != nil {
				return gateErr
			}
			if err := app.client.ModifyConditionalOrder(cmd.Context(), intent); err != nil {
				return userFacingCommandError(err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), i18n.T("order.conditional.modify.done"))
			return nil
		},
	}
	modifyCmd.Flags().StringVar(&mdType, "type", "SINGLE", "SINGLE|OCO|OTO")
	modifyCmd.Flags().Float64Var(&mdQty, "qty", 0, "quantity (required)")
	modifyCmd.Flags().StringVar(&mdOrderType, "order-type", "LIMIT", "LIMIT|MARKET")
	modifyCmd.Flags().StringVar(&mdExpire, "expire", "", "expire date YYYY-MM-DD (required)")
	modifyCmd.Flags().StringVar(&mdFirstSide, "first-side", "", "first leg side BUY|SELL")
	modifyCmd.Flags().Float64Var(&mdFirstTrigger, "first-trigger", 0, "first leg trigger price")
	modifyCmd.Flags().Float64Var(&mdFirstOrder, "first-order-price", 0, "first leg limit price")
	modifyCmd.Flags().StringVar(&mdSecondSide, "second-side", "", "second leg side (OCO/OTO)")
	modifyCmd.Flags().Float64Var(&mdSecondTrigger, "second-trigger", 0, "second leg trigger price")
	modifyCmd.Flags().Float64Var(&mdSecondOrder, "second-order-price", 0, "second leg limit price")
	modifyCmd.Flags().BoolVar(&mdConfirmHigh, "confirm-high-value", false, "consent for orders >= 1억원")
	modifyCmd.Flags().BoolVar(&mdExec, "execute", false, "actually modify (omit for preview + confirm token)")
	modifyCmd.Flags().StringVar(&mdConfirm, "confirm", "", "confirm token from the preview")

	cmd.AddCommand(listCmd, getCmd, cancelCmd, placeCmd, modifyCmd)
	return cmd
}
