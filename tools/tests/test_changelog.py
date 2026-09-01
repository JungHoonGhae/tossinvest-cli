"""Release-credit formatting guards.

GitHub only turns a username into a notification mention when the @handle is
plain Markdown text. Keeping it inside a code span makes it look right while
silently failing to notify the contributor.
"""

import pathlib
import re
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[2]
CHANGELOG = ROOT / "CHANGELOG.md"
CREDIT_HEADING = re.compile(r"^### (?:기여자|Contributors?)\s*$")
HANDLE = re.compile(r"@([A-Za-z0-9][A-Za-z0-9-]*)")
CODE_SPAN = re.compile(r"`[^`\n]*`")


class TestChangelogCredits(unittest.TestCase):
    def test_credit_handles_are_real_github_mentions(self):
        lines = CHANGELOG.read_text(encoding="utf-8").splitlines()
        in_credit_section = False
        sections = 0
        for line in lines:
            if CREDIT_HEADING.match(line):
                in_credit_section = True
                sections += 1
                continue
            if line.startswith("### "):
                in_credit_section = False
            if not in_credit_section:
                continue

            handles = HANDLE.findall(line)
            if not handles:
                continue
            outside_code = CODE_SPAN.sub("", line)
            for handle in handles:
                self.assertIn(
                    f"@{handle}",
                    outside_code,
                    f"@{handle} in a contributor credit must be plain text: {line}",
                )
        self.assertGreater(sections, 0, "CHANGELOG must contain a contributor section")


if __name__ == "__main__":
    unittest.main()
