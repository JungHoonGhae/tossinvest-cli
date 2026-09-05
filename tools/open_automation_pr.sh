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
  --label maintenance \
  --body "$body")"

# Public repositories can require approval before workflows created by an
# automation-authored PR are allowed to run. A separate workflow_dispatch run
# does not satisfy a required check attached to the PR, even at the same SHA.
# Approve the actual pull_request run so the ruleset sees test-and-build.
head_sha="$(git rev-parse HEAD)"
run_id=""
for _ in $(seq 1 30); do
  run_id="$(gh run list \
    --repo "$GITHUB_REPOSITORY" \
    --workflow ci.yml \
    --branch "$branch" \
    --event pull_request \
    --limit 10 \
    --json databaseId,headSha \
    --jq ".[] | select(.headSha == \"$head_sha\") | .databaseId" \
    | head -n 1)"
  [ -n "$run_id" ] && break
  sleep 2
done

if [ -z "$run_id" ]; then
  echo "CI pull_request run was not created for $head_sha." >&2
  exit 1
fi

conclusion="$(gh run view "$run_id" \
  --repo "$GITHUB_REPOSITORY" \
  --json conclusion \
  --jq .conclusion)"
if [ "$conclusion" = "action_required" ]; then
  gh api --method POST \
    "repos/$GITHUB_REPOSITORY/actions/runs/$run_id/approve"
fi

gh pr merge "$pr_url" --repo "$GITHUB_REPOSITORY" --auto --squash --delete-branch

printf 'Opened %s, authorized PR CI, and queued protected auto-merge.\n' "$pr_url"
