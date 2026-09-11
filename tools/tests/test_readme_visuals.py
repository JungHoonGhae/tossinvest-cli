import pathlib
import contextlib
import io
import json
import re
import tempfile
import unittest
from unittest import mock
import xml.etree.ElementTree as ET

from tools import build_readme_diagrams


ROOT = pathlib.Path(__file__).resolve().parents[2]


class TestReadmeVisuals(unittest.TestCase):
    def test_bilingual_copy_edits_rebuild_portable_html(self):
        copy = json.loads((ROOT / "diagrams/readme-content.json").read_text())
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            (root / "diagrams").mkdir()
            for locale in ("ko", "en"):
                for figure in ("features", "workflow", "overview"):
                    copy[locale][figure]["title"] = f'{locale} {figure}: <수정> & "edit"'
            (root / "diagrams/readme-content.json").write_text(
                json.dumps(copy, ensure_ascii=False), encoding="utf-8"
            )
            with mock.patch.object(build_readme_diagrams, "ROOT", root):
                with contextlib.redirect_stdout(io.StringIO()):
                    build_readme_diagrams.main()
            for locale, suffix in (("ko", ""), ("en", ".en")):
                for figure in ("features", "workflow", "overview"):
                    html = (root / f"diagrams/readme-{figure}{suffix}.html").read_text()
                    svg = ET.fromstring(re.search(r"<svg.*?</svg>", html, re.S).group())
                    self.assertEqual(svg[0].text, copy[locale][figure]["title"])
                    self.assertEqual(svg.attrib["aria-labelledby"].split()[0], svg[0].attrib["id"])

    def test_readmes_preserve_visuals_and_automation_markers(self):
        for name in ("README.md", "README.en.md"):
            with self.subTest(readme=name):
                readme = (ROOT / name).read_text(encoding="utf-8")
                for asset in (
                    "docs/assets/hero-banner-v5.png",
                    "docs/assets/demo/install.gif",
                    "docs/assets/demo/mcp.gif",
                    "docs/assets/star-history/star-history-v2-dark.svg",
                    "docs/assets/star-history/star-history-v2-light.svg",
                ):
                    self.assertIn(asset, readme)
                    self.assertTrue((ROOT / asset).is_file(), asset)
                self.assertIn(
                    "https://contrib.rocks/image?repo=JungHoonGhae/tossinvest-cli",
                    readme,
                )
                for section in ("sponsors", "star-history"):
                    start, end = f"<!-- {section}:start -->", f"<!-- {section}:end -->"
                    self.assertEqual(readme.count(start), 1)
                    self.assertEqual(readme.count(end), 1)
                    self.assertLess(readme.index(start), readme.index(end))

    def test_translations_keep_the_same_command_examples(self):
        examples = []
        for name in ("README.md", "README.en.md"):
            readme = (ROOT / name).read_text(encoding="utf-8")
            examples.append([
                line.split("#", 1)[0].strip()
                for block in re.findall(r"```bash\n(.*?)```", readme, re.DOTALL)
                for line in block.splitlines()
                if line.strip() and not line.startswith("#")
            ])
        self.assertTrue(examples[0])
        self.assertEqual(*examples)

    def test_readmes_link_localized_diagrams_and_editable_sources(self):
        for readme_name, suffix in (("README.md", ""), ("README.en.md", ".en")):
            readme = (ROOT / readme_name).read_text(encoding="utf-8")
            for stem in ("readme-features", "readme-workflow", "readme-overview"):
                asset = f"diagrams/{stem}{suffix}.png"
                with self.subTest(readme=readme_name, asset=asset):
                    self.assertIn(asset, readme)
                    self.assertTrue((ROOT / asset).is_file(), asset)
                    self.assertTrue((ROOT / asset).with_suffix(".html").is_file())
            self.assertIn('width="100%"', readme)


if __name__ == "__main__":
    unittest.main()
