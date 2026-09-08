import re
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
WORKFLOWS = ROOT / ".github" / "workflows"
AUTOMATION_PR_HELPER = ROOT / "tools" / "open_automation_pr.sh"
BOT_NAME = "github-actions[bot]"
# The numeric prefix links commits to GitHub's bot account. Without it, the
# main ruleset treats automation commits as unattributed and requires approval.
BOT_EMAIL = "41898282+github-actions[bot]@users.noreply.github.com"
UNATTRIBUTED_EMAIL = re.compile(
    r"(?<!41898282\+)github-actions\[bot\]@users\.noreply\.github\.com"
)


class AutomationIdentityTests(unittest.TestCase):
    def test_github_actions_commits_use_attributed_bot_email(self):
        offenders = []
        missing_email = []

        workflows = [*WORKFLOWS.glob("*.yml"), *WORKFLOWS.glob("*.yaml")]
        for workflow in sorted(workflows):
            source = workflow.read_text(encoding="utf-8")
            if UNATTRIBUTED_EMAIL.search(source):
                offenders.append(workflow.name)
            if f'git config user.name "{BOT_NAME}"' in source or (
                f"git config user.name '{BOT_NAME}'" in source
            ):
                if BOT_EMAIL not in source:
                    missing_email.append(workflow.name)

        self.assertEqual(
            offenders,
            [],
            "automation commits must use GitHub's account-linked bot email",
        )
        self.assertEqual(
            missing_email,
            [],
            "every workflow that commits as github-actions[bot] needs its attributed email",
        )

    def test_automation_prs_receive_their_label_at_creation(self):
        source = AUTOMATION_PR_HELPER.read_text(encoding="utf-8")
        self.assertIn(
            "--label maintenance",
            source,
            "GITHUB_TOKEN-created PRs cannot rely on a follow-up labeling workflow",
        )

    def test_automation_prs_authorize_the_attached_ci_run(self):
        source = (ROOT / "tools/approve_automation_ci.sh").read_text(encoding="utf-8")
        self.assertIn("--event pull_request", source)
        self.assertIn("actions/runs/$run_id/approve", source)
        self.assertNotIn(
            "gh workflow run ci.yml",
            source,
            "workflow_dispatch checks are not attached to the pull request",
        )

    def test_no_automation_workflow_skips_required_ci(self):
        for workflow in WORKFLOWS.glob("*.yml"):
            self.assertNotRegex(workflow.read_text(), r"(?i)\[(?:skip ci|ci skip|no ci|skip actions|actions skip)\]")

    def test_metadata_has_one_schedule_and_shared_refresh_concurrency(self):
        self.assertFalse((WORKFLOWS / "stats.yml").exists())
        self.assertFalse((WORKFLOWS / "star-history.yml").exists())
        self.assertFalse((WORKFLOWS / "sponsors.yml").exists())
        source = (WORKFLOWS / "repository-metadata.yml").read_text()
        for script in ["update_stats.py", "gen_star_history.py", "sponsors.py"]:
            self.assertIn(script, source)
        for family in ["repository-metadata", "wts-monitor", "daily-monitor"]:
            source = (WORKFLOWS / f"{family}.yml").read_text()
            self.assertIn(f"group: automation-{family}", source)
            self.assertLess(source.index("--prepare"), source.index("python3 tools/"))
            self.assertIn("fetch-depth: 0", source)

    def test_automation_does_not_duplicate_push_and_pr_ci(self):
        source = (WORKFLOWS / "ci.yml").read_text()
        self.assertNotIn('"automation/**"', source)
        self.assertIn("pull_request:", source)


if __name__ == "__main__":
    unittest.main()
