#!/usr/bin/env python3
"""Generate a self-hosted star-history chart (dark + light SVG).

star-history.com 무료 API가 이 repo 데이터 접근을 제한(GitHub restricted
starred-data access)하는 문제를 우회하기 위해, 실제 stargazer 타임스탬프로
누적 스타 곡선 SVG 를 직접 그려 docs/assets/star-history/ 에 저장한다.

Usage:
    python3 tools/gen_star_history.py            # gh CLI 로 스타 수집 후 생성
    STARS_FILE=stars.txt python3 tools/...        # 미리 뽑은 타임스탬프 파일 사용

주간 워크플로(.github/workflows/star-history.yml)에서 자동 갱신된다.
"""
import os
import time
import subprocess
from bisect import bisect_right
from datetime import datetime, timezone
from html import escape

REPO = "JungHoonGhae/tossinvest-cli"
OUT_DIR = "docs/assets/star-history"
OUT_BASENAME = "star-history-v2"
MLABEL = ["Jan", "Feb", "Mar", "Apr", "May", "Jun",
          "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"]


def fetch_timestamps() -> list[datetime]:
    """gh CLI 로 stargazer starred_at 을 페이지네이션하며 수집."""
    if os.environ.get("STARS_FILE"):
        raw = open(os.environ["STARS_FILE"]).read().split()
    else:
        raw = []
        page = 1
        while True:
            # Retry transient failures (secondary rate limits, hiccups). gh prints
            # the API error body to stdout on non-2xx, so we must check returncode
            # before treating stdout as data — otherwise `{"message":"..."}` gets
            # parsed as a timestamp and crashes.
            for attempt in range(3):
                out = subprocess.run(
                    ["gh", "api", "-H", "Accept: application/vnd.github.star+json",
                     f"repos/{REPO}/stargazers?per_page=100&page={page}",
                     "--jq", ".[].starred_at"],
                    capture_output=True, text=True,
                )
                if out.returncode == 0:
                    break
                time.sleep(2 * (attempt + 1))
            else:
                print(f"::warning::stargazer fetch failed: {out.stderr.strip()[:200]}", flush=True)
                return []  # signal: couldn't fetch — caller keeps existing charts
            lines = [l for l in out.stdout.split() if l.strip()]
            if not lines:
                break
            raw += lines
            if len(lines) < 100:
                break
            page += 1
    # Defensive: only parse ISO-like lines (a stray non-timestamp never crashes).
    ts = [datetime.fromisoformat(x.strip().replace("Z", "+00:00")) for x in raw if x[:2].isdigit()]
    ts.sort()
    return ts


def build(ts: list[datetime], theme: str) -> str:
    n = len(ts)
    t0, t1 = ts[0], ts[-1]
    span = max((t1 - t0).total_seconds(), 1)
    W, H = 800, 533.333
    PL, PR, PT, PB = 70, 30, 60, 50
    IW, IH = W - PL - PR, H - PT - PB
    step = 100 if n < 1000 else 1000
    maxy = max(step, n * 1.1)

    def x_of(t): return (t - t0).total_seconds() / span

    sample_count = min(50, max(12, n))
    sample_times = [t0 + (t1 - t0) * i / sample_count for i in range(sample_count + 1)]
    pts = [(x_of(t), bisect_right(ts, t)) for t in sample_times]
    pts[0] = (0.0, 0)
    yticks = list(range(0, int(maxy) + 1, step))

    tick_count = 8
    xticks = [t0 + (t1 - t0) * i / tick_count for i in range(tick_count + 1)]

    if theme == "dark":
        bg, txt, muted = "#0d1117", "#f0f6fc", "#f0f6fc"
    else:
        bg, txt, muted = "#ffffff", "#24292f", "#24292f"
    line = "#ff6b6b"
    font = "xkcd,'Comic Neue','Chalkboard SE','Comic Sans MS',sans-serif"

    def y_label(value: int) -> str:
        if value >= 1000:
            return f"{value // 1000}K" if value % 1000 == 0 else f"{value / 1000:.1f}K"
        return str(value)

    P = [f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W} {H}" width="{W}" height="{H}" '
         f'style="stroke-width:3;font-family:{font};background:{bg}">']
    P.append("<defs><filter id=\"rough\" x=\"-5\" y=\"-5\" width=\"100%\" height=\"100%\" filterUnits=\"userSpaceOnUse\">"
             "<feTurbulence type=\"fractalNoise\" baseFrequency=\".05\" result=\"noise\"/>"
             "<feDisplacementMap in=\"SourceGraphic\" in2=\"noise\" scale=\"5\" xChannelSelector=\"R\" yChannelSelector=\"G\"/>"
             "</filter></defs>")
    P.append(f'<rect width="{W}" height="{H}" fill="{bg}"/>')
    P.append(f'<g transform="translate({PL} {PT})">')

    # The roughened frame and coral line intentionally mirror OpenConnector's
    # self-hosted chart aesthetic while keeping the data source in this repo.
    P.append(f'<g fill="none" stroke="{muted}" stroke-linecap="round" filter="url(#rough)">')
    P.append(f'<path d="M 0 0 L 0 {IH:.1f} L {IW:.1f} {IH:.1f}"/>')
    P.append('</g>')
    for yt in yticks:
        Y = (1 - yt / maxy) * IH
        if yt:
            P.append(f'<text x="-10" y="{Y + 5:.1f}" fill="{txt}" font-size="16" font-weight="700" text-anchor="end">{y_label(yt)}</text>')
    for index, tick in enumerate(xticks):
        X = x_of(tick) * IW
        anchor = "start" if index == 0 else "end" if index == tick_count else "middle"
        P.append(f'<text x="{X:.1f}" y="{IH + 20:.1f}" fill="{txt}" font-size="15" font-weight="700" text-anchor="{anchor}">{MLABEL[tick.month - 1]} {tick.day:02d}</text>')

    coords = [(x * IW, (1 - y / maxy) * IH) for x, y in pts]
    d = f"M {coords[0][0]:.1f} {coords[0][1]:.1f}"
    for (x0, y0), (x1, y1) in zip(coords, coords[1:]):
        middle = (x0 + x1) / 2
        d += f" C {middle:.1f} {y0:.1f} {middle:.1f} {y1:.1f} {x1:.1f} {y1:.1f}"
    P.append(f'<path d="{d}" fill="none" stroke="{line}" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" filter="url(#rough)"/>')

    legend_width = max(210, len(REPO) * 9 + 36)
    P.append(f'<g transform="translate(12 10)" filter="url(#rough)">')
    P.append(f'<rect width="{legend_width}" height="34" rx="5" fill="{bg}" stroke="{txt}"/>')
    P.append(f'<rect x="10" y="11" width="11" height="11" rx="2" fill="{line}" stroke="none"/>')
    P.append(f'<text x="29" y="23" fill="{txt}" font-size="15" font-weight="700">{escape(REPO)}</text>')
    P.append('</g>')
    P.append(f'<text x="{IW / 2:.1f}" y="{IH + 43:.1f}" fill="{txt}" font-size="16" font-weight="700" text-anchor="middle">Date</text>')
    P.append(f'<text x="{-PL + 18}" y="{IH / 2:.1f}" fill="{txt}" font-size="16" font-weight="700" text-anchor="middle" transform="rotate(-90 {-PL + 18} {IH / 2:.1f})">GitHub Stars</text>')
    P.append('</g>')
    P.append('<circle cx="327" cy="23" r="11" fill="#3182f6"/>')
    P.append('<text x="327" y="28" fill="#ffffff" font-family="sans-serif" font-size="14" font-weight="700" text-anchor="middle">t</text>')
    P.append(f'<text x="400" y="30" fill="{txt}" font-size="20" font-weight="700" text-anchor="middle">Star History</text>')
    P.append("</svg>")
    return "\n".join(P)


def main():
    ts = fetch_timestamps()
    if not ts:
        # Transient fetch failure (rate limit, API hiccup) — keep the committed
        # charts and exit clean so the cron doesn't red-fail on a hiccup.
        print("no stargazer data (fetch failed or rate-limited); keeping existing charts")
        return
    os.makedirs(OUT_DIR, exist_ok=True)
    for theme in ("dark", "light"):
        with open(f"{OUT_DIR}/{OUT_BASENAME}-{theme}.svg", "w") as f:
            f.write(build(ts, theme))
    print(f"{len(ts)} stars, {ts[0].date()} -> {ts[-1].date()}")


if __name__ == "__main__":
    main()
