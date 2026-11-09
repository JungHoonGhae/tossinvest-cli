#!/usr/bin/env python3
"""Precheck for the unattended development automation (docs/automation/auto-dev.md).

Run by Orca before each scheduled run; exit 0 starts the agent, anything else
skips it. Also turns WTS catalog regressions into auto-dev issues.

    git -C <repo> fetch -q origin main
    git -C <repo> show origin/main:tools/auto_dev_precheck.py | python3 - --repo <repo>

Exit 0 when there is work:
  - an open auto-dev issue without auto-dev:in-progress / ready-for-human, or
  - auto-dev changes merged since the last release and no release yet today (KST).

stdlib + git + gh only.
"""
from __future__ import annotations

import argparse
import datetime as dt
import importlib.util
import json
import shutil
import subprocess
import sys
import tempfile

REPO_SLUG = "JungHoonGhae/tossinvest-cli"
CATALOG = "docs/reverse-engineering/wts-endpoints.json"
LABEL = "auto-dev"
BLOCKING_LABELS = {"auto-dev:in-progress", "ready-for-human"}
KST = dt.timezone(dt.timedelta(hours=9))
SCAN_DAYS = 30


def run(*args: str, cwd: str | None = None) -> str:
    return subprocess.run(args, cwd=cwd, check=True, capture_output=True, text=True).stdout


def gh_json(*args: str):
    return json.loads(run("gh", *args, "--repo", REPO_SLUG) or "null")


def catalog_at(repo: str, rev: str) -> dict:
    try:
        return json.loads(run("git", "show", f"{rev}:{CATALOG}", cwd=repo)).get("endpoints", {})
    except (subprocess.CalledProcessError, json.JSONDecodeError):
        return {}


def removed_implemented(previous: dict, current: dict) -> list[str]:
    return sorted(
        path for path, entry in previous.items()
        if isinstance(entry, dict) and entry.get("status") == "implemented" and path not in current
    )


def go_exposures(repo: str) -> set[str]:
    """WTS paths that origin/main's production Go code calls today.

    A path that left the catalog matters only while the code still calls it;
    otherwise it was retired on purpose. Uses a detached origin/main worktree
    so a local checkout on another branch cannot skew the answer.
    """
    tmp = tempfile.mkdtemp(prefix="tossctl-auto-dev-")
    tree = f"{tmp}/main"
    try:
        run("git", "worktree", "add", "-q", "--detach", tree, "origin/main", cwd=repo)
        spec = importlib.util.spec_from_file_location("wts_endpoints_main", f"{tree}/tools/wts_endpoints.py")
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        return set(module.discover_go_exposures(tree))
    finally:
        subprocess.run(["git", "worktree", "remove", "--force", tree], cwd=repo, capture_output=True)
        shutil.rmtree(tmp, ignore_errors=True)


def file_wts_regressions(repo: str, known_bodies: list[str]) -> None:
    commits = run("git", "log", f"--since={SCAN_DAYS}.days", "--format=%H", "origin/main", "--", CATALOG, cwd=repo).split()
    used: set[str] | None = None
    for commit in commits:
        marker = f"wts-regression:{commit[:8]}"
        if any(marker in body for body in known_bodies):
            continue
        gone = removed_implemented(catalog_at(repo, f"{commit}^"), catalog_at(repo, commit))
        if not gone:
            continue
        if used is None:
            used = go_exposures(repo)
        gone = [path for path in gone if path in used]
        if not gone:
            continue
        paths = "\n".join(f"- `{p}`" for p in gone)
        body = (
            "WTS 카탈로그 갱신에서 tossctl 이 쓰는(implemented) 경로가 공개 번들에서 사라졌습니다.\n\n"
            f"{paths}\n\n"
            f"카탈로그 커밋: {commit}\n\n"
            "경로가 바뀌었는지(새 경로로 이전), 기능이 없어졌는지 확인해 대응합니다. 절차: docs/automation/auto-dev.md\n\n"
            f"<!-- {marker} -->"
        )
        run("gh", "issue", "create", "--repo", REPO_SLUG, "--label", LABEL,
            "--title", f"[auto-dev] WTS API 회귀 대응 ({commit[:8]})", "--body", body)
        print(f"filed WTS regression issue for {commit[:8]}: {len(gone)} paths")


def eligible_issue(issues: list[dict]) -> dict | None:
    for issue in sorted(issues, key=lambda i: i["createdAt"]):
        labels = {label["name"] for label in issue.get("labels", [])}
        if issue.get("state", "OPEN") == "OPEN" and LABEL in labels and not labels & BLOCKING_LABELS:
            return issue
    return None


def release_pending(repo: str, today: dt.date) -> bool:
    tags = run("git", "tag", "--sort=-creatordate", "--list", "v*", cwd=repo).split()
    if not tags:
        return False
    tagged = dt.datetime.fromisoformat(run("git", "log", "-1", "--format=%cI", tags[0], cwd=repo).strip())
    if tagged.astimezone(KST).date() >= today:
        return False  # one release per day
    merged = gh_json("pr", "list", "--state", "merged", "--limit", "50",
                     "--json", "headRefName,mergedAt")
    return any(
        pr["headRefName"].startswith("auto-dev/")
        and dt.datetime.fromisoformat(pr["mergedAt"].replace("Z", "+00:00")) > tagged
        for pr in merged or []
    )


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo", required=True, help="local checkout of tossinvest-cli")
    args = parser.parse_args(argv)
    run("git", "fetch", "-q", "--tags", "origin", "main", cwd=args.repo)

    issues = gh_json("issue", "list", "--state", "all", "--label", LABEL, "--limit", "200",
                     "--json", "number,body,labels,createdAt,state") or []
    file_wts_regressions(args.repo, [issue.get("body") or "" for issue in issues])
    issues = gh_json("issue", "list", "--state", "open", "--label", LABEL, "--limit", "200",
                     "--json", "number,labels,createdAt,state") or []

    issue = eligible_issue(issues)
    if issue:
        print(f"work: issue #{issue['number']}")
        return 0
    if release_pending(args.repo, dt.datetime.now(KST).date()):
        print("work: release pending")
        return 0
    print("no auto-dev work")
    return 1


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
