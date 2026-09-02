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
HANDLE = re.compile(r"@([A-Za-z0-9][A-Za-z0-9-]*)")
CODE_SPAN = re.compile(r"`[^`\n]*`")
NON_CREDIT_HANDLES = {"latest"}  # Go module version suffix, not a GitHub mention.


class TestChangelogCredits(unittest.TestCase):
    def test_all_credit_handles_are_real_github_mentions(self):
        lines = CHANGELOG.read_text(encoding="utf-8").splitlines()
        sections = 0
        for line in lines:
            if CREDIT_HEADING.match(line):
                sections += 1

            for code_span in CODE_SPAN.findall(line):
                handles = set(HANDLE.findall(code_span)) - NON_CREDIT_HANDLES
                self.assertFalse(
                    handles,
                    f"a contributor handle must be plain Markdown text, not code: {line}",
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
