#!/usr/bin/env python3
"""Generate STATS.md — daily stars + forks (backfilled from launch),
release downloads, and an integrated Total.

Stars and forks are recomputed every run from each stargazer's `starredAt`
and each fork's `createdAt`. These are reconstructions from currently remaining
stars/forks, not historical daily observations; removals can revise old rows.
GitHub only exposes *cumulative* release download totals (no historical
dailies), so downloads are logged forward from the day this script first runs,
persisted in docs/.stats-downloads.json. Total = stars + forks + downloads.

Usage: python3 tools/update_stats.py   (needs `gh` authenticated)
Run daily via CI to append a fresh row.
"""
from __future__ import annotations

import json
import subprocess
from datetime import date, datetime, timedelta, timezone
from pathlib import Path

REPO = "JungHoonGhae/tossinvest-cli"
ROOT = Path(__file__).resolve().parent.parent
STATS_FILE = ROOT / "STATS.md"
DL_LOG = ROOT / "docs" / ".stats-downloads.json"

STAR_QUERY = """
query($endCursor: String) {
  repository(owner: "JungHoonGhae", name: "tossinvest-cli") {
    stargazers(first: 100, after: $endCursor) {
      pageInfo { hasNextPage endCursor }
      edges { starredAt }
    }
  }
}
"""

FORK_QUERY = """
query($endCursor: String) {
  repository(owner: "JungHoonGhae", name: "tossinvest-cli") {
    forks(first: 100, after: $endCursor) {
      pageInfo { hasNextPage endCursor }
      nodes { createdAt }
    }
  }
}
"""


def _gh_json(args: list[str]) -> dict:
    out = subprocess.run(["gh", *args], capture_output=True, text=True, check=True).stdout
    return json.loads(out)


def _paginate_dates(query: str, conn_key: str, items_key: str, date_field: str) -> list[str]:
    """Page through a connection and collect each item's date (UTC YYYY-MM-DD)."""
    dates: list[str] = []
    cursor: str | None = None
    while True:
        args = ["api", "graphql", "-f", f"query={query}"]
        if cursor:
            args += ["-F", f"endCursor={cursor}"]
        conn = _gh_json(args)["data"]["repository"][conn_key]
        dates += [item[date_field][:10] for item in conn[items_key]]
        if conn["pageInfo"]["hasNextPage"]:
            cursor = conn["pageInfo"]["endCursor"]
        else:
            break
    return sorted(dates)


def fetch_star_dates() -> list[str]:
    return _paginate_dates(STAR_QUERY, "stargazers", "edges", "starredAt")


def fetch_fork_dates() -> list[str]:
    return _paginate_dates(FORK_QUERY, "forks", "nodes", "createdAt")


def fetch_download_total() -> int:
    releases = _gh_json(["api", f"repos/{REPO}/releases?per_page=100"])
    return sum(a.get("download_count", 0) for r in releases for a in r.get("assets", []))


def load_dl_log() -> dict[str, int]:
    if DL_LOG.exists():
        return json.loads(DL_LOG.read_text())
    return {}


def fmt(n: int) -> str:
    return f"{n:,}"


def cell(cur: int, prev: int | None) -> str:
    if prev is None:
        return fmt(cur)
    delta = cur - prev
    sign = "+" if delta >= 0 else "−"
    return f"{fmt(cur)} ({sign}{fmt(abs(delta))})"


def counts_per_day(dates: list[str]) -> dict[str, int]:
    per: dict[str, int] = {}
    for d in dates:
        per[d] = per.get(d, 0) + 1
    return per


def main() -> None:
    today = datetime.now(timezone.utc).date()
    star_dates = fetch_star_dates()
    fork_dates = fetch_fork_dates()
    if not star_dates:
        raise SystemExit("no stargazers found")

    stars_per = counts_per_day(star_dates)
    forks_per = counts_per_day(fork_dates)

    # launch = earliest signal across stars/forks
    first = min([star_dates[0]] + ([fork_dates[0]] if fork_dates else []))
    launch = date.fromisoformat(first)

    # download log: record today's cumulative total, keep history
    dl_log = load_dl_log()
    dl_log[today.isoformat()] = fetch_download_total()
    DL_LOG.parent.mkdir(parents=True, exist_ok=True)
    DL_LOG.write_text(json.dumps(dict(sorted(dl_log.items())), indent=2) + "\n")

    total_stars = len(star_dates)
    total_forks = len(fork_dates)
    total_dl = dl_log[today.isoformat()]
    grand_total = total_stars + total_forks + total_dl

    rows: list[str] = []
    cum_stars = cum_forks = 0
    running_dl: int | None = None  # last known cumulative downloads, carried forward
    prev_stars = prev_forks = prev_total = prev_dl_sample = None
    d = launch
    while d <= today:
        iso = d.isoformat()
        cum_stars += stars_per.get(iso, 0)
        cum_forks += forks_per.get(iso, 0)
        sample = dl_log.get(iso)
        if sample is not None:
            # fresh measurement this day → show value + delta
            dl_str = cell(sample, prev_dl_sample)
            prev_dl_sample = sample
            running_dl = sample
        elif running_dl is not None:
            # no fresh sample, but downloads are cumulative → carry last value forward
            dl_str = fmt(running_dl)
        else:
            # before download tracking began (GitHub exposes no historical dailies)
            dl_str = "n/a"
        total = cum_stars + cum_forks + (running_dl or 0)
        rows.append(
            f"| {iso} | {cell(cum_stars, prev_stars)} | {cell(cum_forks, prev_forks)} | {dl_str} | {cell(total, prev_total)} |"
        )
        prev_stars, prev_forks, prev_total = cum_stars, cum_forks, total
        d += timedelta(days=1)

    header = f"""# Stats

> Auto-updated daily. **Stars** and **forks** are backfilled from launch via each
> remaining stargazer's `starredAt` / each fork's `createdAt`. Removed stars/forks
> can revise old rows; this is not a historical daily snapshot. **Release downloads** are
> GitHub's cumulative asset totals — GitHub exposes no historical dailies, so the
> download count is tracked forward from the day measurement started; rows before
> that show `n/a`, and days without a fresh sample carry the last known value
> forward. **Total** = stars + forks + downloads.

**⭐ {fmt(total_stars)} stars · 🍴 {fmt(total_forks)} forks · ⬇️ {fmt(total_dl)} downloads · Σ {fmt(grand_total)} total · since {launch.isoformat()}**

_Last updated: {today.isoformat()} (UTC)_

| Date | Stars | Forks | Release Downloads | Total |
| ---------- | -------------------- | -------------------- | -------------------- | -------------------- |
"""
    # Latest day first (rows are built oldest→newest; deltas stay vs. prior day).
    STATS_FILE.write_text(header + "\n".join(reversed(rows)) + "\n")
    print(
        f"wrote {STATS_FILE.relative_to(ROOT)}: {total_stars} stars, {total_forks} forks, "
        f"{total_dl} downloads, Σ {grand_total}, {(today - launch).days + 1} daily rows since {launch}"
    )


if __name__ == "__main__":
    main()
