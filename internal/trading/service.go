package trading

import (
	"context"
	"crypto/subtle"
	"fmt"
	"strings"

	"github.com/JungHoonGhae/tossinvest-cli/internal/config"
	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
	"github.com/JungHoonGhae/tossinvest-cli/internal/orderintent"
	"github.com/JungHoonGhae/tossinvest-cli/internal/orderlineage"
)

type Broker interface {
	PlacePendingOrder(ctx context.Context, intent orderintent.PlaceIntent) (MutationResult, error)
	GetOrderAvailableActions(ctx context.Context, orderID string) (map[string]any, error)
	CancelPendingOrder(ctx context.Context, intent orderintent.CancelIntent) (MutationResult, error)
	AmendPendingOrder(ctx context.Context, intent orderintent.AmendIntent) (MutationResult, error)
}

// ConditionalBroker executes conditional-order mutations after Service has
// applied the shared config, execute, and confirm-token guard.
type ConditionalBroker interface {
	CreateConditionalOrder(ctx context.Context, intent orderintent.ConditionalPlaceIntent) (domain.ConditionalOrderRef, error)
	CancelConditionalOrder(ctx context.Context, intent orderintent.ConditionalCancelIntent) error
	ModifyConditionalOrder(ctx context.Context, intent orderintent.ConditionalModifyIntent) error
}

type Preview struct {
	Kind          string   `json:"kind"`
	Canonical     string   `json:"canonical"`
	ConfirmToken  string   `json:"confirm_token"`
	Warnings      []string `json:"warnings,omitempty"`
	LiveReady     bool     `json:"live_ready"`
	MutationReady bool     `json:"mutation_ready"`
}

type ExecuteOptions struct {
	Execute bool
	Confirm string
}

// LineageRecorder remembers that an order was replaced by another, so a later
// cancel/amend can find the live order from the id the user originally used.
//
// It lives behind an interface here rather than in each surface because every
// caller of Place/Cancel/Amend needs it: the CLI used to record lineage itself
// while the MCP path silently did not, which meant agent-placed orders lost
// their trail. Recording alongside the guard keeps write policy and its
// bookkeeping in one module. Conditional-order mutations use this same Service
// for the same reason: every write surface must apply one safety policy.
type LineageRecorder interface {
	Record(originalOrderID string, entry orderlineage.Entry) error
}

type Service struct {
	policy      config.Trading
	broker      Broker
	conditional ConditionalBroker
	lineage     LineageRecorder // optional; nil disables recording
}

// WithConditionalBroker attaches the official-only conditional-order adapter.
func (s *Service) WithConditionalBroker(b ConditionalBroker) *Service {
	s.conditional = b
	return s
}

func NewService(policy config.Trading, broker Broker) *Service {
	return &Service{
		policy: policy,
		broker: broker,
	}
}

// WithLineage attaches the recorder. Optional: a Service without one behaves
// exactly as before, which keeps the many tests that construct a bare Service
// working and lets read-only callers skip the dependency.
func (s *Service) WithLineage(r LineageRecorder) *Service {
	s.lineage = r
	return s
}

// recordLineage is called after a successful mutation. A recording failure is
// reported on the result rather than failing the mutation — the order already
// went through, and losing the local trail must not look like a failed order.
func (s *Service) recordLineage(res MutationResult) MutationResult {
	if s.lineage == nil {
		return res
	}
	original := strings.TrimSpace(res.OriginalOrderID)
	if original == "" {
		return res
	}
	entry := orderlineage.Entry{
		CurrentOrderID: strings.TrimSpace(res.CurrentOrderID),
		Kind:           strings.TrimSpace(res.Kind),
		Symbol:         strings.TrimSpace(res.Symbol),
		Market:         strings.TrimSpace(res.Market),
		Quantity:       res.Quantity,
		Price:          res.Price,
		OrderDate:      strings.TrimSpace(res.OrderDate),
	}
	if err := s.lineage.Record(original, entry); err != nil {
		res.Warnings = append(res.Warnings,
			fmt.Sprintf("Could not update the local order-lineage cache: %v", err))
	}
	return res
}

// withLineage wraps a broker call so every mutation path records identically.
func (s *Service) withLineage(res MutationResult, err error) (MutationResult, error) {
	if err != nil {
		return res, err
	}
	return s.recordLineage(res), nil
}

func (s *Service) PreviewPlace(intent orderintent.PlaceIntent) Preview {
	canonical := orderintent.CanonicalPlace(intent)
	warnings := []string{
		"Live place supports US/KR buy/sell limit orders (US accepts KRW or USD price input) and US fractional (market) orders.",
		"US orders may still require funding, FX consent, or product-risk acknowledgement before submission.",
	}
	liveReady := placeIntentSupported(intent)
	if !s.policy.Place {
		warnings = append(warnings, "Config currently disables `order place`.")
	}
	if intent.Side == "sell" && !s.policy.Sell {
		warnings = append(warnings, "Config currently disables `order place --side sell`.")
	}
	if intent.Fractional && !s.policy.Fractional {
		warnings = append(warnings, "Config currently disables `order place --fractional`.")
	}
	if !s.policy.AllowLiveOrderActions {
		warnings = append(warnings, "Config currently disables live order actions.")
	}
	mutationReady := liveReady && s.policy.Place && s.policy.AllowLiveOrderActions
	if intent.Side == "sell" {
		mutationReady = mutationReady && s.policy.Sell
	}
	if intent.Fractional {
		mutationReady = mutationReady && s.policy.Fractional
	}
	return Preview{
		Kind:          "place",
		Canonical:     canonical,
		ConfirmToken:  orderintent.ConfirmToken(canonical),
		Warnings:      warnings,
		LiveReady:     liveReady,
		MutationReady: mutationReady,
	}
}

func (s *Service) PreviewCancel(intent orderintent.CancelIntent) Preview {
	canonical := orderintent.CanonicalCancel(intent)
	warnings := []string{"Single-order cancel is wired for same-day pending orders and still reconciles through pending history."}
	if !s.policy.Cancel {
		warnings = append(warnings, "Config currently disables `order cancel`.")
	}
	if !s.policy.AllowLiveOrderActions {
		warnings = append(warnings, "Config currently disables live order actions.")
	}
	return Preview{
		Kind:          "cancel",
		Canonical:     canonical,
		ConfirmToken:  orderintent.ConfirmToken(canonical),
		Warnings:      warnings,
		LiveReady:     true,
		MutationReady: s.policy.Cancel && s.policy.AllowLiveOrderActions,
	}
}

func (s *Service) PreviewAmend(intent orderintent.AmendIntent) Preview {
	canonical := orderintent.CanonicalAmend(intent)
	warnings := []string{
		"Amend reconciles against the surviving pending order record after mutation.",
		"Amend wiring exists, but the current beta slice still needs more live verification.",
	}
	if !s.policy.Amend {
		warnings = append(warnings, "Config currently disables `order amend`.")
	}
	if !s.policy.AllowLiveOrderActions {
		warnings = append(warnings, "Config currently disables live order actions.")
	}
	return Preview{
		Kind:          "amend",
		Canonical:     canonical,
		ConfirmToken:  orderintent.ConfirmToken(canonical),
		Warnings:      warnings,
		LiveReady:     true,
		MutationReady: s.policy.Amend && s.policy.AllowLiveOrderActions,
	}
}

func (s *Service) PreviewConditionalPlace(intent orderintent.ConditionalPlaceIntent) Preview {
	return s.previewConditional("conditional_place", orderintent.CanonicalConditionalPlace(intent))
}

func (s *Service) PreviewConditionalCancel(intent orderintent.ConditionalCancelIntent) Preview {
	return s.previewConditional("conditional_cancel", orderintent.CanonicalConditionalCancel(intent))
}

func (s *Service) PreviewConditionalModify(intent orderintent.ConditionalModifyIntent) Preview {
	return s.previewConditional("conditional_modify", orderintent.CanonicalConditionalModify(intent))
}

func (s *Service) previewConditional(kind, canonical string) Preview {
	warnings := []string{"Conditional orders use the official Open API only."}
	if !s.policy.Conditional {
		warnings = append(warnings, "Config currently disables conditional order actions.")
	}
	if !s.policy.AllowLiveOrderActions {
		warnings = append(warnings, "Config currently disables live order actions.")
	}
	return Preview{
		Kind:          kind,
		Canonical:     canonical,
		ConfirmToken:  orderintent.ConfirmToken(canonical),
		Warnings:      warnings,
		LiveReady:     true,
		MutationReady: s.policy.Conditional && s.policy.AllowLiveOrderActions,
	}
}

func (s *Service) Place(ctx context.Context, intent orderintent.PlaceIntent, opts ExecuteOptions) (MutationResult, error) {
	// 1. capability check
	if !placeIntentSupported(intent) {
		return MutationResult{}, ErrPlaceUnsupported
	}
	// 2. side policy
	if intent.Side == "sell" && !s.policy.Sell {
		return MutationResult{}, &DisabledActionError{Action: "sell"}
	}
	// 3. fractional policy
	if intent.Fractional && !s.policy.Fractional {
		return MutationResult{}, &DisabledActionError{Action: "fractional"}
	}
	// 4. execution guard (--execute, confirm token, allow_live)
	if err := s.guard(ctx, ActionPlace, s.PreviewPlace(intent), opts); err != nil {
		return MutationResult{}, err
	}
	// 5. broker call
	if s.broker == nil {
		return MutationResult{}, ErrLiveMutationPending
	}
	return s.withLineage(s.broker.PlacePendingOrder(ctx, intent))
}

func (s *Service) Cancel(ctx context.Context, intent orderintent.CancelIntent, opts ExecuteOptions) (MutationResult, error) {
	if err := s.guard(ctx, ActionCancel, s.PreviewCancel(intent), opts); err != nil {
		return MutationResult{}, err
	}
	if s.broker == nil {
		return MutationResult{}, ErrLiveMutationPending
	}

	if _, err := s.broker.GetOrderAvailableActions(ctx, intent.OrderID); err != nil {
		return MutationResult{}, err
	}
	return s.withLineage(s.broker.CancelPendingOrder(ctx, intent))
}

func (s *Service) Amend(ctx context.Context, intent orderintent.AmendIntent, opts ExecuteOptions) (MutationResult, error) {
	if err := s.guard(ctx, ActionAmend, s.PreviewAmend(intent), opts); err != nil {
		return MutationResult{}, err
	}
	if s.broker == nil {
		return MutationResult{}, ErrLiveMutationPending
	}
	if _, err := s.broker.GetOrderAvailableActions(ctx, intent.OrderID); err != nil {
		return MutationResult{}, err
	}
	return s.withLineage(s.broker.AmendPendingOrder(ctx, intent))
}

func (s *Service) PlaceConditional(ctx context.Context, intent orderintent.ConditionalPlaceIntent, opts ExecuteOptions) (domain.ConditionalOrderRef, error) {
	if err := s.guard(ctx, ActionConditional, s.PreviewConditionalPlace(intent), opts); err != nil {
		return domain.ConditionalOrderRef{}, err
	}
	if s.conditional == nil {
		return domain.ConditionalOrderRef{}, ErrLiveMutationPending
	}
	return s.conditional.CreateConditionalOrder(ctx, intent)
}

func (s *Service) CancelConditional(ctx context.Context, intent orderintent.ConditionalCancelIntent, opts ExecuteOptions) error {
	if err := s.guard(ctx, ActionConditional, s.PreviewConditionalCancel(intent), opts); err != nil {
		return err
	}
	if s.conditional == nil {
		return ErrLiveMutationPending
	}
	return s.conditional.CancelConditionalOrder(ctx, intent)
}

func (s *Service) ModifyConditional(ctx context.Context, intent orderintent.ConditionalModifyIntent, opts ExecuteOptions) error {
	if err := s.guard(ctx, ActionConditional, s.PreviewConditionalModify(intent), opts); err != nil {
		return err
	}
	if s.conditional == nil {
		return ErrLiveMutationPending
	}
	return s.conditional.ModifyConditionalOrder(ctx, intent)
}

func (s *Service) guard(ctx context.Context, action Action, preview Preview, opts ExecuteOptions) error {
	if err := s.requireActionEnabled(action); err != nil {
		return err
	}
	if !opts.Execute {
		return fmt.Errorf("%w; rerun with --execute after reviewing `tossctl order preview`", ErrExecuteRequired)
	}
	if !s.policy.AllowLiveOrderActions {
		return ErrLiveActionsDisabled
	}
	if subtle.ConstantTimeCompare([]byte(opts.Confirm), []byte(preview.ConfirmToken)) != 1 {
		return ErrConfirmMismatch
	}
	return nil
}

func (s *Service) requireActionEnabled(action Action) error {
	enabled := false
	switch action {
	case ActionPlace:
		enabled = s.policy.Place
	case ActionCancel:
		enabled = s.policy.Cancel
	case ActionAmend:
		enabled = s.policy.Amend
	case ActionConditional:
		enabled = s.policy.Conditional
	}
	if enabled {
		return nil
	}
	return &DisabledActionError{Action: action}
}

func placeIntentSupported(intent orderintent.PlaceIntent) bool {
	if intent.Market != "us" && intent.Market != "kr" {
		return false
	}
	if intent.Fractional {
		// fractional: US market + market order, KRW or USD
		return intent.Market == "us" && intent.OrderType == "market" &&
			(intent.CurrencyMode == "KRW" || intent.CurrencyMode == "USD")
	}
	// non-fractional: limit only. KR requires KRW. US accepts KRW
	// (converted to USD via exchange rate at request time) or USD
	// (sent to server as-is in the price field — see buildPlaceBody).
	if intent.OrderType != "limit" {
		return false
	}
	if intent.Market == "kr" {
		return intent.CurrencyMode == "KRW"
	}
	return intent.CurrencyMode == "KRW" || intent.CurrencyMode == "USD"
}
