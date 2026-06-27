#!/usr/bin/env python3
"""Generate the README custom dark-pill badge SVGs.

Single source of truth for the bespoke badges in README (taste-skill style:
dark pill, gray uppercase label + white value, green brand accent). Static
badges carry fixed values; the two "verified vs official API" / "spec checked"
badges read their values from docs/migration/.openapi-snapshot.json so they
stay fresh — the daily monitor re-runs this and commits the result.

Usage: python3 tools/make_badges.py   (no args)
Output: docs/assets/badges/*.svg
"""
from __future__ import annotations

import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
OUT = ROOT / "docs" / "assets" / "badges"
SNAP = ROOT / "docs" / "migration" / ".openapi-snapshot.json"

ACCENT = "#34D399"  # tossctl brand green


def _w_label(s: str) -> float:
    return len(s) * 6.5


def _w_value(s: str) -> float:
    return sum(8.4 if c.isupper() else (3.4 if c in " ·." else 7.0) for c in s)


def badge(filename: str, label: str, value: str, accent: str = ACCENT) -> None:
    H, r, acc = 46, 9, 3
    pad_l, gap_ld, gap_dv, pad_r = 14, 12, 14, 16
    lw, vw = _w_label(label), _w_value(value)
    x_label = acc + pad_l
    x_div = x_label + lw + gap_ld
    x_value = x_div + gap_dv
    W = int(x_value + vw + pad_r)
    cy = H / 2
    svg = f'''<svg xmlns="http://www.w3.org/2000/svg" width="{W}" height="{H}" viewBox="0 0 {W} {H}" role="img" aria-label="{label}: {value}">
  <defs><clipPath id="r"><rect width="{W}" height="{H}" rx="{r}"/></clipPath></defs>
  <g clip-path="url(#r)">
    <rect width="{W}" height="{H}" fill="#15161a"/>
    <rect x="0" y="0" width="{acc}" height="{H}" fill="{accent}"/>
    <line x1="{x_div:.1f}" y1="11" x2="{x_div:.1f}" y2="{H - 11}" stroke="#2b2d34" stroke-width="1"/>
  </g>
  <rect x="0.5" y="0.5" width="{W - 1}" height="{H - 1}" rx="{r}" fill="none" stroke="#2b2d34" stroke-width="1"/>
  <text x="{x_label:.1f}" y="{cy:.1f}" dominant-baseline="central" font-family="ui-monospace,SFMono-Regular,Menlo,monospace" font-size="10.5" letter-spacing="0.8" fill="#8b8d98">{label.upper()}</text>
  <text x="{x_value:.1f}" y="{cy:.1f}" dominant-baseline="central" font-family="ui-sans-serif,-apple-system,Segoe UI,Roboto,sans-serif" font-size="13" font-weight="600" fill="#f2f3f5">{value}</text>
</svg>
'''
    (OUT / filename).write_text(svg)
    print(f"  {filename}  ({W}x{H})  {label} | {value}")


def main() -> None:
    OUT.mkdir(parents=True, exist_ok=True)

    # Static badges
    badge("license.svg", "License", "MIT")
    badge("go.svg", "Built with", "Go 1.25+")
    badge("agents.svg", "Works with", "Claude · Codex · Cursor")
    badge("output.svg", "Output", "JSON · CSV · SSE")
    badge("hybrid.svg", "Official API", "Optional hybrid")

    # Dynamic badges — values from the verified Open API snapshot
    spec_version, last_checked = "?", "?"
    if SNAP.exists():
        snap = json.loads(SNAP.read_text())
        spec_version = str(snap.get("spec_version", "?"))
        last_checked = str(snap.get("last_checked_at", "?"))
    badge("verified-api.svg", "vs Official API", f"v{spec_version} verified")
    badge("spec-checked.svg", "Spec checked", last_checked)


if __name__ == "__main__":
    main()
