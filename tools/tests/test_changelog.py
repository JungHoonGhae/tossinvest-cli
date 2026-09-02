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
WEBSITE_CHANGELOGS = (
    ROOT / "website-fumadocs/content/docs/changelog.mdx",
    ROOT / "website-fumadocs/content/docs/changelog.en.mdx",
)
CREDIT_HEADING = re.compile(r"^### (?:기여자|Contributors?)\s*$")
CREDIT_MARKER = re.compile(r"기여|제보|감사|contribut|thank", re.IGNORECASE)
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
            if not in_credit_section and not CREDIT_MARKER.search(line):
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

    def test_website_changelogs_match_the_source(self):
        source = CHANGELOG.read_text(encoding="utf-8")
        source_body = source[source.index("## [") :].strip()
        for generated in WEBSITE_CHANGELOGS:
            with self.subTest(generated=generated.name):
                content = generated.read_text(encoding="utf-8")
                generated_body = content[content.index("## [") :].strip()
                self.assertEqual(
                    generated_body,
                    source_body,
                    f"{generated} is stale; run tools/sync_changelog.py",
                )


if __name__ == "__main__":
    unittest.main()
