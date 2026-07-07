package main

import (
	"crypto/subtle"
	"fmt"

	"github.com/JungHoonGhae/tossinvest-cli/internal/config"
	"github.com/JungHoonGhae/tossinvest-cli/internal/orderintent"
)

// conditionalGate enforces the same safety model as order place/cancel/amend:
// config must enable conditional live actions, --execute must be set, and
// --confirm must match the canonical confirm token. Returns nil when the
// mutation may proceed; a user-facing error otherwise. When execute is false it
// returns errConditionalPreviewOnly so the caller renders a preview + token.
var errConditionalPreviewOnly = fmt.Errorf("preview only")

func conditionalGate(cfg config.File, canonical string, execute bool, confirm string) error {
	if !cfg.Trading.Conditional || !cfg.Trading.AllowLiveOrderActions {
		return fmt.Errorf("conditional live actions are disabled; set `trading.conditional=true` and `trading.allow_live_order_actions=true` in config.json")
	}
	if !execute {
		return errConditionalPreviewOnly
	}
	token := orderintent.ConfirmToken(canonical)
	if subtle.ConstantTimeCompare([]byte(confirm), []byte(token)) != 1 {
		return fmt.Errorf("confirmation token mismatch; rerun without --execute to get the current --confirm token")
	}
	return nil
}
