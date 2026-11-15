# Agent skills

Distributable [agent skills](https://docs.claude.com/en/docs/claude-code/skills)
for working with `tossctl`. This is the **canonical, version-controlled source**
— the local `.claude/skills/` directory is gitignored, so skills live here and
are copied into a user's skills directory on install.

## `tossctl/` — use tossctl from an agent

Teaches an agent to drive `tossctl` (CLI + MCP): discover operations
(`list_operations`→`describe_operation`→`call_operation`, or `tossctl ops …`),
set up auth, respect the read-first + trading-gated safety model, and never
expose real account data.

`tossctl/references/operations.md` is **generated** from the live operation
registry — see "Staying in sync" below.

## Install

**One line (skills.sh style), no clone:**

```sh
curl -fsSL https://raw.githubusercontent.com/JungHoonGhae/tossinvest-cli/main/skills/install.sh | sh
```

**From a clone (offline):**

```sh
skills/install.sh
```

Both copy `tossctl/` into `$CLAUDE_SKILLS_DIR` (default `~/.claude/skills/`).
Restart your Claude client (or re-scan skills) afterward. Override the target
with `CLAUDE_SKILLS_DIR`, or the source with `TOSSCTL_SKILL_REPO` /
`TOSSCTL_SKILL_REF`.

**Manual:** copy `skills/tossctl/` to `~/.claude/skills/tossctl/`.

## Staying in sync with development

The skill's operation catalog is not hand-maintained — it is generated from the
`internal/ops` registry (the same source behind MCP `list_operations`), so
adding or renaming an operation updates the skill in one step:

```sh
python3 tools/build_skill.py          # regenerate tossctl/references/operations.md
python3 tools/build_skill.py --check  # verify it is in sync (CI runs this)
```

CI (`에이전트 스킬 동기화 체크`) fails any PR whose skill catalog has drifted
from the registry, so the skill can't quietly go stale.

## Follow-ups (not yet built)

- `tossctl skill install` — a CLI-native installer (embeds this skill in the
  binary via `go:embed`) plus a nudge from `tossctl doctor`.
- An `npx` package for users who run the MCP without the CLI.
- Publishing the skill as a release asset on tag.
