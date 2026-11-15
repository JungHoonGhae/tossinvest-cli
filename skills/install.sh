#!/bin/sh
# Install the tossctl agent skill into a Claude skills directory.
#
#   Remote (no clone needed):
#     curl -fsSL https://raw.githubusercontent.com/JungHoonGhae/tossinvest-cli/main/skills/install.sh | sh
#   From a local clone:
#     skills/install.sh
#
# Target dir: $CLAUDE_SKILLS_DIR, else ~/.claude/skills. Override the source
# branch/repo with TOSSCTL_SKILL_REF / TOSSCTL_SKILL_REPO.
set -eu

REPO="${TOSSCTL_SKILL_REPO:-JungHoonGhae/tossinvest-cli}"
REF="${TOSSCTL_SKILL_REF:-main}"
DEST_BASE="${CLAUDE_SKILLS_DIR:-$HOME/.claude/skills}"
DEST="$DEST_BASE/tossctl"
FILES="SKILL.md references/operations.md"

say() { printf '%s\n' "$*"; }

mkdir -p "$DEST/references"

# Prefer a local clone when this script sits next to the skill (offline install);
# otherwise fetch the raw files from GitHub.
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
if [ -f "$SCRIPT_DIR/tossctl/SKILL.md" ]; then
  say "Installing tossctl skill from local clone → $DEST"
  for f in $FILES; do
    mkdir -p "$DEST/$(dirname "$f")"
    cp "$SCRIPT_DIR/tossctl/$f" "$DEST/$f"
  done
else
  say "Fetching tossctl skill ($REPO@$REF) → $DEST"
  base="https://raw.githubusercontent.com/$REPO/$REF/skills/tossctl"
  for f in $FILES; do
    mkdir -p "$DEST/$(dirname "$f")"
    curl -fsSL "$base/$f" -o "$DEST/$f"
  done
fi

say "Installed:"
for f in $FILES; do say "  $DEST/$f"; done
say ""
say "Restart your Claude client (or re-scan skills) to pick up 'tossctl'."
