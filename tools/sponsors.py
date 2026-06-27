#!/usr/bin/env python3
"""Refresh the README sponsor block from live GitHub Sponsors data.

Fetches the maintainer's sponsors (including one-time and private) via the
GitHub GraphQL API and rewrites the region between the
`<!-- sponsors:start -->` / `<!-- sponsors:end -->` markers in each target
file. Run on a schedule by .github/workflows/sponsors.yml so new sponsors
appear automatically.

Privacy: PUBLIC sponsors are always shown (avatar + profile link). PRIVATE
sponsors are shown ONLY if their login is listed in
tools/sponsors_allowlist.json -> show_private (explicit per-person consent);
otherwise they count toward the total but stay anonymous. The total count
always reflects everyone (one-time included), matching the GitHub sponsor page.

Auth: uses `gh api graphql`. Locally that is your `gh` login; in CI set
GH_TOKEN to a PAT with the read:user / sponsor scope (secret SPONSORS_TOKEN).

Usage:
  python3 tools/sponsors.py                       # README.md:repo-ko README.en.md:repo-en
  python3 tools/sponsors.py README.md:profile     # custom file:context pairs
Contexts: repo-ko | repo-en | profile
"""
from __future__ import annotations

import html
import json
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
ALLOWLIST = ROOT / "tools" / "sponsors_allowlist.json"
SPONSOR_URL = "https://github.com/sponsors/JungHoonGhae"
START, END = "<!-- sponsors:start -->", "<!-- sponsors:end -->"

QUERY = """
query {
  viewer {
    sponsorshipsAsMaintainer(first: 100, includePrivate: true, activeOnly: false) {
      totalCount
      nodes {
        privacyLevel
        sponsorEntity {
          __typename
          ... on User { login name avatarUrl url }
          ... on Organization { login name avatarUrl url }
        }
      }
    }
  }
}
"""


def fetch() -> tuple[int, list[dict]]:
    out = subprocess.run(
        ["gh", "api", "graphql", "-f", f"query={QUERY}"],
        capture_output=True, text=True, check=True,
    ).stdout
    conn = json.loads(out)["data"]["viewer"]["sponsorshipsAsMaintainer"]
    total = conn["totalCount"]
    show_private = set(json.loads(ALLOWLIST.read_text()).get("show_private", []))
    seen, shown = set(), []
    for node in conn["nodes"]:
        ent = node.get("sponsorEntity")
        if not ent:
            continue
        login = ent["login"]
        if login in seen:
            continue
        seen.add(login)
        is_public = node["privacyLevel"] == "PUBLIC"
        if is_public or login in show_private:
            shown.append({"login": login, "name": ent.get("name") or login,
                          "url": ent.get("url") or f"https://github.com/{login}"})
    return total, shown


def _avatars(shown: list[dict], anon: int, px: int, anon_src: str) -> str:
    out = []
    for s in shown:
        nm = html.escape(s["name"], quote=True)
        out.append(
            f'<a href="{s["url"]}" title="{nm}">'
            f'<img src="https://github.com/{s["login"]}.png?size={px * 2}" '
            f'width="{px}" height="{px}" alt="{nm}" /></a>'
        )
    for _ in range(anon):  # private/undisclosed sponsors: anonymous silhouette
        out.append(
            f'<a href="{SPONSOR_URL}" title="비공개 후원자 / private sponsor">'
            f'<img src="{anon_src}" width="{px}" height="{px}" alt="private sponsor" /></a>'
        )
    return "".join(out)


def render(context: str, total: int, shown: list[dict]) -> str:
    anon = max(0, total - len(shown))
    if context == "profile":
        if total == 0:
            body = (f'<sub>be the first to <a href="{SPONSOR_URL}">back my work</a>. '
                    f'one-time is welcome too.</sub>')
        else:
            who = "sponsor" if total == 1 else "sponsors"
            av = _avatars(shown, anon, 22, "assets/anonymous.svg")
            lead = (f'backed by {av} · ' if av else "")
            body = (f'<sub>{lead}<strong>{total}</strong> {who} so far '
                    f'(one-time included). be the next.</sub>')
        return f"{START}\n\n{body}\n\n{END}"

    # repo-ko / repo-en: centered avatars + a count line
    avs = _avatars(shown, anon, 56, "docs/assets/sponsors/anonymous.svg")
    avatars_p = f'<p align="center">\n  {avs}\n</p>\n\n' if avs else ""
    if context == "repo-en":
        if total == 0:
            line = ("Be the first sponsor — your support funds my open-source work, "
                    "tossinvest-cli included.")
        else:
            who = "person backs" if total == 1 else "people back"
            line = (f"<strong>{total}</strong> {who} my open-source work "
                    "(one-time included). Sponsorship funds my projects, tossinvest-cli included.")
    else:  # repo-ko
        if total == 0:
            line = "첫 후원자가 되어주세요 — 후원은 tossinvest-cli 를 포함한 제 오픈소스 작업에 쓰입니다."
        else:
            line = (f"현재 <strong>{total}</strong>분이 제 오픈소스 작업을 후원하고 있습니다 "
                    "(일회성 포함). 후원은 tossinvest-cli 를 포함한 제 작업 전반에 쓰입니다.")
    return f'{START}\n\n{avatars_p}<p align="center"><sub>{line}</sub></p>\n\n{END}'


def apply(path: Path, context: str, total: int, shown: list[dict]) -> bool:
    text = path.read_text()
    if START not in text or END not in text:
        print(f"  ! {path.name}: markers not found — skipped")
        return False
    pre, rest = text.split(START, 1)
    _, post = rest.split(END, 1)
    new = pre + render(context, total, shown) + post
    if new == text:
        print(f"  = {path.name}: no change")
        return False
    path.write_text(new)
    print(f"  ✓ {path.name} ({context}): {total} sponsor(s), {len(shown)} shown")
    return True


def main() -> None:
    pairs = sys.argv[1:] or ["README.md:repo-ko", "README.en.md:repo-en"]
    total, shown = fetch()
    changed = False
    for pair in pairs:
        file, _, context = pair.partition(":")
        changed |= apply(ROOT / file, context or "repo-ko", total, shown)
    if not changed:
        print("sponsors: up to date")


if __name__ == "__main__":
    main()
