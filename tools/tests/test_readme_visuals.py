import pathlib
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[2]


class TestReadmeVisuals(unittest.TestCase):
    def test_readmes_use_versioned_polished_routing_diagrams(self):
        cases = (
            ("README.md", "diagrams/official-vs-wts-v2.svg"),
            ("README.en.md", "diagrams/official-vs-wts-v2.en.svg"),
        )
        for readme_name, asset in cases:
            with self.subTest(readme=readme_name):
                readme = (ROOT / readme_name).read_text(encoding="utf-8")
                self.assertIn(asset, readme)
                self.assertIn('width="100%"', readme)

    def test_diagrams_keep_the_compact_dark_visual_contract(self):
        for asset in (
            "diagrams/official-vs-wts-v2.svg",
            "diagrams/official-vs-wts-v2.en.svg",
        ):
            with self.subTest(asset=asset):
                svg = (ROOT / asset).read_text(encoding="utf-8")
                self.assertIn('width="1600" height="720"', svg)
                self.assertIn('fill="#111418"', svg)
                self.assertIn("#3182f6", svg)
                self.assertIn("WTS ONLY", svg)


if __name__ == "__main__":
    unittest.main()
