#!/usr/bin/env bash
set -euo pipefail
: "${GH_TOKEN:?GH_TOKEN is required}"
: "${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is required}"
branch="$1"
head_sha="$2"
[[ "$branch" =~ ^automation/[a-z0-9-]+$ ]] || exit 2
[[ "$head_sha" =~ ^[a-f0-9]{40}$ ]] || exit 2

# GITHUB_TOKEN PR events create approval-required runs. Authorize the actual
# PR run at this SHA, never a detached workflow_dispatch substitute.
for _attempt in {1..30}; do
  run_id="$(gh run list --repo "$GITHUB_REPOSITORY" --workflow ci.yml \
    --branch "$branch" --event pull_request --limit 20 \
    --json databaseId,headSha \
    --jq ".[] | select(.headSha == \"$head_sha\") | .databaseId" | head -n 1)"
  if [ -n "$run_id" ]; then
    conclusion="$(gh run view "$run_id" --repo "$GITHUB_REPOSITORY" --json conclusion --jq .conclusion)"
    if [ "$conclusion" = action_required ]; then
      gh api --method POST "repos/$GITHUB_REPOSITORY/actions/runs/$run_id/approve"
      exit 0
    fi
    status="$(gh run view "$run_id" --repo "$GITHUB_REPOSITORY" --json status --jq .status)"
    if [ "$status" = in_progress ] || [ "$status" = completed ]; then
      exit 0
    fi
  fi
  sleep 2
done
echo "CI pull_request run missing for $head_sha; check skip-CI markers and workflow permissions." >&2
exit 1
