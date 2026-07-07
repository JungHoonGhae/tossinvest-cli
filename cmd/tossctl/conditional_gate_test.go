package main

import (
	"errors"
	"testing"

	"github.com/JungHoonGhae/tossinvest-cli/internal/config"
	"github.com/JungHoonGhae/tossinvest-cli/internal/orderintent"
)

func TestConditionalGate(t *testing.T) {
	canonical := orderintent.CanonicalConditionalCancel(orderintent.ConditionalCancelIntent{ID: "co-1"})
	token := orderintent.ConfirmToken(canonical)

	enabled := config.File{}
	enabled.Trading.Conditional = true
	enabled.Trading.AllowLiveOrderActions = true

	// 1. config disabled → refuse (not preview, not success)
	if err := conditionalGate(config.File{}, canonical, true, token); err == nil || errors.Is(err, errConditionalPreviewOnly) {
		t.Errorf("config disabled: want refusal error, got %v", err)
	}
	// 2. enabled but no --execute → preview sentinel
	if err := conditionalGate(enabled, canonical, false, ""); !errors.Is(err, errConditionalPreviewOnly) {
		t.Errorf("no execute: want errConditionalPreviewOnly, got %v", err)
	}
	// 3. enabled + execute + wrong confirm → mismatch error (not preview)
	if err := conditionalGate(enabled, canonical, true, "wrong-token"); err == nil || errors.Is(err, errConditionalPreviewOnly) {
		t.Errorf("wrong confirm: want mismatch error, got %v", err)
	}
	// 4. enabled + execute + correct confirm → nil (proceed)
	if err := conditionalGate(enabled, canonical, true, token); err != nil {
		t.Errorf("valid: want nil, got %v", err)
	}
}
