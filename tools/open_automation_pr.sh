#!/usr/bin/env bash
set -euo pipefail

: "${GH_TOKEN:?GH_TOKEN is required}"
: "${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is required}"
: "${GITHUB_RUN_ID:?GITHUB_RUN_ID is required}"

if [ "$#" -lt 2 ] || [ "$#" -gt 3 ]; then
  echo "usage: open_automation_pr.sh <branch-prefix> <title> [body]" >&2
  exit 2
fi

branch_prefix="$1"
title="$2"
body="${3:-Automated repository maintenance. CI must pass before this PR can merge.}"

if [[ ! "$branch_prefix" =~ ^automation/[a-z0-9-]+$ ]]; then
  echo "branch prefix must match automation/[a-z0-9-]+" >&2
  exit 2
fi

if [ "$(git rev-list --count "origin/main..HEAD")" -eq 0 ]; then
  echo "No automation commits to publish."
  exit 0
fi

branch="${branch_prefix}-${GITHUB_RUN_ID}-${GITHUB_RUN_ATTEMPT:-1}"
git switch -c "$branch"
git push --set-upstream origin "$branch"

pr_url="$(gh pr create \
  --repo "$GITHUB_REPOSITORY" \
  --base main \
  --head "$branch" \
  --title "$title" \
  --body "$body")"

# Events emitted by GITHUB_TOKEN do not recursively trigger ordinary push/PR
# workflows. workflow_dispatch is the documented exception, so explicitly run
# CI for this exact head commit before enabling protected-branch auto-merge.
gh workflow run ci.yml --repo "$GITHUB_REPOSITORY" --ref "$branch"
gh pr merge "$pr_url" --repo "$GITHUB_REPOSITORY" --auto --squash --delete-branch

printf 'Opened %s and queued protected auto-merge.\n' "$pr_url"
