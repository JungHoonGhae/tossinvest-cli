#!/usr/bin/env python3
"""Embed upstream Pretendard subsets in the Korean standalone diagrams.

Uses the unmodified v1.3.9 variable subsets and includes their OFL license.
Run again after changing Korean text, before render_readme_diagrams.py.
"""

import base64
from concurrent.futures import ThreadPoolExecutor
from html import unescape
from pathlib import Path
import re
from urllib.request import urlopen
from urllib.parse import urljoin


ROOT = Path(__file__).resolve().parents[1]
BASE = "https://cdn.jsdelivr.net/gh/orioncactus/pretendard@v1.3.9/"
CSS_URL = BASE + "dist/web/variable/pretendardvariable-dynamic-subset.css"
STEMS = ("readme-features", "readme-workflow", "readme-overview")
START = "<!-- pretendard-embed:start -->"
END = "<!-- pretendard-embed:end -->"


def download(url):
    with urlopen(url, timeout=30) as response:
        return response.read()


def main():
    css = download(CSS_URL).decode()
    license_text = download(BASE + "LICENSE").decode()
    blocks = []
    for block in re.findall(r"@font-face\s*\{[^}]+\}", css):
        ranges = re.search(r"unicode-range:\s*([^;]+);", block).group(1)
        points = set()
        for start, end in re.findall(r"U\+([0-9a-f]+)(?:-([0-9a-f]+))?", ranges, re.I):
            points.update(range(int(start, 16), int(end or start, 16) + 1))
        url = urljoin(CSS_URL, re.search(r"url\(([^)]+)\)", block).group(1))
        if not url.startswith(BASE + "packages/pretendard/dist/web/variable/"):
            raise ValueError(f"Unexpected upstream font URL: {url}")
        blocks.append((block, points, url))

    sources = []
    for stem in STEMS:
        file = ROOT / "diagrams" / f"{stem}.html"
        source = re.sub(re.escape(START) + r".*?" + re.escape(END) + r"\n?", "", file.read_text(), flags=re.S)
        content = re.search(r"<svg.*?</svg>", source, re.S).group()
        content += re.search(r'<p class="note">.*?</p>', source, re.S).group()
        points = {ord(char) for char in unescape(re.sub(r"<[^>]*>", "", content))}
        selected = [(block, url) for block, chars, url in blocks if points & chars]
        sources.append((file, source, selected))

    urls = sorted({url for _, _, selected in sources for _, url in selected})
    with ThreadPoolExecutor(max_workers=8) as pool:
        fonts = dict(zip(urls, pool.map(download, urls)))
    if any(not data.startswith(b"wOF2") for data in fonts.values()):
        raise ValueError("Expected unmodified upstream WOFF2 fonts")

    for file, source, selected in sources:
        embedded = []
        for block, url in selected:
            data = base64.b64encode(fonts[url]).decode()
            embedded.append(re.sub(r"url\([^)]+\)", f"url(data:font/woff2;base64,{data})", block))
        # Keep the complete license with each portable file. Decorative divider
        # lines are omitted so the license also forms a valid HTML comment.
        license_comment = "\n".join(line.rstrip() for line in license_text.splitlines() if not line.startswith("---"))
        font_block = f'{START}\n<!--\nSource: {CSS_URL}\n{license_comment}\n-->\n<style id="pretendard-fonts">\n' + "\n".join(embedded) + f"\n</style>\n{END}\n"
        source = source.replace("</head>", font_block + "</head>")
        file.write_text(source)
        print(f"Embedded {len(selected)} upstream Pretendard subsets in {file.name}")


if __name__ == "__main__":
    main()
