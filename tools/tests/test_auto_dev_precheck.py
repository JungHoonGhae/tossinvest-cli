import os
import sys
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

import auto_dev_precheck as P  # noqa: E402


class RemovedImplementedTests(unittest.TestCase):
    def test_only_implemented_paths_that_disappeared_count(self):
        previous = {
            "/api/v1/kept": {"status": "implemented"},
            "/api/v1/gone": {"status": "implemented"},
            "/api/v1/candidate-gone": {"status": "candidate"},
        }
        current = {"/api/v1/kept": {"status": "implemented"}}
        self.assertEqual(P.removed_implemented(previous, current), ["/api/v1/gone"])

    def test_empty_previous_catalog_reports_nothing(self):
        self.assertEqual(P.removed_implemented({}, {"/x": {"status": "implemented"}}), [])


class EligibleIssueTests(unittest.TestCase):
    def issue(self, number, created, *labels, state="OPEN"):
        return {"number": number, "createdAt": created, "state": state,
                "labels": [{"name": name} for name in labels]}

    def test_oldest_unblocked_issue_wins(self):
        issues = [
            self.issue(3, "2026-11-03T00:00:00Z", "auto-dev"),
            self.issue(1, "2026-11-01T00:00:00Z", "auto-dev", "ready-for-human"),
            self.issue(2, "2026-11-02T00:00:00Z", "auto-dev", "auto-dev:in-progress"),
            self.issue(4, "2026-11-04T00:00:00Z", "auto-dev"),
        ]
        self.assertEqual(P.eligible_issue(issues)["number"], 3)

    def test_no_work_when_everything_is_blocked_or_closed(self):
        issues = [
            self.issue(1, "2026-11-01T00:00:00Z", "auto-dev", "ready-for-human"),
            self.issue(2, "2026-11-02T00:00:00Z", "auto-dev", state="CLOSED"),
        ]
        self.assertIsNone(P.eligible_issue(issues))


if __name__ == "__main__":
    unittest.main()
