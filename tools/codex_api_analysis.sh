#!/usr/bin/env bash
set -euo pipefail

repo="JungHoonGhae/tossinvest-cli"
mode="${1:-run}"
if [ "$mode" != "run" ] && [ "$mode" != "--check" ]; then
  echo "usage: codex_api_analysis.sh [--check]" >&2
  exit 2
fi

for command_name in codex gh git python3; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "missing required command: $command_name" >&2
    exit 1
  fi
done

if ! codex login status >/dev/null 2>&1; then
  echo "Codex is not logged in; run 'codex login' interactively" >&2
  exit 1
fi
if ! gh auth status --hostname github.com >/dev/null 2>&1; then
  echo "GitHub CLI is not logged in; run 'gh auth login' interactively" >&2
  exit 1
fi

work_dir="$(mktemp -d "${TMPDIR:-/tmp}/tossctl-codex-analysis.XXXXXX")"
cleanup() {
  case "$work_dir" in
    */tossctl-codex-analysis.*) rm -rf -- "$work_dir" ;;
    *) echo "refusing to remove unexpected work directory: $work_dir" >&2 ;;
  esac
}
trap cleanup EXIT

checkout="$work_dir/repo"
gh repo clone "$repo" "$checkout" -- --filter=blob:none --quiet

spec_commit=$(git -C "$checkout" log -1 --format=%H -- \
  docs/migration/openapi.latest.json docs/migration/asyncapi.latest.json)
if [ -z "$spec_commit" ]; then
  echo "No official API snapshot commit found."
  exit 0
fi

analysis_date=$(TZ=UTC git -C "$checkout" show -s --format=%cd \
  --date=format-local:%Y-%m-%d "$spec_commit")
short_commit=$(git -C "$checkout" rev-parse --short=8 "$spec_commit")
base_file="docs/reverse-engineering/change-analysis/${analysis_date}.md"
analysis_file="$base_file"

if [ -f "$checkout/$base_file" ]; then
  file_commit=$(git -C "$checkout" log -1 --format=%H -- "$base_file")
  if [ -n "$file_commit" ] && git -C "$checkout" merge-base --is-ancestor "$spec_commit" "$file_commit"; then
    echo "Official API snapshot $short_commit is already covered by $base_file."
    exit 0
  fi
  analysis_file="docs/reverse-engineering/change-analysis/${analysis_date}-${short_commit}.md"
fi

branch="analysis/api-change-${analysis_date//-/}-${short_commit}"
if gh pr list --repo "$repo" --state open --head "$branch" --json number --jq 'length' | grep -qx '1'; then
  echo "An analysis PR is already open for $short_commit."
  exit 0
fi

if [ "$mode" = "--check" ]; then
  echo "Pending Codex analysis: commit=$short_commit output=$analysis_file"
  exit 0
fi

git -C "$checkout" checkout -q -b "$branch" origin/main
read -r -d '' prompt <<'PROMPT' || true
The official Toss Open API snapshot changed in the commit specified below. Analyze it and write
exactly one Markdown file at the specified output path. Do not change any other file, commit, push,
call GitHub, or make live API requests.

1. Run tools/openapi_diff.py with --rev and the specified commit. Do not use a raw text diff.
   Inspect every path left in the final [미분류 변경] section; it must be empty before concluding.
   If docs/migration/asyncapi.latest.json changed in that commit, inspect that commit diff too.
2. Compare new endpoints with internal/official/ and the shared internal/ops/ registry. For
   removals or changes, identify affected implementations.
3. Classify the result as (a) no-op, (b) feature candidate, or (c) breaking risk.
4. Write the report in Korean. Use the specified heading and include a summary, coverage comparison,
   classification, and recommended follow-up. For (c), put > breaking-risk on its own line at the top.

Never include real account data. Even a no-op classification must produce the file.
PROMPT
{
  printf '%s\n\n' "$prompt"
  printf 'Spec commit: %s\n' "$spec_commit"
  printf 'Output path: %s\n' "$analysis_file"
  printf 'Required first heading: # API 변경 분석 — %s\n' "$analysis_date"
} | codex exec --ephemeral --ignore-user-config --sandbox workspace-write --cd "$checkout" -

if [ ! -s "$checkout/$analysis_file" ]; then
  echo "Codex did not create the required analysis file: $analysis_file" >&2
  exit 1
fi
changed_files=$(git -C "$checkout" status --porcelain --untracked-files=all | cut -c4-)
if [ "$changed_files" != "$analysis_file" ]; then
  echo "Codex changed files outside the required analysis artifact:" >&2
  printf '%s\n' "$changed_files" >&2
  exit 1
fi

title_prefix=""
if grep -qiE '^> *breaking-risk *$' "$checkout/$analysis_file"; then
  title_prefix="[breaking-risk] "
fi
git -C "$checkout" config user.name "JungHoonGhae"
git -C "$checkout" config user.email "236698722+JungHoonGhae@users.noreply.github.com"
git -C "$checkout" add "$analysis_file"
git -C "$checkout" commit -m "docs(analysis): API 변경 분석 ${analysis_date}"
git -C "$checkout" push --set-upstream origin "$branch"
gh pr create --repo "$repo" --base main --head "$branch" \
  --title "${title_prefix}docs(analysis): API 변경 분석 ${analysis_date}" \
  --body "$(printf '%s\n\n---\nCodex 정기 분석입니다. 검토 후 병합하거나 닫아 주세요.' "$(cat "$checkout/$analysis_file")")"
