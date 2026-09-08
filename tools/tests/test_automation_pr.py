"""Exercise the real shell helper against disposable Git remotes and a fake gh."""
import json
import os
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
HELPER = ROOT / "tools/open_automation_pr.sh"
FAMILY = "automation/repository-metadata"
BRANCH = FAMILY + "-100-1"
BOT = "41898282+github-actions[bot]@users.noreply.github.com"


class AutomationPRTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.remote = self.root / "remote.git"
        self.repo = self.root / "repo"
        self.repo.mkdir()
        self.env = dict(os.environ, GH_TOKEN="fixture", GITHUB_REPOSITORY="owner/repo",
                        GITHUB_RUN_ID="200", GITHUB_RUN_ATTEMPT="1")
        self.git("init", "--bare", str(self.remote))
        self.git("init", "-b", "main")
        self.git("config", "user.name", "github-actions[bot]")
        self.git("config", "user.email", BOT)
        (self.repo / "docs").mkdir()
        (self.repo / "tools").mkdir()
        (self.repo / "tools/approve_automation_ci.sh").write_text((ROOT / "tools/approve_automation_ci.sh").read_text())
        (self.repo / "docs/.stats-downloads.json").write_text('{"2026-09-06":4804}\n')
        (self.repo / "README.md").write_text("base\n")
        self.git("add", ".")
        self.git("commit", "-m", "base")
        self.git("remote", "add", "origin", str(self.remote))
        self.git("push", "-u", "origin", "main")
        bindir = self.root / "bin"
        bindir.mkdir()
        gh = bindir / "gh"
        gh.write_text("""#!/usr/bin/env python3
import json, os, sys
args = sys.argv[1:]
with open(os.environ['FIXTURE_GH_LOG'], 'a') as log:
    log.write(json.dumps(args) + '\\n')
if args[:2] == ['pr', 'list']:
    print(os.environ.get('FIXTURE_PRS', '[]'))
elif args[:2] == ['pr', 'create']:
    print('https://github.com/owner/repo/pull/1')
elif args[:2] == ['run', 'list']:
    print('' if os.environ.get('FIXTURE_NO_RUN') else '123')
elif args[:2] == ['run', 'view']:
    print('completed' if 'status' in args else os.environ.get('FIXTURE_CONCLUSION', 'success'))
elif args[:1] == ['api']:
    pass
elif args[:2] != ['pr', 'merge']:
    sys.exit('unexpected gh call: ' + repr(args))
""")
        gh.chmod(0o755)
        self.env["PATH"] = str(bindir) + os.pathsep + self.env["PATH"]
        self.env["FIXTURE_GH_LOG"] = str(self.root / "gh.log")
        sleep = bindir / "sleep"
        sleep.write_text("#!/bin/sh\nexit 0\n")
        sleep.chmod(0o755)
        self.set_prs([])

    def git(self, *args, check=True):
        return subprocess.run(["git", *args], cwd=self.repo, env=self.env,
                              capture_output=True, text=True, check=check)

    def set_prs(self, prs):
        self.env["FIXTURE_PRS"] = json.dumps(prs)

    def open_pr(self, **overrides):
        pr = dict(url="https://github.com/owner/repo/pull/1", headRefName=BRANCH,
                  author={"login": "app/github-actions"}, isCrossRepository=False)
        pr.update(overrides)
        self.set_prs([pr])
        return pr

    def prepare(self):
        return subprocess.run(["bash", str(HELPER), "--prepare", FAMILY], cwd=self.repo,
                              env=self.env, capture_output=True, text=True)

    def publish(self):
        return subprocess.run(["bash", str(HELPER), FAMILY, "chore(metadata): fixture"],
                              cwd=self.repo, env=self.env, capture_output=True, text=True)

    def calls(self):
        return [json.loads(line) for line in (self.root / "gh.log").read_text().splitlines()]

    def test_publish_twice_reuses_existing_pr_and_approves_only_its_ci(self):
        self.assertEqual(self.prepare().returncode, 0)
        branch = self.git("branch", "--show-current").stdout.strip()
        for day in ["07", "08"]:
            (self.repo / "STATS.md").write_text("sample " + day)
            self.git("add", "STATS.md")
            self.git("commit", "-m", "daily sample")
            self.env["FIXTURE_CONCLUSION"] = "action_required"
            result = self.publish()
            self.assertEqual(result.returncode, 0, result.stderr)
            self.open_pr(headRefName=branch)
        calls = self.calls()
        self.assertEqual(sum(call[:2] == ["pr", "create"] for call in calls), 1)
        self.assertEqual(sum(call[:2] == ["pr", "merge"] for call in calls), 2)
        self.assertEqual(sum(call[:1] == ["api"] for call in calls), 2)
        for call in calls:
            if call[:2] == ["run", "list"]:
                self.assertIn("pull_request", call)
                self.assertIn("ci.yml", call)
                self.assertIn(branch, call)
                self.assertIn("headSha", call[call.index("--jq") + 1])

    def test_missing_ci_fails_without_queuing_merge(self):
        self.assertEqual(self.prepare().returncode, 0)
        (self.repo / "STATS.md").write_text("sample")
        self.git("add", "STATS.md")
        self.git("commit", "-m", "sample")
        self.env["FIXTURE_NO_RUN"] = "1"
        result = self.publish()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("CI pull_request run missing", result.stderr)
        self.assertFalse(any(call[:2] == ["pr", "merge"] for call in self.calls()))

    def test_exact_family_branch_is_accepted_in_prepare_and_publish(self):
        self.pending_branch()
        self.git("push", "origin", f"origin/{BRANCH}:refs/heads/{FAMILY}")
        self.open_pr(headRefName=FAMILY)
        self.assertEqual(self.prepare().returncode, 0)
        result = self.publish()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertFalse(any(call[:1] == ["api"] for call in self.calls()))

    def test_publish_rejects_a_remote_race_without_force(self):
        self.pending_branch()
        self.assertEqual(self.prepare().returncode, 0)
        other = self.root / "other"
        self.git("clone", "--branch", BRANCH, str(self.remote), str(other))
        for args in [("config", "user.name", "another writer"), ("config", "user.email", "fixture@example.invalid")]:
            subprocess.run(["git", *args], cwd=other, check=True, capture_output=True)
        (other / "README.md").write_text("concurrent update\n")
        for args in [("add", "README.md"), ("commit", "-m", "concurrent"), ("push", "origin", BRANCH)]:
            subprocess.run(["git", *args], cwd=other, check=True, capture_output=True)
        before = self.git("ls-remote", "origin", BRANCH).stdout
        (self.repo / "STATS.md").write_text("local sample")
        self.git("add", "STATS.md")
        self.git("commit", "-m", "local")
        result = self.publish()
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.git("ls-remote", "origin", BRANCH).stdout, before)
        self.assertFalse(any(call[:2] == ["pr", "merge"] for call in self.calls()))

    def pending_branch(self, file="docs/.stats-downloads.json", contents='{"2026-09-06":4804,"2026-09-07":4819}\n'):
        self.git("switch", "-c", BRANCH)
        (self.repo / file).write_text(contents)
        self.git("add", file)
        self.git("commit", "-m", "pending observation")
        self.git("push", "origin", BRANCH)
        self.git("switch", "main")
        self.git("branch", "-D", BRANCH)  # disposable test clone only
        self.open_pr()

    def test_prepare_reuses_pr_and_preserves_unpublished_observations(self):
        self.pending_branch()
        (self.repo / "README.md").write_text("updated main\n")
        self.git("add", "README.md")
        self.git("commit", "-m", "main advanced")
        self.git("push", "origin", "main")
        result = self.prepare()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.git("branch", "--show-current").stdout.strip(), BRANCH)
        self.assertEqual(json.loads((self.repo / "docs/.stats-downloads.json").read_text())["2026-09-07"], 4819)
        self.assertEqual((self.repo / "README.md").read_text(), "updated main\n")
        self.assertEqual(self.git("merge-base", "--is-ancestor", "origin/main", "HEAD").returncode, 0)

    def test_no_open_pr_prepares_new_branch_without_remote_write(self):
        self.assertEqual(self.prepare().returncode, 0)
        self.assertEqual(self.git("branch", "--show-current").stdout.strip(), FAMILY + "-200-1")
        self.assertEqual(self.git("ls-remote", "--heads", "origin", FAMILY + "-200-1").stdout, "")

    def test_duplicate_or_untrusted_prs_fail_before_checkout(self):
        for changes in [{"isCrossRepository": True}, {"author": {"login": "someone"}}]:
            self.open_pr(**changes)
            self.assertNotEqual(self.prepare().returncode, 0)
        pr = self.open_pr()
        self.set_prs([pr, pr])
        self.assertNotEqual(self.prepare().returncode, 0)
        self.assertEqual(self.git("branch", "--show-current").stdout.strip(), "main")

    def test_unexpected_file_is_rejected_before_checkout(self):
        self.pending_branch("unsafe.sh", "exit 0\n")
        result = self.prepare()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Unexpected automation file", result.stderr)
        self.assertEqual(self.git("branch", "--show-current").stdout.strip(), "main")

    def test_conflict_stops_without_changing_remote(self):
        self.pending_branch("README.md", "pending edit\n")
        before = self.git("ls-remote", "origin", BRANCH).stdout
        (self.repo / "README.md").write_text("different main edit\n")
        self.git("add", "README.md")
        self.git("commit", "-m", "conflicting main")
        self.git("push", "origin", "main")
        self.assertNotEqual(self.prepare().returncode, 0)
        self.assertEqual(self.git("ls-remote", "origin", BRANCH).stdout, before)


if __name__ == "__main__":
    unittest.main()
