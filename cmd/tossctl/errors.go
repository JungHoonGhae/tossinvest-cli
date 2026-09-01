package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/auth"
	tossclient "github.com/JungHoonGhae/tossinvest-cli/internal/client"
	"github.com/JungHoonGhae/tossinvest-cli/internal/config"
	"github.com/JungHoonGhae/tossinvest-cli/internal/hybrid"
	"github.com/JungHoonGhae/tossinvest-cli/internal/orderintent"

	"github.com/JungHoonGhae/tossinvest-cli/internal/session"
	"github.com/JungHoonGhae/tossinvest-cli/internal/trading"
)

func userFacingCommandError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("canceled")
	}

	if errors.Is(err, hybrid.ErrOfficialAccessRequired) {
		return fmt.Errorf("this feature requires Toss official Open API access\n  connect a key with `tossctl openapi login` and use `--backend auto|openapi`\n  (issue a key: https://corp.tossinvest.com/ko/open-api)")
	}

	if errors.Is(err, session.ErrNoSession) || errors.Is(err, tossclient.ErrNoSession) {
		return fmt.Errorf("no active session; run `tossctl auth login`")
	}

	if tossclient.IsAuthError(err) {
		return fmt.Errorf("stored session is no longer valid; run `tossctl auth extend` to renew, or `tossctl auth login` to re-authenticate")
	}
	if errors.Is(err, auth.ErrExtensionTimeout) {
		// Pull the elapsed-time detail from the typed wrapper so we don't have
		// to scrape the error string for "(waited Ns)".
		var timeout *auth.ExtensionTimeoutError
		if errors.As(err, &timeout) {
			return fmt.Errorf("phone approval did not complete (waited %s); rerun `tossctl auth extend` to retry", timeout.Elapsed.Round(time.Second))
		}
		return fmt.Errorf("phone approval did not complete; rerun `tossctl auth extend` to retry")
	}
	if errors.Is(err, auth.ErrExtensionRejected) {
		return fmt.Errorf("phone approval was denied or canceled; rerun `tossctl auth extend` to retry")
	}
	if errors.Is(err, auth.ErrExtensionNotConfigured) {
		return fmt.Errorf("internal error: ExtensionRunner is not configured")
	}
	if errors.Is(err, trading.ErrExecuteRequired) {
		return fmt.Errorf("live trading is blocked by default; rerun with `--execute` after reviewing `tossctl order preview`")
	}
	if errors.Is(err, trading.ErrConfirmMismatch) {
		return fmt.Errorf("confirmation token mismatch; rerun `tossctl order preview` and pass the new `--confirm` token")
	}
	if errors.Is(err, trading.ErrLiveMutationPending) {
		return fmt.Errorf("safety gates passed, but live trading mutation wiring is not implemented yet")
	}
	if errors.Is(err, trading.ErrPlaceUnsupported) {
		return fmt.Errorf("live place supports `--market us|kr --type limit` and `--market us --fractional` (market order) in KRW")
	}
	if errors.Is(err, trading.ErrPlaceNotReconciled) {
		return fmt.Errorf("place mutation returned, but the new order was not found in pending reconciliation; check `tossctl orders list` and completed history before retrying")
	}
	if errors.Is(err, trading.ErrCancelStillPending) {
		return fmt.Errorf("cancel mutation returned, but the order is still pending; reconcile with `tossctl orders list` before retrying")
	}
	if errors.Is(err, trading.ErrInteractiveAuthRequired) {
		return fmt.Errorf("broker requested interactive trade authentication; complete the trade action in the web app and keep the browser session open")
	}

	return err
}

func userFacingTradingError(paths config.Paths, err error) error {
	if err == nil {
		return nil
	}

	var branchRequired *trading.BranchRequiredError
	if errors.As(err, &branchRequired) {
		return formatBranchRequiredError(branchRequired, nil)
	}

	var prepareRejected *trading.PrepareRejectedError
	if errors.As(err, &prepareRejected) {
		if message := strings.TrimSpace(prepareRejected.BrokerMessage); message != "" {
			return fmt.Errorf("broker rejected order preparation before submission: %s", message)
		}
		return fmt.Errorf("broker rejected order preparation before submission; review balance, FX consent, or broker prompts in the app/web and retry from `tossctl order preview`")
	}

	var disabled *trading.DisabledActionError
	if errors.As(err, &disabled) {
		return fmt.Errorf("trading action `%s` is disabled; run `tossctl config init` if needed and update %s", disabled.Action, paths.ConfigFile)
	}
	if errors.Is(err, trading.ErrLiveActionsDisabled) {
		return fmt.Errorf("live order actions are disabled; set `trading.allow_live_order_actions=true` in %s", paths.ConfigFile)
	}

	return userFacingCommandError(err)
}

func userFacingPlaceError(paths config.Paths, err error, intent *orderintent.PlaceIntent) error {
	if err == nil {
		return nil
	}

	var branchRequired *trading.BranchRequiredError
	if errors.As(err, &branchRequired) {
		return formatBranchRequiredError(branchRequired, intent)
	}

	var prepareRejected *trading.PrepareRejectedError
	if errors.As(err, &prepareRejected) {
		previewCommand := "tossctl order preview ..."
		if intent != nil {
			previewCommand = buildPlaceCommand("preview", intent, "")
		}
		if message := strings.TrimSpace(prepareRejected.BrokerMessage); message != "" {
			return fmt.Errorf("broker rejected order preparation before submission: %s\n1. Check balance, FX consent, or broker prompts in the Toss app/web.\n2. Once ready, rerun `%s`.\n3. Rerun with a new confirm token: `tossctl order place ... --execute --confirm <new-confirm-token>`.", message, previewCommand)
		}
		return fmt.Errorf("broker rejected order preparation before submission.\n1. Check balance, FX consent, or broker prompts in the Toss app/web.\n2. Once ready, rerun `%s`.\n3. Rerun with a new confirm token: `tossctl order place ... --execute --confirm <new-confirm-token>`.", previewCommand)
	}

	return userFacingTradingError(paths, err)
}

func formatBranchRequiredError(branchRequired *trading.BranchRequiredError, intent *orderintent.PlaceIntent) error {
	if branchRequired == nil {
		return fmt.Errorf("broker requires operator action")
	}

	previewCommand := "tossctl order preview ..."
	placeCommand := "tossctl order place ... --execute --confirm <new-confirm-token>"
	if intent != nil {
		previewCommand = buildPlaceCommand("preview", intent, "")
		placeCommand = buildPlaceCommand("place", intent, "<new-confirm-token>")
	}

	messageSuffix := ""
	if message := strings.TrimSpace(branchRequired.BrokerMessage); message != "" {
		messageSuffix = "\nBroker message: " + message
	}

	switch branchRequired.Branch {
	case trading.BranchFundingRequired:
		return fmt.Errorf("order preparation stopped because balance or orderable amount is insufficient.%s\n1. Top up the orderable amount in the Toss app or web.\n2. If needed, complete a KRW deposit or account top-up.\n3. Once done, rerun `%s` to get a new confirm token.\nRetry: `%s`", messageSuffix, previewCommand, placeCommand)
	case trading.BranchFXConsentRequired:
		if branchRequired.Source == trading.BranchSourcePostPrepareConfirmation {
			return formatPostPrepareFXBranchError(branchRequired)
		}
		return fmt.Errorf("order preparation stopped because FX exchange or foreign-currency usage consent is required.%s\n1. In the Toss app or web, go to the FX exchange / foreign-currency consent screen for this US stock order.\n2. Complete the FX exchange or foreign-currency consent.\n3. Once done, rerun `%s` to get a new confirm token.\nRetry: `%s`", messageSuffix, previewCommand, placeCommand)
	default:
		if messageSuffix == "" {
			messageSuffix = "\nBroker message: unavailable"
		}
		return fmt.Errorf("broker requires operator action before the order can continue.%s\n1. Complete the required guidance in the Toss app or web.\n2. Once done, rerun `%s` to get a new confirm token.\n3. Rerun `%s` with the new confirm token.", messageSuffix, previewCommand, placeCommand)
	}
}

func formatPostPrepareFXBranchError(branchRequired *trading.BranchRequiredError) error {
	lines := []string{
		"Order preparation passed, but was stopped at the same FX confirmation step as the web.",
	}

	if fx := branchRequired.FX; fx != nil {
		if fx.NeedExchangeUSD > 0 {
			lines = append(lines, fmt.Sprintf("Short by $%s.", formatDisplayDecimal(fx.NeedExchangeUSD, 2)))
		}
		lines = append(lines, "We'll exchange currency to purchase the stock.")
		if fx.EstimatedExchangeKRW > 0 {
			lines = append(lines, fmt.Sprintf("Estimated exchange amount: %s KRW", formatGroupedInt(int64(math.Round(fx.EstimatedExchangeKRW)))))
		}
		if fx.USDExchangeRate > 0 {
			lines = append(lines, fmt.Sprintf("Estimated exchange rate: %s KRW/USD", formatGroupedFixed(fx.USDExchangeRate, 2)))
		}
	}

	if message := strings.TrimSpace(branchRequired.BrokerMessage); message != "" {
		lines = append(lines, "Broker message: "+message)
	}

	lines = append(lines,
		"Note: if the order is canceled, the funds will remain in the account as USD.",
		"1. Go to the same US stock order screen in the Toss app or web.",
		"2. Decide whether to proceed with the order on the FX confirmation screen.",
		"3. The default behavior stops here. To proceed automatically, set `trading.dangerous_automation.accept_fx_consent=true` and retry.",
	)

	return fmt.Errorf("%s", strings.Join(lines, "\n"))
}

// buildPlaceCommand reconstructs a retry command from the NORMALIZED intent
// (not the raw flags) so auto-detected fields are accurate — e.g. a KR symbol
// shows `--market kr`, not the flag default `--market us`. It also emits the
// flags that match the order shape: fractional buys are amount-based (--amount,
// no --qty/--price), limits carry --price, market orders omit it.
func buildPlaceCommand(kind string, intent *orderintent.PlaceIntent, confirm string) string {
	if intent == nil {
		if kind == "preview" {
			return "tossctl order preview ..."
		}
		return "tossctl order place ... --execute --confirm " + confirm
	}

	args := []string{
		"tossctl",
		"order",
		kind,
		"--symbol", intent.Symbol,
		"--market", intent.Market,
		"--side", intent.Side,
		"--type", intent.OrderType,
	}
	if intent.Fractional && intent.Side == "buy" {
		args = append(args, "--amount", formatCommandFloat(intent.Amount))
	} else {
		args = append(args, "--qty", formatCommandFloat(intent.Quantity))
		if intent.OrderType == "limit" {
			args = append(args, "--price", formatCommandFloat(intent.Price))
		}
	}
	if intent.CurrencyMode != "" {
		args = append(args, "--currency-mode", intent.CurrencyMode)
	}
	if intent.Fractional {
		args = append(args, "--fractional")
	}
	if kind == "place" {
		args = append(args, "--execute", "--confirm", confirm)
	}
	return strings.Join(args, " ")
}

func formatCommandFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func formatDisplayDecimal(value float64, decimals int) string {
	text := strconv.FormatFloat(value, 'f', decimals, 64)
	text = strings.TrimRight(text, "0")
	text = strings.TrimRight(text, ".")
	if text == "" {
		return "0"
	}
	return text
}

func formatGroupedInt(value int64) string {
	negative := value < 0
	if negative {
		value = -value
	}

	text := strconv.FormatInt(value, 10)
	for i := len(text) - 3; i > 0; i -= 3 {
		text = text[:i] + "," + text[i:]
	}
	if negative {
		return "-" + text
	}
	return text
}

func formatGroupedFixed(value float64, decimals int) string {
	text := strconv.FormatFloat(value, 'f', decimals, 64)
	parts := strings.SplitN(text, ".", 2)
	grouped := formatGroupedIntString(parts[0])
	if len(parts) == 1 {
		return grouped
	}
	return grouped + "." + parts[1]
}

func formatGroupedIntString(value string) string {
	if value == "" {
		return "0"
	}

	negative := strings.HasPrefix(value, "-")
	if negative {
		value = strings.TrimPrefix(value, "-")
	}
	for i := len(value) - 3; i > 0; i -= 3 {
		value = value[:i] + "," + value[i:]
	}
	if negative {
		return "-" + value
	}
	return value
}
