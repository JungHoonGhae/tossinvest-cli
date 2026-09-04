import importlib.util
import unittest
from datetime import datetime, timedelta, timezone
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location(
    "gen_star_history", ROOT / "tools" / "gen_star_history.py"
)
MODULE = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(MODULE)


class TestStarHistory(unittest.TestCase):
    def test_versioned_asset_name_avoids_stale_readme_cache(self):
        self.assertEqual("star-history-v2", MODULE.OUT_BASENAME)

    def test_chart_matches_readme_visual_contract(self):
        start = datetime(2026, 1, 1, tzinfo=timezone.utc)
        timestamps = [start + timedelta(days=i) for i in range(125)]

        chart = MODULE.build(timestamps, "dark")

        self.assertIn('height="533.333"', chart)
        self.assertIn("#0d1117", chart)
        self.assertIn("#ff6b6b", chart)
        self.assertIn('baseFrequency=".05"', chart)
        self.assertIn('scale="5"', chart)
        self.assertIn(" C ", chart)
        self.assertIn("GitHub Stars", chart)
        self.assertIn("Star History", chart)
        self.assertIn('>t</text>', chart)
        self.assertIn(MODULE.REPO, chart)
        self.assertNotIn("<polygon", chart)

    def test_chart_has_light_theme_background(self):
        start = datetime(2026, 1, 1, tzinfo=timezone.utc)
        chart = MODULE.build([start, start + timedelta(days=1)], "light")

        self.assertIn('fill="#ffffff"', chart)
        self.assertIn("#24292f", chart)


if __name__ == "__main__":
    unittest.main()
