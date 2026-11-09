#!/usr/bin/env bash
# Scope guard for unattended development (docs/automation/auto-dev.md).
# Fails when a change touches paths an agent must never change without a
# person: order/trading writes, CI and release, install/self-update, the auth
# helper, dependencies, and this automation itself. CI runs it for every
# auto-dev/* pull request, so a blocked change cannot merge.
#
# usage: auto_dev_guard.sh <base-ref> [head-ref]
#        auto_dev_guard.sh --files < paths   (one changed path per line, renames as both paths)
set -euo pipefail

protected='^(internal/trading/|internal/orderintent/|internal/client/(trading|order_reconciler|order_funding)[^/]*\.go$|internal/official/(orders_write|conditional_writes)[^/]*\.go$|internal/hybrid/broker[^/]*\.go$|internal/ops/write_operations[^/]*\.go$|cmd/tossctl/order[^/]*\.go$|\.github/|install\.sh$|install\.ps1$|internal/selfupdate/|auth-helper/|go\.mod$|go\.sum$|Makefile$|renovate\.json$|website-fumadocs/package\.json$|website-fumadocs/pnpm-lock\.yaml$|tools/(approve_automation_ci|open_automation_pr|api_change_analysis)\.sh$|tools/auto_dev_|docs/automation/auto-dev\.md$)'

if [ "${1:-}" = "--files" ]; then
  changed="$(cat)"
else
  base="${1:?usage: auto_dev_guard.sh <base-ref> [head-ref] | --files}"
  head="${2:-HEAD}"
  # --no-renames: a rename would otherwise list only the new path, so moving
  # a protected file elsewhere would slip past the check.
  # core.quotePath=false: otherwise non-ASCII paths come back quoted and
  # escaped ("internal/trading/\352..."), which the regex below would miss.
  changed="$(git -c core.quotePath=false diff --no-renames --name-only "$base...$head")"
fi
blocked="$(printf '%s\n' "$changed" | grep -E "$protected" || true)"
# Fail closed on anything still quoted (control characters, odd bytes): such a
# path cannot be judged reliably.
quoted="$(printf '%s\n' "$changed" | grep -E '^"' || true)"
if [ -n "$quoted" ]; then
  blocked="$(printf '%s\n%s' "$blocked" "$quoted" | grep -v '^$' || true)"
fi
if [ -n "$blocked" ]; then
  echo "auto-dev scope guard: these paths need a person, not unattended development:" >&2
  printf '  %s\n' $blocked >&2
  exit 1
fi
echo "auto-dev scope guard: ok ($(printf '%s\n' "$changed" | grep -c . || true) files)"
