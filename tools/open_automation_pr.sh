#!/usr/bin/env bash
set -euo pipefail

: "${GH_TOKEN:?GH_TOKEN is required}"
: "${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is required}"

mode=publish
if [ "${1:-}" = "--prepare" ]; then mode=prepare; shift; fi
if [ "$#" -lt 1 ] || [ "$#" -gt 3 ] || { [ "$mode" = publish ] && [ "$#" -lt 2 ]; }; then
  echo "usage: open_automation_pr.sh [--prepare] <branch-prefix> [title] [body]" >&2
  exit 2
fi
branch_prefix="$1"
case "$branch_prefix" in
  automation/repository-metadata|automation/wts-monitor|automation/daily-monitor) ;;
  *) echo "Unknown automation family: $branch_prefix" >&2; exit 2 ;;
esac

# Fail closed on duplicate, cross-repository, or human-authored branches.
# All callers serialize prepare -> generate -> publish per family.
prs="$(gh pr list --repo "$GITHUB_REPOSITORY" --state open --base main --limit 100 \
  --json url,headRefName,author,isCrossRepository \
  --jq "[.[] | select(.headRefName == \"$branch_prefix\" or (.headRefName | startswith(\"$branch_prefix-\")))]")"
if [ "$(jq length <<< "$prs")" -gt 1 ]; then
  echo "Multiple PRs for $branch_prefix; consolidate them before generating more." >&2
  exit 1
fi
pr_url="$(jq -r '.[0].url // empty' <<< "$prs")"
branch="$(jq -r '.[0].headRefName // empty' <<< "$prs")"
if [ -n "$pr_url" ]; then
  jq -e '.[0] | .isCrossRepository == false and (.author.login == "app/github-actions" or .author.login == "github-actions[bot]")' <<< "$prs" >/dev/null
  [[ "$branch" =~ ^automation/[a-z0-9-]+$ ]] || exit 2
fi

validate_files() {
  local ref="$1" path paths
  paths="$(git diff --name-only "origin/main...$ref")"
  while IFS= read -r path; do
    [ -n "$path" ] || continue
    case "$branch_prefix:$path" in
      automation/repository-metadata:STATS.md|automation/repository-metadata:docs/.stats-downloads.json|automation/repository-metadata:README.md|automation/repository-metadata:README.en.md|automation/repository-metadata:docs/assets/star-history/*.svg) ;;
      automation/wts-monitor:docs/reverse-engineering/wts-endpoints.json|automation/wts-monitor:docs/reverse-engineering/android-app.json) ;;
      automation/daily-monitor:docs/migration/.openapi-snapshot.json|automation/daily-monitor:docs/migration/openapi.latest.json|automation/daily-monitor:docs/migration/asyncapi.latest.json|automation/daily-monitor:website-fumadocs/content/docs/reference/support-scope.mdx|automation/daily-monitor:website-fumadocs/content/docs/reference/support-scope.en.mdx) ;;
      *) echo "Unexpected automation file: $path" >&2; return 1 ;;
    esac
  done <<< "$paths"
}

if [ "$mode" = prepare ]; then
  [ -z "$(git status --porcelain)" ] || { echo "Working tree must be clean." >&2; exit 1; }
  git fetch origin main
  git config user.name "github-actions[bot]"
  git config user.email "41898282+github-actions[bot]@users.noreply.github.com"
  if [ -n "$pr_url" ]; then
    git fetch origin "refs/heads/$branch"
    validate_files FETCH_HEAD
    git switch -c "$branch" FETCH_HEAD
    # Preserve unpublished download observations and all reviewed history.
    # Conflicts stop the workflow; never reset or force-push generated data.
    git merge --no-edit origin/main
  else
    : "${GITHUB_RUN_ID:?GITHUB_RUN_ID is required}"
    branch="${branch_prefix}-${GITHUB_RUN_ID}-${GITHUB_RUN_ATTEMPT:-1}"
    git switch -c "$branch" origin/main
  fi
  exit 0
fi

current_branch="$(git branch --show-current)"
[[ "$current_branch" == "$branch_prefix" || "$current_branch" == "$branch_prefix"-* ]] || { echo "Run --prepare before generation." >&2; exit 2; }
if [ -n "$branch" ] && [ "$branch" != "$current_branch" ]; then
  echo "Another automation PR appeared; refusing a duplicate." >&2
  exit 1
fi
branch="$current_branch"
if git diff --quiet origin/main...HEAD; then
  echo "No automation changes to publish."
  exit 0
fi
validate_files HEAD
head_sha="$(git rev-parse HEAD)"
# Normal push rejects races rather than overwriting another writer.
git push --set-upstream origin "$branch"
if [ -z "$pr_url" ]; then
  pr_url="$(gh pr create --repo "$GITHUB_REPOSITORY" --base main --head "$branch" \
    --title "$2" --label maintenance \
    --body "${3:-Automated repository maintenance. CI must pass before this PR can merge.}")"
fi
bash tools/approve_automation_ci.sh "$branch" "$head_sha"
gh pr merge "$pr_url" --repo "$GITHUB_REPOSITORY" --auto --squash --delete-branch --match-head-commit "$head_sha"
printf 'Updated %s and queued protected auto-merge.\n' "$pr_url"
