#!/usr/bin/env python3
"""Generate the README custom badge SVGs (premium dark-glass pill style).

Single source of truth for the bespoke README badges. Style mirrors a high-end
button set: a near-black pill with a soft top-gloss gradient, rim highlight and
drop shadow, a centered uppercase gray label and a centered white value split
by a faint divider. Static badges carry fixed values; the two "verified vs
official API" / "spec checked" badges read their values from
docs/migration/.openapi-snapshot.json so they stay fresh — the daily monitor
re-runs this and commits the result. Also emits a "Become a sponsor" CTA.

Usage: python3 tools/make_badges.py   (no args)
Output: docs/assets/badges/*.svg
"""
from __future__ import annotations

import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
OUT = ROOT / "docs" / "assets" / "badges"
SNAP = ROOT / "docs" / "migration" / ".openapi-snapshot.json"

PH, SP, RX = 44, 6, 11  # pill height, shadow padding, corner radius


def _lw(s: str) -> float:
    return len(s) * 7.2  # monospace label (incl. letter-spacing)


def _vw(s: str) -> float:
    return sum(9.2 if c.isupper() else (3.8 if c in " ·." else 7.6) for c in s)


_DEFS = '''
 <defs>
  <linearGradient id="bg" x1="0" y1="0" x2="0" y2="1">
   <stop offset="0" stop-color="#2a2c37"/><stop offset="0.5" stop-color="#16171d"/><stop offset="1" stop-color="#0c0d11"/>
  </linearGradient>
  <linearGradient id="gloss" x1="0" y1="0" x2="0" y2="1">
   <stop offset="0" stop-color="#ffffff" stop-opacity="0.12"/><stop offset="0.5" stop-color="#ffffff" stop-opacity="0"/>
  </linearGradient>
  <filter id="sh" x="-15%" y="-15%" width="130%" height="150%">
   <feDropShadow dx="0" dy="2" stdDeviation="3" flood-color="#000000" flood-opacity="0.45"/>
  </filter>
 </defs>'''


def _shell(pw: float, extra: str) -> str:
    VW, VH = int(pw + 2 * SP), PH + 2 * SP
    return (
        f'<svg xmlns="http://www.w3.org/2000/svg" width="{VW}" height="{VH}" viewBox="0 0 {VW} {VH}">{_DEFS}\n'
        f' <g filter="url(#sh)"><rect x="{SP}" y="{SP}" width="{pw:.1f}" height="{PH}" rx="{RX}" fill="url(#bg)"/></g>\n'
        f' <rect x="{SP}" y="{SP}" width="{pw:.1f}" height="{PH}" rx="{RX}" fill="url(#gloss)"/>\n'
        f' <rect x="{SP + 0.5}" y="{SP + 0.5}" width="{pw - 1:.1f}" height="{PH - 1}" rx="{RX}" fill="none" stroke="#ffffff" stroke-opacity="0.12" stroke-width="1"/>\n'
        f'{extra}\n</svg>\n'
    )


def badge(fn: str, label: str, value: str) -> None:
    lpad, vpad, minlabel = 18, 22, 78
    lreg = max(_lw(label) + 2 * lpad, minlabel)
    vreg = _vw(value) + 2 * vpad
    pw = lreg + vreg
    cy = SP + PH / 2
    divx = SP + lreg
    lcx, vcx = SP + lreg / 2, divx + vreg / 2
    extra = (
        f' <line x1="{divx:.1f}" y1="{SP + 12}" x2="{divx:.1f}" y2="{SP + PH - 12}" stroke="#ffffff" stroke-opacity="0.22" stroke-width="1.2"/>\n'
        f' <text x="{lcx:.1f}" y="{cy:.1f}" text-anchor="middle" dominant-baseline="central" font-family="ui-monospace,SFMono-Regular,Menlo,monospace" font-size="11" letter-spacing="1.5" fill="#9ea0aa">{label.upper()}</text>\n'
        f' <text x="{vcx:.1f}" y="{cy:.1f}" text-anchor="middle" dominant-baseline="central" font-family="ui-sans-serif,-apple-system,Segoe UI,Roboto,sans-serif" font-size="15" font-weight="600" fill="#fafafa">{value}</text>'
    )
    (OUT / fn).write_text(_shell(pw, extra))
    print(f"  {fn}  {label} | {value}")


def cta(fn: str, text: str) -> None:
    """Single-region call-to-action pill with a pink heart (sponsor)."""
    pad = 22
    pw = 16 + _vw(text) + 2 * pad  # 16 = heart glyph room
    cy, cx = SP + PH / 2, SP + pw / 2
    extra = (
        f' <text x="{cx:.1f}" y="{cy:.1f}" text-anchor="middle" dominant-baseline="central" font-family="ui-sans-serif,-apple-system,Segoe UI,Roboto,sans-serif" font-size="14" font-weight="600" fill="#fafafa">'
        f'<tspan fill="#db61a2">♥</tspan>  {text}</text>'
    )
    (OUT / fn).write_text(_shell(pw, extra))
    print(f"  {fn}  ♥ {text}")


def main() -> None:
    OUT.mkdir(parents=True, exist_ok=True)

    badge("license.svg", "License", "MIT")
    badge("go.svg", "Built with", "Go 1.25+")
    badge("agents.svg", "Works with", "Claude · Codex · Cursor")
    badge("output.svg", "Output", "JSON · CSV · SSE")
    badge("hybrid.svg", "Official API", "Optional hybrid")

    spec_version, last_checked = "?", "?"
    if SNAP.exists():
        snap = json.loads(SNAP.read_text())
        spec_version = str(snap.get("spec_version", "?"))
        last_checked = str(snap.get("last_checked_at", "?"))
    badge("verified-api.svg", "vs Official API", f"v{spec_version} verified")
    badge("spec-checked.svg", "Spec checked", last_checked)

    cta("sponsor.svg", "Become a sponsor")


if __name__ == "__main__":
    main()
