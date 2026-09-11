#!/usr/bin/env python3
"""Render the README's standalone HTML diagrams as PNGs.

Requires Python Playwright and its Chromium browser. Google Fonts must be
reachable; fail instead of silently exporting with substituted typography.
Run from any directory: python3 tools/render_readme_diagrams.py
"""

from pathlib import Path

from playwright.sync_api import sync_playwright


ROOT = Path(__file__).resolve().parents[1]
STEMS = ("readme-overview", "mcp-discovery", "order-safety")


def main():
    with sync_playwright() as playwright:
        browser = playwright.chromium.launch()
        try:
            page = browser.new_page(
                viewport={"width": 992, "height": 800}, device_scale_factor=2
            )
            for stem in STEMS:
                for suffix in ("", ".en"):
                    source = ROOT / "diagrams" / f"{stem}{suffix}.html"
                    page.goto(source.as_uri(), wait_until="networkidle")
                    fonts = page.evaluate("""async () => {
                        await document.fonts.ready;
                        const expected = document.documentElement.lang === 'ko'
                            ? ['Pretendard Variable', 'Geist Mono']
                            : ['Geist', 'Geist Mono', 'Instrument Serif'];
                        const loaded = new Set([...document.fonts]
                            .filter(font => font.status === 'loaded')
                            .map(font => font.family.replaceAll('"', '')));
                        return expected.filter(family => !loaded.has(family));
                    }""")
                    if fonts:
                        raise RuntimeError(f"{source.name}: fonts not loaded: {fonts}")
                    diagram = page.locator('svg[role="img"]')
                    if diagram.count() != 1:
                        raise RuntimeError(f"{source.name}: expected one accessible SVG")
                    target = source.with_suffix(".png")
                    diagram.screenshot(path=str(target), omit_background=True)
                    print(f"Rendered {target.relative_to(ROOT)} (1920×1200)")
        finally:
            browser.close()


if __name__ == "__main__":
    main()
