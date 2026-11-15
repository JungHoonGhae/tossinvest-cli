"""Tests for the tossctl agent-skill operation-catalog generator.

render() is a pure function (no binary, no network) so the dangerous property —
deterministic output regardless of input order — is testable. CI runs this with
PYTHONHASHSEED 0 and 1; a non-deterministic render would surface a spurious
"skill out of sync" failure on every registry change.
"""

import os
import sys
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

import build_skill as B  # noqa: E402

OPS = [
    {"id": "holdings", "category": "account", "backend": "wts",
     "summary": "Current stock holdings."},
    {"id": "buying_power", "category": "account", "backend": "wts",
     "required": ["currency"], "summary": "Cash buying power for a currency."},
    {"id": "place_order", "category": "order", "backend": "wts", "write": True,
     "required": ["symbol", "side"], "summary": "Place a new order | gated."},
    {"id": "sellable_quantity", "category": "order", "backend": "auto",
     "required": ["symbol"], "summary": "Sellable quantity.\nServed by either."},
    {"id": "auth_status", "category": "system", "backend": "none",
     "summary": "Which backends are connected."},
]


class TestRender(unittest.TestCase):
    def test_output_is_order_independent(self):
        # The registry iteration order must not change the file, or CI diffs it
        # on every run and the sync check flaps.
        a = B.render(OPS)
        b = B.render(list(reversed(OPS)))
        self.assertEqual(a, b)

    def test_categories_and_ids_are_sorted(self):
        out = B.render(OPS)
        self.assertLess(out.index("## account"), out.index("## order"))
        self.assertLess(out.index("## order"), out.index("## system"))
        # ids sorted within a category
        self.assertLess(out.index("`buying_power`"), out.index("`holdings`"))

    def test_pipe_and_newline_are_escaped(self):
        out = B.render(OPS)
        self.assertIn("Place a new order \\| gated.", out)  # | escaped
        self.assertIn("Sellable quantity. Served by either.", out)  # newline → space
        self.assertNotIn("either.\n", out.split("Sellable quantity")[1][:40])

    def test_write_and_required_columns(self):
        out = B.render(OPS)
        # place_order row marks write and lists required params
        row = next(l for l in out.splitlines() if l.startswith("| `place_order`"))
        self.assertIn("| yes |", row)
        self.assertIn("symbol, side", row)
        # a read op leaves write blank
        hold = next(l for l in out.splitlines() if l.startswith("| `holdings`"))
        self.assertRegex(hold, r"\|\s*\|")  # empty write cell

    def test_missing_backend_defaults_to_wts(self):
        out = B.render([{"id": "x", "category": "c", "summary": "s"}])
        self.assertIn("| `x` | wts |", out)

    def test_generated_note_present(self):
        self.assertTrue(B.render(OPS).startswith(B.GENERATED_NOTE))


class TestManagedFiles(unittest.TestCase):
    """managed_files() decides what gets written where. The embedded copy the
    binary ships must receive the same catalog as the distributable one, or
    `tossctl skill install` would hand agents a stale skill."""

    def test_catalog_goes_to_both_homes(self):
        files = B.managed_files("CATALOG", "SKILLMD")
        self.assertEqual(files[B.OUT], "CATALOG")
        self.assertEqual(files[B.EMBED_OPS], "CATALOG")

    def test_skill_md_mirrored_verbatim(self):
        files = B.managed_files("CATALOG", "SKILLMD")
        self.assertEqual(files[B.EMBED_SKILL], "SKILLMD")

    def test_exactly_three_managed_paths(self):
        files = B.managed_files("c", "s")
        self.assertEqual(set(files), {B.OUT, B.EMBED_OPS, B.EMBED_SKILL})

    def test_source_skill_is_never_written(self):
        # SRC_SKILL is hand-authored input, not an output this script owns —
        # writing it would let the generator clobber the canonical source.
        self.assertNotIn(B.SRC_SKILL, B.managed_files("c", "s"))


if __name__ == "__main__":
    unittest.main()
