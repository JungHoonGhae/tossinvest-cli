"""Network-free regression tests for tools/android_app_monitor.py."""

import json
import os
import sys
import tempfile
import unittest
from contextlib import redirect_stdout
from io import StringIO

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

import android_app_monitor as A  # noqa: E402


class TestAndroidAppMonitor(unittest.TestCase):
    def test_extract_versions_from_protobuf_like_bytes(self):
        payload = b"\x00Toss\x122\x075.274.1\x00url/Toss_v5.275.0_apkpure.xapk"
        self.assertEqual(A.extract_versions(payload), ["5.274.1", "5.275.0"])

    def test_semantic_version_order_is_numeric(self):
        self.assertGreater(A.version_key("5.100.0"), A.version_key("5.99.9"))

    def test_new_candidate_marks_audit_stale_without_promoting_artifact(self):
        state = {
            "package_id": A.PACKAGE_ID,
            "audited_artifact": {"version_name": "5.275.0", "sha256": "fixed"},
            "latest_candidate": {"version_name": "5.275.0"},
            "audit_status": "current",
        }
        diff = A.update_state(state, "5.276.0", "2026-09-10")
        self.assertTrue(diff["audit_stale"])
        self.assertTrue(diff["candidate_changed"])
        self.assertEqual(state["audited_artifact"]["version_name"], "5.275.0")
        self.assertEqual(state["audited_artifact"]["sha256"], "fixed")
        self.assertEqual(state["latest_candidate"]["version_name"], "5.276.0")

    def test_same_candidate_does_not_rewrite_state(self):
        state = {
            "package_id": A.PACKAGE_ID,
            "audited_artifact": {"version_name": "5.275.0"},
            "latest_candidate": {"version_name": "5.275.0", "first_seen": "2026-09-03"},
            "audit_status": "current",
        }
        before = json.dumps(state, sort_keys=True)
        diff = A.update_state(state, "5.275.0", "2026-09-10")
        self.assertFalse(diff["state_changed"])
        self.assertEqual(json.dumps(state, sort_keys=True), before)

    def test_offline_cli_writes_diff(self):
        with tempfile.TemporaryDirectory() as directory:
            state_path = os.path.join(directory, "state.json")
            metadata_path = os.path.join(directory, "metadata.bin")
            diff_path = os.path.join(directory, "diff.json")
            with open(state_path, "w", encoding="utf-8") as target:
                json.dump({
                    "package_id": A.PACKAGE_ID,
                    "audited_artifact": {"version_name": "5.275.0"},
                    "latest_candidate": {"version_name": "5.275.0"},
                    "audit_status": "current",
                }, target)
            with open(metadata_path, "wb") as target:
                target.write(b"release=5.276.0")
            stdout = StringIO()
            with redirect_stdout(stdout):
                self.assertEqual(A.main([
                    "--state", state_path,
                    "--metadata-file", metadata_path,
                    "--diff-out", diff_path,
                ]), 0)
            with open(diff_path, encoding="utf-8") as source:
                diff = json.load(source)
            self.assertTrue(diff["audit_stale"])
            self.assertEqual(diff["metadata_source"], "offline")
            self.assertIn("source offline", stdout.getvalue())


if __name__ == "__main__":
    unittest.main()
