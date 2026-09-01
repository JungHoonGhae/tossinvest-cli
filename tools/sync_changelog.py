#!/usr/bin/env python3
"""Compatibility wrapper for the canonical website changelog generator.

The website's Node generator owns both MDX output formats. Keeping this legacy
entry point as a delegate prevents the two commands from producing different
documentation from the same CHANGELOG.md.
"""
from __future__ import annotations

import subprocess
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
GENERATOR = ROOT / "website-fumadocs" / "scripts" / "sync-changelog.mjs"


def main() -> None:
    subprocess.run(["node", str(GENERATOR)], cwd=ROOT, check=True)


if __name__ == "__main__":
    main()
