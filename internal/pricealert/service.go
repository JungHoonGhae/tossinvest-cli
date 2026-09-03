// Package pricealert owns the safe preview/confirm/verify workflow for Toss
// Securities target-price alerts.
package pricealert

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/JungHoonGhae/tossinvest-cli/internal/confirmation"
	"github.com/JungHoonGhae/tossinvest-cli/internal/domain"
)

const maxAlerts = 10

type Action string

const (
	ActionAdd    Action = "add"
	ActionRemove Action = "remove"
)

type Client interface {
	ResolveProductCode(context.Context, string) (string, error)
	ListPriceAlerts(context.Context, string) (domain.PriceAlerts, error)
	AddPriceAlert(context.Context, string, float64, string) error
	DeletePriceAlert(context.Context, string, float64, string) error
}

type ExecuteOptions struct {
	Execute bool
	Confirm string
}

type Plan struct {
	Kind          string              `json:"kind"`
	Action        Action              `json:"action"`
	ProductCode   string              `json:"product_code"`
	TargetPrice   float64             `json:"target_price"`
	Currency      string              `json:"currency"`
	CurrentAlerts []domain.PriceAlert `json:"current_alerts"`
	Noop          bool                `json:"noop"`
	Applied       bool                `json:"applied"`
	Reconciled    bool                `json:"reconciled,omitempty"`
	Canonical     string              `json:"canonical"`
	ConfirmToken  string              `json:"confirm_token"`
}

type Service struct {
	client Client
}

func NewService(client Client) *Service {
	return &Service{client: client}
}

func (s *Service) List(ctx context.Context, symbol string) (domain.PriceAlerts, error) {
	if s == nil || s.client == nil {
		return domain.PriceAlerts{}, fmt.Errorf("price alert manager is not configured")
	}
	productCode, err := s.client.ResolveProductCode(ctx, symbol)
	if err != nil {
		return domain.PriceAlerts{}, err
	}
	alerts, err := s.client.ListPriceAlerts(ctx, productCode)
	if err != nil {
		return domain.PriceAlerts{}, err
	}
	alerts.Alerts = normalizeAlerts(alerts.Alerts)
	return alerts, nil
}

func (s *Service) Change(ctx context.Context, action Action, symbol string, targetPrice float64, currency string, opts ExecuteOptions) (Plan, error) {
	if s == nil || s.client == nil {
		return Plan{}, fmt.Errorf("price alert manager is not configured")
	}
	if action != ActionAdd && action != ActionRemove {
		return Plan{}, fmt.Errorf("unsupported price alert action %q", action)
	}
	if math.IsNaN(targetPrice) || math.IsInf(targetPrice, 0) || targetPrice <= 0 {
		return Plan{}, fmt.Errorf("target price must be a finite number greater than zero")
	}
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency != "KRW" && currency != "USD" {
		return Plan{}, fmt.Errorf("currency must be KRW or USD")
	}

	plan, err := s.buildPlan(ctx, action, symbol, targetPrice, currency)
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
	if action == ActionAdd {
		mutateErr = s.client.AddPriceAlert(ctx, plan.ProductCode, targetPrice, currency)
	} else {
		mutateErr = s.client.DeletePriceAlert(ctx, plan.ProductCode, targetPrice, currency)
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
		return Plan{}, fmt.Errorf("%s price alert: %w; post-write verification did not observe the requested state", action, mutateErr)
	}
	if verifyErr != nil {
		return Plan{}, fmt.Errorf("verify %s price alert: %w", action, verifyErr)
	}
	return Plan{}, fmt.Errorf("verify %s price alert: requested state was not observed", action)
}

func (s *Service) buildPlan(ctx context.Context, action Action, symbol string, targetPrice float64, currency string) (Plan, error) {
	current, err := s.List(ctx, symbol)
	if err != nil {
		return Plan{}, err
	}
	present := containsAlert(current.Alerts, targetPrice, currency)
	if action == ActionAdd && !present && len(current.Alerts) >= maxAlerts {
		return Plan{}, fmt.Errorf("price alerts are limited to %d per stock", maxAlerts)
	}
	plan := Plan{
		Kind:          "securities_price_alert_change",
		Action:        action,
		ProductCode:   current.ProductCode,
		TargetPrice:   targetPrice,
		Currency:      currency,
		CurrentAlerts: current.Alerts,
		Noop:          (action == ActionAdd && present) || (action == ActionRemove && !present),
	}
	encoded, err := json.Marshal(struct {
		Kind          string              `json:"kind"`
		Action        Action              `json:"action"`
		ProductCode   string              `json:"product_code"`
		TargetPrice   float64             `json:"target_price"`
		Currency      string              `json:"currency"`
		CurrentAlerts []domain.PriceAlert `json:"current_alerts"`
		Noop          bool                `json:"noop"`
	}{plan.Kind, plan.Action, plan.ProductCode, plan.TargetPrice, plan.Currency, plan.CurrentAlerts, plan.Noop})
	if err != nil {
		return Plan{}, err
	}
	plan.Canonical = string(encoded)
	plan.ConfirmToken = confirmation.Token(plan.Canonical)
	return plan, nil
}

func (s *Service) desiredState(ctx context.Context, plan Plan) (bool, error) {
	current, err := s.client.ListPriceAlerts(ctx, plan.ProductCode)
	if err != nil {
		return false, err
	}
	present := containsAlert(current.Alerts, plan.TargetPrice, plan.Currency)
	return (plan.Action == ActionAdd && present) || (plan.Action == ActionRemove && !present), nil
}

func containsAlert(alerts []domain.PriceAlert, price float64, currency string) bool {
	return slices.ContainsFunc(alerts, func(alert domain.PriceAlert) bool {
		return alert.TargetPrice == price && strings.EqualFold(alert.Currency, currency)
	})
}

func normalizeAlerts(alerts []domain.PriceAlert) []domain.PriceAlert {
	result := append([]domain.PriceAlert(nil), alerts...)
	for i := range result {
		result[i].Currency = strings.ToUpper(strings.TrimSpace(result[i].Currency))
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Currency != result[j].Currency {
			return result[i].Currency < result[j].Currency
		}
		return result[i].TargetPrice < result[j].TargetPrice
	})
	return result
}
