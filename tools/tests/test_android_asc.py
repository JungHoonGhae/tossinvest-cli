"""Offline checks for ASC isolation and evidence validation; no APK/decompiler needed."""

import contextlib
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location("asc_verify", ROOT / "tools/asc/verify.py")
V = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(V)


class TestASCVerification(unittest.TestCase):
    def make_profile(self, directory):
        apk = directory / "fixture.apk"
        apk.write_bytes(b"synthetic fixture; never executed")
        profile = directory / "profile.json"
        profile.write_text(json.dumps({
            "package": "example.test", "version": "1",
            "apk_sha256": hashlib.sha256(apk.read_bytes()).hexdigest(),
            "cases": [{"id": "service", "args": ["getclass", "{apk}", "example.Service"],
                       "required": ["/known/path"],
                       "coverage": {"parameters": "method\\(Body"}}],
        }))
        return apk, profile

    def test_wrong_apk_is_rejected_before_running_any_tool(self):
        with tempfile.TemporaryDirectory() as tmp:
            apk, profile = self.make_profile(Path(tmp))
            apk.write_bytes(b"different artifact")
            with patch.object(V, "run_command") as run, contextlib.redirect_stderr(io.StringIO()):
                with self.assertRaises(SystemExit):
                    V.main([str(apk), "--profile", str(profile), "--output", str(Path(tmp) / "out")])
                run.assert_not_called()

    def test_zero_exit_does_not_hide_missing_contract_details(self):
        with tempfile.TemporaryDirectory() as tmp:
            directory = Path(tmp)
            apk, profile = self.make_profile(directory)
            out = directory / "out"
            with patch.object(V, "run_command", return_value=(
                {"exit_code": 0, "seconds": 0.1, "stdout_bytes": 11}, "/known/path method()"
            )), contextlib.redirect_stdout(io.StringIO()):
                result = V.main([str(apk), "--profile", str(profile), "--output", str(out), "--runs", "1"])
            self.assertEqual(result, 0)
            report = json.loads((out / "report.json").read_text())
            self.assertEqual(report["status"], "partial")
            self.assertFalse(report["cases"][0]["trials"][0]["coverage"]["parameters"])

    def test_empty_success_is_a_failed_verification(self):
        with tempfile.TemporaryDirectory() as tmp:
            directory = Path(tmp)
            apk, profile = self.make_profile(directory)
            out = directory / "out"
            with patch.object(V, "run_command", return_value=(
                {"exit_code": 0, "seconds": 0.1, "stdout_bytes": 0}, ""
            )), contextlib.redirect_stdout(io.StringIO()):
                result = V.main([str(apk), "--profile", str(profile), "--output", str(out), "--runs", "1"])
            self.assertEqual(result, 1)
            self.assertEqual(json.loads((out / "report.json").read_text())["status"], "failed")

    def test_refuses_analysis_output_in_repository(self):
        with contextlib.redirect_stderr(io.StringIO()), patch.object(V, "run_command") as run:
            with self.assertRaises(SystemExit):
                V.main(["unused.apk", "--output", str(ROOT / "asc-output")])
            run.assert_not_called()

    def test_timeout_is_recorded(self):
        with tempfile.TemporaryDirectory() as tmp:
            trial, _ = V.run_command([sys.executable, "-c", "import time; time.sleep(10)"], Path(tmp), "timeout", 0.1)
            self.assertEqual(trial["exit_code"], "timeout")
            self.assertLess(trial["seconds"], 5)


class TestASCLauncher(unittest.TestCase):
    def test_stale_environment_does_not_execute(self):
        with tempfile.TemporaryDirectory() as tmp:
            env = Path(tmp)
            (env / "bin").mkdir()
            binary = env / "bin/droidasc"
            binary.write_text("#!/bin/sh\necho should-not-run\n")
            binary.chmod(0o755)
            (env / "tossctl-asc-requirements.txt").write_text("stale")
            result = subprocess.run([str(V.ASC), "--help"], env={**os.environ, "TOSSCTL_ASC_ENV": tmp}, capture_output=True, text=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertNotIn("should-not-run", result.stdout)
            self.assertIn("setup", result.stderr)

    def test_preserves_literal_class_name_and_spaces(self):
        with tempfile.TemporaryDirectory() as tmp:
            env = Path(tmp)
            (env / "bin").mkdir()
            binary = env / "bin/droidasc"
            binary.write_text('#!/bin/sh\nprintf "%s\\n" "$@"\n')
            binary.chmod(0o755)
            (env / "tossctl-asc-requirements.txt").write_bytes((ROOT / "tools/asc/requirements.txt").read_bytes())
            args = ["getclass", "an apk.apk", "example.Model$$serializer"]
            result = subprocess.run([str(V.ASC)] + args, env={**os.environ, "TOSSCTL_ASC_ENV": tmp}, capture_output=True, text=True, check=True)
            self.assertEqual(result.stdout.splitlines(), args)


if __name__ == "__main__":
    unittest.main()
