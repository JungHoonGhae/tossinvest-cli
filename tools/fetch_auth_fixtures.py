#!/usr/bin/env python3
"""Fetch authenticated Toss Securities fixtures from a saved Playwright storage state."""

from __future__ import annotations

import argparse
import json
import sys
from http.cookiejar import Cookie, CookieJar
from pathlib import Path
from typing import Any
from urllib.request import HTTPCookieProcessor, Request, build_opener

SENSITIVE_KEYS = {
    "accountNo",
    "accountNumber",
    "customerNo",
    "memberNo",
    "userNo",
    "phoneNumber",
    "name",
    "displayName",
}


def build_cookie_jar(storage_state: dict[str, Any]) -> CookieJar:
    jar = CookieJar()
    for item in storage_state.get("cookies", []):
        domain = item["domain"]
        jar.set_cookie(
            Cookie(
                version=0,
                name=item["name"],
                value=item["value"],
                port=None,
                port_specified=False,
                domain=domain,
                domain_specified=True,
                domain_initial_dot=domain.startswith("."),
                path=item["path"],
                path_specified=True,
                secure=item.get("secure", False),
                expires=None,
                discard=False,
                comment=None,
                comment_url=None,
                rest={"HttpOnly": item.get("httpOnly", False)},
                rfc2109=False,
            )
        )
    return jar


def sanitize(value: Any) -> Any:
    if isinstance(value, dict):
        result = {}
        for key, item in value.items():
            if key in SENSITIVE_KEYS:
                result[key] = "<REDACTED>"
            else:
                result[key] = sanitize(item)
        return result
    if isinstance(value, list):
        return [sanitize(item) for item in value]
    return value


def xsrf_token(storage_state: dict[str, Any]) -> str:
    for cookie in storage_state.get("cookies", []):
        if cookie.get("name") == "XSRF-TOKEN":
            return cookie.get("value", "")
    return ""


def fetch_json(opener, url: str, body: bytes | None = None, storage_state: dict[str, Any] | None = None) -> dict[str, Any]:
    headers = {
        "User-Agent": "tossctl-auth-fixtures/0.1",
        "Referer": "https://www.tossinvest.com/account",
    }
    if body is not None:
        headers["Content-Type"] = "application/json"
    if storage_state is not None:
        token = xsrf_token(storage_state)
        if token:
            headers["X-XSRF-TOKEN"] = token

    request = Request(
        url,
        data=body,
        headers=headers,
        method="POST" if body is not None else "GET",
    )
    with opener.open(request, timeout=20) as response:
        return json.loads(response.read().decode("utf-8"))


def specs() -> list[tuple[str, str, bytes | None]]:
    return [
        ("account-list.json", "https://wts-api.tossinvest.com/api/v1/account/list", None),
        (
            "new-watchlists.json",
            "https://wts-cert-api.tossinvest.com/api/v1/new-watchlists?includePrice=true&lazyLoad=false",
            None,
        ),
        (
            "asset-overview.json",
            "https://wts-cert-api.tossinvest.com/api/v3/my-assets/summaries/markets/all/overview",
            None,
        ),
        (
            "pending-orders.json",
            "https://wts-cert-api.tossinvest.com/api/v1/trading/orders/histories/all/pending",
            None,
        ),
        (
            "cached-orderable-amount.json",
            "https://wts-cert-api.tossinvest.com/api/v1/dashboard/common/cached-orderable-amount",
            None,
        ),
        (
            "withdrawable-kr.json",
            "https://wts-api.tossinvest.com/api/v1/my-assets/summaries/markets/kr/withdrawable-amount",
            None,
        ),
        (
            "withdrawable-us.json",
            "https://wts-api.tossinvest.com/api/v1/my-assets/summaries/markets/us/withdrawable-amount",
            None,
        ),
        (
            "asset-sections-v2.json",
            "https://wts-cert-api.tossinvest.com/api/v2/dashboard/asset/sections/all",
            b"{}",
        ),
    ]


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("storage_state", type=Path)
    parser.add_argument("output_dir", type=Path)
    args = parser.parse_args(argv[1:])

    storage_state = json.loads(args.storage_state.read_text())
    opener = build_opener(HTTPCookieProcessor(build_cookie_jar(storage_state)))

    args.output_dir.mkdir(parents=True, exist_ok=True)
    manifest = []

    for filename, url, body in specs():
        payload = sanitize(fetch_json(opener, url, body=body, storage_state=storage_state))
        path = args.output_dir / filename
        path.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        manifest.append({"file": filename, "url": url, "method": "POST" if body is not None else "GET"})

    (args.output_dir / "manifest.json").write_text(
        json.dumps({"fixtures": manifest}, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
