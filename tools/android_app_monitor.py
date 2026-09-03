#!/usr/bin/env python3
"""Detect when the general Toss Android app has moved past our static audit.

The Google Play page does not expose a stable machine-readable version field.
This monitor therefore treats APKPure metadata as an *untrusted candidate
signal*: a newer version marks the audit stale, but it is never considered an
audited artifact until a human/agent downloads it, verifies provenance and
signature continuity, and updates android-app.json.

stdlib only so the weekly GitHub Actions monitor needs no package install.
"""

import argparse
import datetime
import json
import os
import re
import sys
import urllib.parse
import urllib.request


DEFAULT_STATE = os.path.join("docs", "reverse-engineering", "android-app.json")
API = "https://api.pureapk.com/m/v3/cms/app_version"
PACKAGE_ID = "viva.republica.toss"
VERSION_RE = re.compile(rb"(?<![0-9])([0-9]+(?:\.[0-9]+){2,3})(?![0-9])")


def version_key(version):
    parts = tuple(int(part) for part in version.split("."))
    return parts + (0,) * (4 - len(parts))


def extract_versions(payload):
    """Extract semantic-looking version names from APKPure's protobuf body."""
    versions = {match.decode("ascii") for match in VERSION_RE.findall(payload)}
    return sorted(versions, key=version_key)


def fetch_metadata(package_id=PACKAGE_ID):
    query = urllib.parse.urlencode({"hl": "en-US", "package_name": package_id})
    request = urllib.request.Request(
        API + "?" + query,
        headers={
            "User-Agent": "tossctl-android-version-monitor/1",
            "x-cv": "3172501",
            "x-sv": "29",
            "x-abis": "arm64-v8a,armeabi-v7a,armeabi,x86,x86_64;",
            "x-gp": "1",
        },
    )
    with urllib.request.urlopen(request, timeout=25) as response:
        return response.read()


def evaluate(audited_version, candidate_version):
    if version_key(candidate_version) > version_key(audited_version):
        return "stale"
    if version_key(candidate_version) < version_key(audited_version):
        return "source_behind"
    return "current"


def update_state(state, candidate_version, today):
    audited_version = state["audited_artifact"]["version_name"]
    previous = state.get("latest_candidate", {}).get("version_name", "")
    status = evaluate(audited_version, candidate_version)
    changed = previous != candidate_version or state.get("audit_status") != status
    if changed:
        state["latest_candidate"] = {
            "version_name": candidate_version,
            "source": "APKPure metadata (untrusted candidate signal)",
            "first_seen": today,
        }
        state["audit_status"] = status
    return {
        "candidate_changed": previous != candidate_version,
        "previous_candidate": previous,
        "current_candidate": candidate_version,
        "audited_version": audited_version,
        "audit_status": status,
        "audit_stale": status == "stale",
        "state_changed": changed,
        "checked_at": today,
    }


def parse_args(argv):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--state", default=DEFAULT_STATE)
    parser.add_argument("--metadata-file", help="offline protobuf fixture; skips network")
    parser.add_argument("--diff-out", default=os.environ.get("ANDROID_DIFF_OUT", ""))
    return parser.parse_args(argv)


def main(argv=None):
    args = parse_args(argv)
    with open(args.state, encoding="utf-8") as source:
        state = json.load(source)
    if state.get("package_id") != PACKAGE_ID:
        print("ERROR: android state package_id does not match Toss", file=sys.stderr)
        return 1

    if args.metadata_file:
        with open(args.metadata_file, "rb") as source:
            payload = source.read()
    else:
        payload = fetch_metadata()
    versions = extract_versions(payload)
    if not versions:
        print("ERROR: no Android version found in metadata", file=sys.stderr)
        return 1

    today = os.environ.get("ANDROID_DATE") or datetime.date.today().isoformat()
    diff = update_state(state, versions[-1], today)
    if diff["state_changed"]:
        with open(args.state, "w", encoding="utf-8") as target:
            json.dump(state, target, ensure_ascii=False, indent=2)
            target.write("\n")
    if args.diff_out:
        with open(args.diff_out, "w", encoding="utf-8") as target:
            json.dump(diff, target, ensure_ascii=False)

    print(
        "Android candidate: " + diff["current_candidate"]
        + " · audited " + diff["audited_version"]
        + " · audit " + diff["audit_status"]
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
