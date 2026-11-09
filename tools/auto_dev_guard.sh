#!/usr/bin/env bash
# Scope guard for unattended development (docs/automation/auto-dev.md).
# Fails when a change touches paths an agent must never change without a
# person: order/trading writes, CI and release, install/self-update, the auth
# helper, dependencies, and this automation itself. CI runs it for every
# auto-dev/* pull request, so a blocked change cannot merge.
#
# usage: auto_dev_guard.sh <base-ref> [head-ref]
set -euo pipefail
base="${1:?usage: auto_dev_guard.sh <base-ref> [head-ref]}"
head="${2:-HEAD}"

protected='^(internal/trading/|internal/orderintent/|internal/client/(trading|order_reconciler|order_funding)[^/]*\.go$|internal/official/(orders_write|conditional_writes)[^/]*\.go$|internal/hybrid/broker[^/]*\.go$|internal/ops/write_operations[^/]*\.go$|cmd/tossctl/order[^/]*\.go$|\.github/|install\.sh$|install\.ps1$|internal/selfupdate/|auth-helper/|go\.mod$|go\.sum$|Makefile$|renovate\.json$|website-fumadocs/package\.json$|website-fumadocs/pnpm-lock\.yaml$|tools/(approve_automation_ci|open_automation_pr|api_change_analysis)\.sh$|tools/auto_dev_|docs/automation/auto-dev\.md$)'

changed="$(git diff --name-only "$base...$head")"
blocked="$(printf '%s\n' "$changed" | grep -E "$protected" || true)"
if [ -n "$blocked" ]; then
  echo "auto-dev scope guard: these paths need a person, not unattended development:" >&2
  printf '  %s\n' $blocked >&2
  exit 1
fi
echo "auto-dev scope guard: ok ($(printf '%s\n' "$changed" | grep -c . || true) files)"
