import pathlib
import re
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[2]


class TestReadmeVisuals(unittest.TestCase):
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
            for stem in ("readme-overview", "mcp-discovery", "order-safety"):
                asset = f"diagrams/{stem}{suffix}.png"
                with self.subTest(readme=readme_name, asset=asset):
                    self.assertIn(asset, readme)
                    self.assertTrue((ROOT / asset).is_file(), asset)
                    self.assertTrue((ROOT / asset).with_suffix(".html").is_file())
            self.assertIn('width="100%"', readme)
            self.assertIn("diagrams/README.md", readme)


if __name__ == "__main__":
    unittest.main()
