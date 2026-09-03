"""Regression tests for README new-feature marker release state."""

import os
import sys
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

import update_new_markers as markers  # noqa: E402


class TestNewMarkers(unittest.TestCase):
    def test_unreleased_commands_are_not_given_fake_release_dates(self):
        for command in markers.UNRELEASED_FEATURES:
            with self.subTest(command=command):
                self.assertNotIn(command, markers.FEATURE_DATES)
                self.assertEqual(markers.row_date(f"| feature | `{command}` |"), "unreleased")


if __name__ == "__main__":
    unittest.main()
