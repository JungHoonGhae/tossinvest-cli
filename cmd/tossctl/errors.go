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
			return fmt.Errorf("broker rejected order preparation before submission: %s\n1. Toss app/web에서 잔액, 환전 동의, 또는 broker prompt를 확인합니다.\n2. 준비가 끝나면 `%s`를 다시 실행합니다.\n3. 새 confirm token으로 `tossctl order place ... --execute --confirm <new-confirm-token>`를 다시 실행합니다.", message, previewCommand)
		}
		return fmt.Errorf("broker rejected order preparation before submission.\n1. Toss app/web에서 잔액, 환전 동의, 또는 broker prompt를 확인합니다.\n2. 준비가 끝나면 `%s`를 다시 실행합니다.\n3. 새 confirm token으로 `tossctl order place ... --execute --confirm <new-confirm-token>`를 다시 실행합니다.", previewCommand)
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
		return fmt.Errorf("주문 준비 단계에서 잔액 또는 주문가능금액이 부족해 진행이 중단되었습니다.%s\n1. Toss 앱 또는 웹에서 주문가능금액을 채웁니다.\n2. 필요한 경우 원화 입금 또는 계좌 충전을 완료합니다.\n3. 완료 후 `%s`를 다시 실행해 새 confirm token을 받습니다.\nRetry: `%s`", messageSuffix, previewCommand, placeCommand)
	case trading.BranchFXConsentRequired:
		if branchRequired.Source == trading.BranchSourcePostPrepareConfirmation {
			return formatPostPrepareFXBranchError(branchRequired)
		}
		return fmt.Errorf("주문 준비 단계에서 환전 또는 외화 사용 동의가 필요해 진행이 중단되었습니다.%s\n1. Toss 앱 또는 웹에서 해당 미국주식 주문의 환전 또는 외화 사용 동의 화면으로 이동합니다.\n2. 환전 또는 외화 사용 동의를 완료합니다.\n3. 완료 후 `%s`를 다시 실행해 새 confirm token을 받습니다.\nRetry: `%s`", messageSuffix, previewCommand, placeCommand)
	default:
		if messageSuffix == "" {
			messageSuffix = "\nBroker message: unavailable"
		}
		return fmt.Errorf("broker requires operator action before the order can continue.%s\n1. Toss 앱 또는 웹에서 필요한 안내를 완료합니다.\n2. 완료 후 `%s`를 다시 실행해 새 confirm token을 받습니다.\n3. 새 confirm token으로 `%s`를 다시 실행합니다.", messageSuffix, previewCommand, placeCommand)
	}
}

func formatPostPrepareFXBranchError(branchRequired *trading.BranchRequiredError) error {
	lines := []string{
		"주문 준비는 통과했지만, 웹과 동일한 환전 확인 단계에서 중단되었습니다.",
	}

	if fx := branchRequired.FX; fx != nil {
		if fx.NeedExchangeUSD > 0 {
			lines = append(lines, fmt.Sprintf("%s달러가 부족해요.", formatDisplayDecimal(fx.NeedExchangeUSD, 2)))
		}
		lines = append(lines, "주식 구매를 위해 환전할게요.")
		if fx.EstimatedExchangeKRW > 0 {
			lines = append(lines, fmt.Sprintf("예상 환전 금액: %s원", formatGroupedInt(int64(math.Round(fx.EstimatedExchangeKRW)))))
		}
		if fx.USDExchangeRate > 0 {
			lines = append(lines, fmt.Sprintf("예상 환율: %s원/USD", formatGroupedFixed(fx.USDExchangeRate, 2)))
		}
	}

	if message := strings.TrimSpace(branchRequired.BrokerMessage); message != "" {
		lines = append(lines, "Broker message: "+message)
	}

	lines = append(lines,
		"주의: 주문이 취소되면 계좌에는 달러로 남아있어요.",
		"1. Toss 앱 또는 웹에서 같은 미국주식 주문 화면으로 이동합니다.",
		"2. 환전 확인 화면에서 주문을 계속 진행할지 결정합니다.",
		"3. 기본 동작은 여기서 중단입니다. 자동으로 계속 진행하려면 `trading.dangerous_automation.accept_fx_consent=true`를 설정한 뒤 다시 시도합니다.",
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
