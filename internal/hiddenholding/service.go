// Package hiddenholding owns the safe preview/confirm/verify workflow for
// holdings hidden from a Toss Securities portfolio.
package hiddenholding

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/confirmation"
	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

type Action string

const (
	ActionHide Action = "hide"
	ActionShow Action = "show"
)

type Client interface {
	ResolveProductCode(context.Context, string) (string, error)
	ListHiddenHoldings(context.Context, string) (domain.HiddenHoldings, error)
	HideHolding(context.Context, string, string) error
	ShowHolding(context.Context, string, string) error
}

type ExecuteOptions struct {
	Execute bool
	Confirm string
}

type Plan struct {
	Kind         string `json:"kind"`
	Action       Action `json:"action"`
	ProductCode  string `json:"product_code"`
	AccountScope string `json:"account_scope"`
	Noop         bool   `json:"noop"`
	Applied      bool   `json:"applied"`
	Reconciled   bool   `json:"reconciled,omitempty"`
	Canonical    string `json:"canonical"`
	ConfirmToken string `json:"confirm_token"`
	accountKey   string
}

type Service struct {
	client Client
}

func NewService(client Client) *Service {
	return &Service{client: client}
}

func (s *Service) List(ctx context.Context, accountKey string) (domain.HiddenHoldings, error) {
	if s == nil || s.client == nil {
		return domain.HiddenHoldings{}, fmt.Errorf("hidden holding manager is not configured")
	}
	result, err := s.client.ListHiddenHoldings(ctx, strings.TrimSpace(accountKey))
	if err != nil {
		return domain.HiddenHoldings{}, err
	}
	result.AccountScope = confirmation.Token("account:" + result.AccountKey)
	sort.Slice(result.Holdings, func(i, j int) bool { return result.Holdings[i].ProductCode < result.Holdings[j].ProductCode })
	return result, nil
}

func (s *Service) Change(ctx context.Context, action Action, symbol, accountKey string, opts ExecuteOptions) (Plan, error) {
	if s == nil || s.client == nil {
		return Plan{}, fmt.Errorf("hidden holding manager is not configured")
	}
	if action != ActionHide && action != ActionShow {
		return Plan{}, fmt.Errorf("unsupported hidden holding action %q", action)
	}
	productCode, err := s.client.ResolveProductCode(ctx, symbol)
	if err != nil {
		return Plan{}, err
	}
	plan, err := s.buildPlan(ctx, action, productCode, accountKey)
	if err != nil {
		return Plan{}, err
	}
	if !opts.Execute {
		return plan, nil
	}
	if !confirmation.Matches(opts.Confirm, plan.ConfirmToken) {
		return Plan{}, fmt.Errorf("confirmation token mismatch; preview again and pass its confirm_token")
	}
	if plan.Noop {
		plan.Applied = true
		return plan, nil
	}

	var mutateErr error
	if action == ActionHide {
		mutateErr = s.client.HideHolding(ctx, plan.accountKey, productCode)
	} else {
		mutateErr = s.client.ShowHolding(ctx, plan.accountKey, productCode)
	}
	verifyCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	applied, verifyErr := s.desiredState(verifyCtx, plan)
	if applied {
		plan.Applied = true
		plan.Reconciled = mutateErr != nil
		return plan, nil
	}
	if mutateErr != nil {
		return Plan{}, fmt.Errorf("%s hidden holding: %w; post-write verification did not observe the requested state", action, mutateErr)
	}
	if verifyErr != nil {
		return Plan{}, fmt.Errorf("verify %s hidden holding: %w", action, verifyErr)
	}
	return Plan{}, fmt.Errorf("verify %s hidden holding: requested state was not observed", action)
}

func (s *Service) buildPlan(ctx context.Context, action Action, productCode, accountKey string) (Plan, error) {
	current, err := s.List(ctx, accountKey)
	if err != nil {
		return Plan{}, err
	}
	present := containsHolding(current.Holdings, productCode)
	plan := Plan{
		Kind:         "securities_hidden_holding_change",
		Action:       action,
		ProductCode:  productCode,
		AccountScope: current.AccountScope,
		Noop:         (action == ActionHide && present) || (action == ActionShow && !present),
		accountKey:   current.AccountKey,
	}
	encoded, err := json.Marshal(struct {
		Kind         string   `json:"kind"`
		Action       Action   `json:"action"`
		ProductCode  string   `json:"product_code"`
		AccountScope string   `json:"account_scope"`
		HiddenCodes  []string `json:"hidden_codes"`
		Noop         bool     `json:"noop"`
	}{plan.Kind, plan.Action, plan.ProductCode, plan.AccountScope, holdingCodes(current.Holdings), plan.Noop})
	if err != nil {
		return Plan{}, err
	}
	plan.Canonical = string(encoded)
	plan.ConfirmToken = confirmation.Token(plan.Canonical)
	return plan, nil
}

func (s *Service) desiredState(ctx context.Context, plan Plan) (bool, error) {
	current, err := s.client.ListHiddenHoldings(ctx, plan.accountKey)
	if err != nil {
		return false, err
	}
	present := containsHolding(current.Holdings, plan.ProductCode)
	return (plan.Action == ActionHide && present) || (plan.Action == ActionShow && !present), nil
}

func containsHolding(holdings []domain.HiddenHolding, productCode string) bool {
	for _, holding := range holdings {
		if holding.ProductCode == productCode {
			return true
		}
	}
	return false
}

func holdingCodes(holdings []domain.HiddenHolding) []string {
	codes := make([]string, 0, len(holdings))
	for _, holding := range holdings {
		codes = append(codes, holding.ProductCode)
	}
	sort.Strings(codes)
	return codes
}
