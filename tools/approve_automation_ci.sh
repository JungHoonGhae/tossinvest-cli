#!/usr/bin/env bash
set -euo pipefail
: "${GH_TOKEN:?GH_TOKEN is required}"
: "${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is required}"
branch="$1"
head_sha="$2"
[[ "$branch" =~ ^automation/[a-z0-9-]+$ ]] || exit 2
[[ "$head_sha" =~ ^[a-f0-9]{40}$ ]] || exit 2

seen_run=""

# GITHUB_TOKEN PR events create approval-required runs. Authorize the actual
# PR run at this SHA, never a detached workflow_dispatch substitute.
# Returns 0 once the run is approved or running; 1 if it never settled.
wait_for_run() {
  local attempts="$1" run_id conclusion status
  for ((i = 0; i < attempts; i++)); do
    run_id="$(gh run list --repo "$GITHUB_REPOSITORY" --workflow ci.yml \
      --branch "$branch" --event pull_request --limit 20 \
      --json databaseId,headSha \
      --jq ".[] | select(.headSha == \"$head_sha\") | .databaseId" | head -n 1)"
    if [ -n "$run_id" ]; then
      seen_run="$run_id"
      conclusion="$(gh run view "$run_id" --repo "$GITHUB_REPOSITORY" --json conclusion --jq .conclusion)"
      if [ "$conclusion" = action_required ]; then
        gh api --method POST "repos/$GITHUB_REPOSITORY/actions/runs/$run_id/approve"
        return 0
      fi
      status="$(gh run view "$run_id" --repo "$GITHUB_REPOSITORY" --json status --jq .status)"
      if [ "$status" = in_progress ] || [ "$status" = completed ]; then
        return 0
      fi
      # queued/requested/waiting: it may still turn into action_required.
    fi
    sleep 4
  done
  return 1
}

# pull_request runs have shown up ~110s after the push; 30 polls of ~5s each
# allow ~2.5 minutes per attempt (two attempts fit the 10-minute job budget).
# A run that exists but is still queued is attached to the PR; it starts on
# its own, and the refresh workflow approves it if it later needs approval.
settle() {
  wait_for_run 30 && exit 0
  if [ -n "$seen_run" ]; then
    echo "CI run $seen_run for $head_sha is still queued; leaving it to start." >&2
    exit 0
  fi
}

settle

# GitHub occasionally drops the pull_request event. It never arrives late, and
# re-pushing the same SHA emits nothing, so the PR would stay blocked until the
# data changes. Reopening the PR emits a fresh pull_request event at this SHA.
echo "No CI run for $head_sha; reopening the PR to re-emit pull_request." >&2
gh pr close "$branch" --repo "$GITHUB_REPOSITORY"
gh pr reopen "$branch" --repo "$GITHUB_REPOSITORY"
settle

echo "CI pull_request run missing for $head_sha; check skip-CI markers and workflow permissions." >&2
exit 1
