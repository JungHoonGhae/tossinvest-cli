#!/usr/bin/env python3
"""Verify a pinned APK against known ASC search/decompile results, without running it."""

import argparse
import datetime
import hashlib
import json
import os
import platform
import re
import signal
import shutil
import statistics
import subprocess
import sys
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
ASC = Path(__file__).with_name("run.sh")


def validate_profile(profile):
    if not isinstance(profile, dict):
        raise ValueError("profile must be an object")
    for key in ("package", "version", "apk_sha256"):
        if not isinstance(profile.get(key), str) or not profile[key].strip():
            raise ValueError(f"profile requires a nonempty {key}")
    if not re.fullmatch(r"[a-f0-9]{64}", profile["apk_sha256"]):
        raise ValueError("apk_sha256 must be a lowercase SHA-256 digest")
    cases = profile.get("cases")
    if not isinstance(cases, list) or not cases:
        raise ValueError("profile must contain at least one case")
    ids = set()
    for case in cases:
        if not isinstance(case, dict):
            raise ValueError("each case must be an object")
        name = case.get("id")
        if not isinstance(name, str) or not re.fullmatch(r"[a-z0-9][a-z0-9-]*", name) or name in ids:
            raise ValueError("case ids must be unique lowercase names, digits and hyphens")
        ids.add(name)
        args = case.get("args")
        if (not isinstance(args, list) or not args or not all(isinstance(arg, str) for arg in args)
                or args[0] not in {"getclass", "getmanifest", "findrefs"} or args.count("{apk}") != 1):
            raise ValueError(f"{name}: args must select a read command and contain one {{apk}}")
        required = case.get("required")
        if not isinstance(required, list) or not required or not all(isinstance(value, str) and value.strip() for value in required):
            raise ValueError(f"{name}: required must contain nonempty expected strings")
        coverage = case.get("coverage", {})
        if not isinstance(coverage, dict):
            raise ValueError(f"{name}: coverage must be an object")
        for key, pattern in coverage.items():
            if not key or not isinstance(pattern, str) or not pattern.strip():
                raise ValueError(f"{name}: coverage requires named, nonempty patterns")
            re.compile(pattern)


def inspect_output(case, output):
    return {
        "missing_required": [value for value in case["required"] if value not in output],
        "coverage": {
            name: re.search(pattern, output) is not None
            for name, pattern in case.get("coverage", {}).items()
        },
    }


def run_command(command, output_dir, name, timeout):
    start = time.perf_counter()
    stdout_path = output_dir / (name + ".stdout")
    stderr_path = output_dir / (name + ".stderr")
    with stdout_path.open("wb") as stdout, stderr_path.open("wb") as stderr:
        process = subprocess.Popen(command, stdout=stdout, stderr=stderr, start_new_session=True)
        try:
            code = process.wait(timeout=timeout)
        except subprocess.TimeoutExpired:
            # ASC can fork DEX workers. Stop the whole invocation, not just its parent.
            try:
                os.killpg(process.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            process.wait()
            code = "timeout"
    return {
        "exit_code": code,
        "seconds": round(time.perf_counter() - start, 4),
        "stdout_bytes": stdout_path.stat().st_size,
    }, stdout_path.read_text(errors="replace")


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("apk", type=Path)
    parser.add_argument("--profile", type=Path, default=Path(__file__).with_name("toss-5.275.0.json"))
    parser.add_argument("--output", type=Path, required=True, help="new output directory outside the repository")
    parser.add_argument("--runs", type=int, default=3)
    parser.add_argument("--timeout", type=int, default=180)
    parser.add_argument("--jadx", action="store_true", help="also time one fresh JADX single-class invocation")
    args = parser.parse_args(argv)
    if not 1 <= args.runs <= 10 or args.timeout <= 0:
        parser.error("runs must be 1..10 and timeout must be positive")
    output_dir = args.output.resolve()
    if output_dir.is_relative_to(ROOT):
        parser.error("keep APK analysis output outside the repository")
    try:
        profile = json.loads(args.profile.read_text())
        validate_profile(profile)
    except (ValueError, re.error) as error:
        parser.error(str(error))
    service = next((case for case in profile["cases"] if case["id"] == "service"), None)
    if args.jadx and (not shutil.which("jadx") or not service or service["args"][0] != "getclass" or len(service["args"]) < 3):
        parser.error("--jadx requires JADX and a service getclass case in the profile")
    apk = args.apk.resolve()
    with apk.open("rb") as source:
        digest = hashlib.file_digest(source, "sha256").hexdigest()
    if digest != profile["apk_sha256"]:
        parser.error("APK hash does not match the audited profile; verify provenance before creating a new profile")
    # Never overwrite prior evidence. The APK is read as data only.
    output_dir.mkdir(parents=True, exist_ok=False)
    report = {
        "created_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
        "package": profile["package"],
        "version": profile["version"],
        "apk_sha256": digest,
        "apk_bytes": apk.stat().st_size,
        "platform": platform.platform(),
        "lock_sha256": hashlib.sha256(ASC.with_name("requirements.txt").read_bytes()).hexdigest(),
        "timing_scope": "wall time per subprocess; repeated runs may use OS filesystem caches; no cold-cache claim",
        "cases": [],
    }
    failed = False
    limited = False
    for case in profile["cases"]:
        trials = []
        for run in range(args.runs):
            command = [str(ASC)] + [str(apk) if arg == "{apk}" else arg for arg in case["args"]]
            trial, output = run_command(command, output_dir, f'{case["id"]}-{run + 1}', args.timeout)
            trial.update(inspect_output(case, output))
            failed |= trial["exit_code"] != 0 or bool(trial["missing_required"])
            limited |= not all(trial["coverage"].values())
            trials.append(trial)
        report["cases"].append({
            "id": case["id"], "trials": trials,
            "median_seconds": round(statistics.median(trial["seconds"] for trial in trials), 4),
        })
        print(case["id"], json.dumps(report["cases"][-1], ensure_ascii=False), flush=True)
    if args.jadx:
        target = output_dir / "jadx-service.java"
        trial, _ = run_command([
            "jadx", "--no-res", "--single-class", service["args"][2],
            "--single-class-output", str(target), str(apk),
        ], output_dir, "jadx-service", args.timeout)
        trial.update(inspect_output(service, target.read_text() if target.exists() else ""))
        report["jadx_reference"] = trial
        print("jadx_reference", json.dumps(trial), flush=True)
    report["status"] = "failed" if failed else "partial" if limited else "passed"
    (output_dir / "report.json").write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
    print("status:", report["status"], "report:", output_dir / "report.json")
    # Partial preserves known discovery limitations; never present it as a full pass.
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
