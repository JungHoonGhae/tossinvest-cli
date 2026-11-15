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

**With the CLI (installs the copy embedded in your `tossctl` binary):**

```sh
tossctl skill install
```

Offline, no network fetch, and always the skill that matches the installed
binary. Target: `--dir`, else `$CLAUDE_SKILLS_DIR`, else `~/.claude/skills`.
Re-run after upgrading `tossctl` to refresh the skill.

**One line (skills.sh style), no clone — when you run the MCP without the CLI:**

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
python3 tools/build_skill.py          # regenerate the catalog + sync the embedded copy
python3 tools/build_skill.py --check  # verify every managed path is in sync (CI runs this)
```

The same script also mirrors this skill into `internal/skill/assets/`, the copy
the binary embeds for `tossctl skill install` — so the embedded skill and the
distributable one can never drift apart. CI (`에이전트 스킬 동기화 체크`) fails
any PR whose catalog or embedded copy has drifted from the registry / canonical
source, so the skill can't quietly go stale.

## Follow-ups (not yet built)

- An `npx` package for users who run the MCP without the CLI. Low priority: the
  curl|sh one-liner above already installs without the Go CLI.

`tossctl skill install` (CLI-native, via `go:embed`) and the `tossctl doctor`
nudge are built. A separate release-asset download is unnecessary now that the
binary carries the skill.
