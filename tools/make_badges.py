#!/usr/bin/env python3
"""Generate the README custom badge SVGs (premium dark-glass pill style).

Single source of truth for the bespoke README badges. The geometry is
reverse-engineered from a high-end button set (taste-skill's webp buttons):
flat, wide pills (aspect ~4:1) that fill the frame with only a hair of margin,
a near-black neutral vertical gradient, a faint top sheen and a light rim — no
external drop shadow (that is what kept ours looking chunky and small). A
small uppercase gray label and a larger white value are split by a faint
divider. All sizes are expressed as ratios of the pill height PH so the look
is resolution-independent.

Static badges carry fixed values; the "vs Official API" / "spec checked"
badges read their values from docs/migration/.openapi-snapshot.json so they
stay fresh — the daily monitor re-runs this and commits the result. Also
emits a "Become a sponsor" CTA.

Usage: python3 tools/make_badges.py   (no args)
Output: docs/assets/badges/*.svg
"""
from __future__ import annotations

import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
OUT = ROOT / "docs" / "assets" / "badges"
SNAP = ROOT / "docs" / "migration" / ".openapi-snapshot.json"

# Pill geometry, in a 120-unit-tall coordinate space (ratios mirror taste:
# margin ~0.025·PH, radius ~0.13·PH, value cap ~0.22·PH, label cap ~0.15·PH,
# side padding ~0.60·PH). No drop shadow — the pill fills the frame.
PH = 120
SP = 3                       # transparent margin (AA room for the rim)
RX = 16                      # corner radius (~0.13·PH)
VALUE_FS = 37                # value font-size (~0.31·PH → cap ~0.22·PH)
LABEL_FS = 25                # label font-size (~0.21·PH → cap ~0.15·PH)
LABEL_LS = 4                 # label letter-spacing
PAD = 72                     # side padding per region (~0.60·PH)
MIN_LABEL = 190              # minimum label-region width
SANS = "ui-sans-serif,-apple-system,Segoe UI,Roboto,Helvetica,Arial,sans-serif"


def _lw(s: str) -> float:
    return len(s) * 18.0  # uppercase label advance incl. letter-spacing


def _vw(s: str) -> float:
    w = 0.0
    for c in s:
        if c == " ":
            w += 10.0
        elif c == "·":
            w += 13.0
        elif c == ".":
            w += 9.0
        elif c.isupper() or c.isdigit():
            w += 21.5
        else:
            w += 18.5
    return w


_DEFS = '''
 <defs>
  <linearGradient id="bg" x1="0" y1="0" x2="0" y2="1">
   <stop offset="0" stop-color="#1c1d1f"/><stop offset="0.5" stop-color="#101113"/><stop offset="1" stop-color="#0a0a0b"/>
  </linearGradient>
  <linearGradient id="gloss" x1="0" y1="0" x2="0" y2="1">
   <stop offset="0" stop-color="#ffffff" stop-opacity="0.07"/><stop offset="0.45" stop-color="#ffffff" stop-opacity="0"/>
  </linearGradient>
  <linearGradient id="rim" x1="0" y1="0" x2="0" y2="1">
   <stop offset="0" stop-color="#ffffff" stop-opacity="0.22"/><stop offset="0.5" stop-color="#ffffff" stop-opacity="0.10"/><stop offset="1" stop-color="#ffffff" stop-opacity="0.05"/>
  </linearGradient>
 </defs>'''


def _shell(pw: float, extra: str) -> str:
    VW, VH = int(pw + 2 * SP), PH + 2 * SP
    return (
        f'<svg xmlns="http://www.w3.org/2000/svg" width="{VW}" height="{VH}" viewBox="0 0 {VW} {VH}">{_DEFS}\n'
        f' <rect x="{SP}" y="{SP}" width="{pw:.1f}" height="{PH}" rx="{RX}" fill="url(#bg)"/>\n'
        f' <rect x="{SP}" y="{SP}" width="{pw:.1f}" height="{PH}" rx="{RX}" fill="url(#gloss)"/>\n'
        f' <rect x="{SP + 0.75}" y="{SP + 0.75}" width="{pw - 1.5:.1f}" height="{PH - 1.5}" rx="{RX - 0.75:.1f}" fill="none" stroke="url(#rim)" stroke-width="1.5"/>\n'
        f'{extra}\n</svg>\n'
    )


def badge(fn: str, label: str, value: str) -> None:
    lreg = max(_lw(label) + 2 * PAD, MIN_LABEL)
    vreg = _vw(value) + 2 * PAD
    pw = lreg + vreg
    cy = SP + PH / 2
    divx = SP + lreg
    lcx, vcx = SP + lreg / 2, divx + vreg / 2
    extra = (
        f' <line x1="{divx:.1f}" y1="{SP + 0.32 * PH:.1f}" x2="{divx:.1f}" y2="{SP + 0.68 * PH:.1f}" stroke="#ffffff" stroke-opacity="0.15" stroke-width="2"/>\n'
        f' <text x="{lcx:.1f}" y="{cy:.1f}" text-anchor="middle" dominant-baseline="central" font-family="{SANS}" font-size="{LABEL_FS}" font-weight="500" letter-spacing="{LABEL_LS}" fill="#8b8d94">{label.upper()}</text>\n'
        f' <text x="{vcx:.1f}" y="{cy:.1f}" text-anchor="middle" dominant-baseline="central" font-family="{SANS}" font-size="{VALUE_FS}" font-weight="600" fill="#fafafa">{value}</text>'
    )
    (OUT / fn).write_text(_shell(pw, extra))
    print(f"  {fn}  {label} | {value}")


def cta(fn: str, text: str) -> None:
    """Single-region call-to-action pill with a pink heart (sponsor)."""
    pw = 46 + _vw(text) + 2 * PAD  # 46 = heart glyph room
    cy, cx = SP + PH / 2, SP + pw / 2
    extra = (
        f' <text x="{cx:.1f}" y="{cy:.1f}" text-anchor="middle" dominant-baseline="central" font-family="{SANS}" font-size="{VALUE_FS - 4}" font-weight="600" fill="#fafafa">'
        f'<tspan fill="#ec6cb9" font-size="{VALUE_FS}">♥</tspan>  {text}</text>'
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
