#!/usr/bin/env python3
"""Sanitize HAR files before they are committed to the repository."""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path
from typing import Any
from urllib.parse import parse_qsl, urlencode, urlsplit, urlunsplit

SENSITIVE_HEADERS = {
    "authorization",
    "cookie",
    "set-cookie",
    "x-csrf-token",
    "x-xsrf-token",
    "x-device-id",
    "x-session-id",
    "x-request-id",
}

# Lowercase throughout: matching lowercases the key first, so Token=, TOKEN=
# etc. are all caught (the old set missed case variants).
SENSITIVE_QUERY_KEYS = {
    "authorization",
    "token",
    "access_token",
    "refresh_token",
    "tabid",
    "deviceid",
    "device_id",
    "requestid",
    "request_id",
    "sessionid",
    "session_id",
}

SENSITIVE_JSON_KEYS = {
    "authorization",
    "token",
    "accessToken",
    "refreshToken",
    "cookie",
    "cookies",
    "deviceId",
    "sessionId",
    "name",
    "phoneNumber",
    "accountNumber",
    "residentRegistrationNumber",
}

TOKEN_PATTERNS = [
    # Base64 and base64url alphabets plus padding: the old [A-Za-z0-9._-]
    # stopped at +/= and leaked the token tail.
    (re.compile(r"Bearer\s+[A-Za-z0-9._~+\-/=]+"), "Bearer <REDACTED_TOKEN>"),
    (re.compile(r"(tabId=)[A-Za-z0-9._~+\-/=]+"), r"\1<REDACTED_TAB_ID>"),
]


def redact_string(value: str) -> str:
    redacted = value
    for pattern, replacement in TOKEN_PATTERNS:
        redacted = pattern.sub(replacement, redacted)
    return redacted


def sanitize_url(url: str) -> str:
    split = urlsplit(url)
    query_items = []
    for key, value in parse_qsl(split.query, keep_blank_values=True):
        if key.lower() in SENSITIVE_QUERY_KEYS:
            query_items.append((key, "<REDACTED>"))
        else:
            query_items.append((key, redact_string(value)))
    return urlunsplit(
        (split.scheme, split.netloc, split.path, urlencode(query_items, doseq=True), split.fragment)
    )


def sanitize_query_params(items: Any) -> None:
    """Sanitize a HAR queryString/params array in place (mirrors the URL query)."""
    if not isinstance(items, list):
        return
    for param in items:
        if not isinstance(param, dict):
            continue
        name = param.get("name", "")
        if isinstance(name, str) and name.lower() in SENSITIVE_QUERY_KEYS:
            param["value"] = "<REDACTED>"
        elif isinstance(param.get("value"), str):
            param["value"] = redact_string(param["value"])


def sanitize_headers(items: list[dict[str, Any]]) -> None:
    for header in items:
        name = str(header.get("name", "")).lower()
        if name in SENSITIVE_HEADERS:
            header["value"] = "<REDACTED>"
        elif isinstance(header.get("value"), str):
            header["value"] = redact_string(header["value"])


def sanitize_cookies(items: list[dict[str, Any]]) -> None:
    for cookie in items:
        cookie["value"] = "<REDACTED>"


def sanitize_post_data(post_data: dict[str, Any]) -> None:
    text = post_data.get("text")
    mime_type = str(post_data.get("mimeType", ""))
    if not isinstance(text, str):
        return

    if "application/json" in mime_type:
        try:
            payload = json.loads(text)
        except json.JSONDecodeError:
            post_data["text"] = redact_string(text)
            return
        post_data["text"] = json.dumps(sanitize_object(payload), ensure_ascii=False)
        return

    post_data["text"] = redact_string(text)


def sanitize_object(value: Any) -> Any:
    if isinstance(value, dict):
        sanitized = {}
        for key, item in value.items():
            if key in SENSITIVE_JSON_KEYS:
                sanitized[key] = "<REDACTED>"
            else:
                sanitized[key] = sanitize_object(item)
        return sanitized

    if isinstance(value, list):
        return [sanitize_object(item) for item in value]

    if isinstance(value, str):
        return redact_string(value)

    return value


def sanitize_entry(entry: dict[str, Any]) -> None:
    request = entry.get("request", {})
    response = entry.get("response", {})

    if isinstance(request.get("url"), str):
        request["url"] = sanitize_url(request["url"])

    # queryString duplicates the URL query as an array; sanitize it the same way.
    sanitize_query_params(request.get("queryString"))

    sanitize_headers(request.get("headers", []))
    sanitize_headers(response.get("headers", []))
    sanitize_cookies(request.get("cookies", []))
    sanitize_cookies(response.get("cookies", []))

    if isinstance(request.get("postData"), dict):
        sanitize_post_data(request["postData"])
        # Form posts duplicate fields in a params array.
        sanitize_query_params(request["postData"].get("params"))

    content = response.get("content", {})
    text = content.get("text") if isinstance(content, dict) else None
    if isinstance(text, str):
        mime = str(content.get("mimeType", ""))
        if "json" in mime:
            # Account/balance data lives in response bodies: apply the same
            # key masking as request payloads instead of regex-only redaction.
            try:
                content["text"] = json.dumps(sanitize_object(json.loads(text)), ensure_ascii=False)
            except json.JSONDecodeError:
                content["text"] = redact_string(text)
        else:
            content["text"] = redact_string(text)


def sanitize_har(document: dict[str, Any]) -> dict[str, Any]:
    log = document.get("log", {})
    for entry in log.get("entries", []):
        sanitize_entry(entry)
    return document


def main(argv: list[str]) -> int:
    if len(argv) != 3:
        print("usage: python3 tools/sanitize_har.py <input.har> <output.har>", file=sys.stderr)
        return 2

    input_path = Path(argv[1])
    output_path = Path(argv[2])

    with input_path.open("r", encoding="utf-8") as fp:
        document = json.load(fp)

    sanitized = sanitize_har(document)

    output_path.parent.mkdir(parents=True, exist_ok=True)
    with output_path.open("w", encoding="utf-8") as fp:
        json.dump(sanitized, fp, ensure_ascii=False, indent=2)
        fp.write("\n")

    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
