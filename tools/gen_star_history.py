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
import subprocess
from datetime import datetime, timezone

REPO = "JungHoonGhae/tossinvest-cli"
OUT_DIR = "docs/assets/star-history"
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
            out = subprocess.run(
                ["gh", "api", "-H", "Accept: application/vnd.github.star+json",
                 f"repos/{REPO}/stargazers?per_page=100&page={page}",
                 "--jq", ".[].starred_at"],
                capture_output=True, text=True,
            )
            lines = [l for l in out.stdout.split() if l.strip()]
            if not lines:
                break
            raw += lines
            if len(lines) < 100:
                break
            page += 1
    ts = [datetime.fromisoformat(x.strip().replace("Z", "+00:00")) for x in raw]
    ts.sort()
    return ts


def build(ts: list[datetime], theme: str) -> str:
    n = len(ts)
    t0, t1 = ts[0], ts[-1]
    span = max((t1 - t0).total_seconds(), 1)
    W, H = 800, 400
    PL, PR, PT, PB = 64, 24, 28, 48
    IW, IH = W - PL - PR, H - PT - PB
    maxy = n

    def x_of(t): return (t - t0).total_seconds() / span
    def sx(x): return PL + x * IW
    def sy(y): return PT + (1 - y / maxy) * IH

    pts = [(x_of(t), i + 1) for i, t in enumerate(ts)]
    yticks = list(range(0, maxy + 1, 100))
    if not yticks or yticks[-1] < maxy:
        yticks.append((yticks[-1] if yticks else 0) + 100)

    months = []
    cur = datetime(t0.year, t0.month, 1, tzinfo=timezone.utc)
    while cur <= t1:
        months.append(cur)
        y = cur.year + (cur.month // 12)
        m = cur.month % 12 + 1
        cur = datetime(y, m, 1, tzinfo=timezone.utc)

    if theme == "dark":
        grid, axis, line, fill, txt, dot = "#30363d", "#8b949e", "#e3b341", "#e3b34122", "#c9d1d9", "#e3b341"
    else:
        grid, axis, line, fill, txt, dot = "#e5e7eb", "#6b7280", "#d4a017", "#d4a01720", "#374151", "#d4a017"

    P = [f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W} {H}" width="{W}" height="{H}" '
         f'font-family="-apple-system,BlinkMacSystemFont,Segoe UI,Helvetica,Arial,sans-serif">']
    P.append(f'<rect width="{W}" height="{H}" fill="none"/>')
    P.append(f'<text x="{PL}" y="18" fill="{txt}" font-size="13" font-weight="600">Star History — {REPO}</text>')
    P.append(f'<text x="{W-PR}" y="18" fill="{axis}" font-size="11" text-anchor="end">{n} ★</text>')
    for yt in yticks:
        Y = sy(yt)
        P.append(f'<line x1="{PL}" y1="{Y:.1f}" x2="{W-PR}" y2="{Y:.1f}" stroke="{grid}" stroke-width="1"/>')
        P.append(f'<text x="{PL-8}" y="{Y+4:.1f}" fill="{axis}" font-size="10" text-anchor="end">{yt}</text>')
    for mo in months:
        X = max(PL, min(W - PR, sx(x_of(mo)) if mo >= t0 else PL))
        P.append(f'<text x="{X:.1f}" y="{H-PB+18}" fill="{axis}" font-size="10" text-anchor="middle">{MLABEL[mo.month-1]}</text>')
    d = "M " + " L ".join(f"{sx(x):.1f} {sy(y):.1f}" for x, y in pts)
    area = (f"M {sx(0):.1f} {sy(0):.1f} L "
            + " L ".join(f"{sx(x):.1f} {sy(y):.1f}" for x, y in pts)
            + f" L {sx(pts[-1][0]):.1f} {sy(0):.1f} Z")
    P.append(f'<path d="{area}" fill="{fill}" stroke="none"/>')
    P.append(f'<path d="{d}" fill="none" stroke="{line}" stroke-width="2.5" stroke-linejoin="round"/>')
    ex, ey = pts[-1]
    P.append(f'<circle cx="{sx(ex):.1f}" cy="{sy(ey):.1f}" r="4" fill="{dot}"/>')
    P.append("</svg>")
    return "\n".join(P)


def main():
    ts = fetch_timestamps()
    if not ts:
        raise SystemExit("no stargazer data")
    os.makedirs(OUT_DIR, exist_ok=True)
    for theme in ("dark", "light"):
        with open(f"{OUT_DIR}/star-history-{theme}.svg", "w") as f:
            f.write(build(ts, theme))
    print(f"{len(ts)} stars, {ts[0].date()} -> {ts[-1].date()}")


if __name__ == "__main__":
    main()
