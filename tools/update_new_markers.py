#!/usr/bin/env python3
"""Maintain 🆕 markers on README comparison-table rows.

Policy: a feature row is marked 🆕 if its command was first released within the
last NEW_WINDOW_DAYS (rolling, from today). Release dates live in FEATURE_DATES
below (single source of truth — keyed by the command shown in the row), so the
README tables stay clean (no inline date comments).

    python3 tools/update_new_markers.py          # uses today
    NEW_MARKER_DATE=2026-06-19 python3 tools/update_new_markers.py   # pin date
    python3 tools/update_new_markers.py --check   # CI: non-zero if stale

When you add a new comparison-table row, add its command + release date here.
stdlib only.
"""
import datetime
import os
import re
import sys

NEW_WINDOW_DAYS = 30
FILES = ["README.md", "README.en.md"]

# command (as written in the row's `backticks`) -> first-release date.
# Longest keys are matched first so e.g. "market ranking" never matches a
# "community rankings" row. Date = CHANGELOG version date of first appearance.
FEATURE_DATES = {
    "community boards": "2026-08-04",
    "market option-hours": "2026-08-04",
    "order funding": "2026-08-04",
    "tax ria": "2026-08-04",
    "account interest": "2026-08-04",
    "account commission": "2026-08-04",
    "market issues": "2026-08-03",
    "order autotrade": "2026-08-03",
    "market calendar": "2026-08-03",
    "account detail": "2026-07-25",
    "market news": "2026-07-25",
    "profit summary": "2026-07-25",
    "profit daily": "2026-07-25",
    "profit": "2026-07-22",
    "tax overseas": "2026-07-22",
    "accumulate list": "2026-07-21",
    "accumulate status": "2026-07-21",
    "portfolio dividends": "2026-06-19",
    "community rankings": "2026-06-19",
    "market sectors": "2026-06-19",
    "market themes": "2026-06-29",
    "market briefing": "2026-06-19",
    "market investors": "2026-06-19",
    "market earnings": "2026-06-19",
    "quote orderbook": "2026-06-04",
    "quote sellable": "2026-06-04",
    "quote commission": "2026-06-04",
    "market signals": "2026-06-04",
    "market screener": "2026-06-04",
    "market index": "2026-06-04",
    "market ranking": "2026-06-04",
    "quote flows": "2026-06-04",
    "watchlist list": "2026-06-04",  # 관심종목 관리(group CRUD) 0.5.0
    "market fx": "2026-06-03",
    "market hours": "2026-06-03",
    "quote trades": "2026-06-03",
    "quote limits": "2026-06-03",
    "quote warnings": "2026-06-03",
    "quote chart": "2026-05-20",
    "quote batch": "2026-03-21",
    "quote get": "2026-03-21",
    "orders list": "2026-03-21",
    "transactions list": "2026-04-23",
    "transactions overview": "2026-04-23",
    "export positions": "2026-03-21",
    "portfolio positions": "2026-04-23",
    "account list": "2026-03-21",
}
_KEYS = sorted(FEATURE_DATES, key=len, reverse=True)

# first table cell: | [**]['🆕 ']<name>[**] |
cell_re = re.compile(r"^(\| )(\*\*)?(?:🆕 )?(.*?)(\*\*)?( \|)")
# legacy inline date comments to strip (migrated into FEATURE_DATES)
since_re = re.compile(r"\s*<!--since:\d{4}-\d{2}-\d{2}-->")


def row_date(line):
    for k in _KEYS:
        if "`" + k in line:
            return FEATURE_DATES[k]
    return None


def main():
    check = "--check" in sys.argv
    today = datetime.date.fromisoformat(
        os.environ.get("NEW_MARKER_DATE") or datetime.date.today().isoformat()
    )
    cutoff = today - datetime.timedelta(days=NEW_WINDOW_DAYS)

    changed_any = False
    for path in FILES:
        if not os.path.exists(path):
            continue
        src = open(path, encoding="utf-8").read()
        lines = src.split("\n")
        new_count = 0
        for i, ln in enumerate(lines):
            if not ln.startswith("|") or "`" not in ln:
                continue
            ln = since_re.sub("", ln)  # drop legacy inline comments
            date = row_date(ln)
            if date is None:
                lines[i] = ln
                continue
            is_new = datetime.date.fromisoformat(date) >= cutoff
            if is_new:
                new_count += 1

            def repl(mm, is_new=is_new):
                pre, b1, name, b2, post = (
                    mm.group(1), mm.group(2) or "", mm.group(3), mm.group(4) or "", mm.group(5),
                )
                return f"{pre}{b1}{'🆕 ' if is_new else ''}{name}{b2}{post}"

            lines[i] = cell_re.sub(repl, ln, count=1)
        out = "\n".join(lines)
        if out != src:
            changed_any = True
            if not check:
                open(path, "w", encoding="utf-8").write(out)
        print(f"{path}: {new_count} rows marked 🆕 (released >= {cutoff})"
              + (" [WOULD CHANGE]" if check and out != src else ""))

    if check and changed_any:
        print("ERROR: 🆕 markers are stale — run tools/update_new_markers.py", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
