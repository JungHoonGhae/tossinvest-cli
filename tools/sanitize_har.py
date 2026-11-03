#!/usr/bin/env python3
"""Sanitize HAR captures of tossctl/WTS traffic.

Masks credentials (cookies, tokens, session/tab headers) and identifying JSON
fields (account numbers/keys, names, user ids). Numeric amounts such as
balances and prices are deliberately kept so captures stay useful for
reverse engineering — sanitized HARs are local working material, not fixtures.
"""

from __future__ import annotations

import base64
import binascii
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
    # Sent by internal/client (account.go, account_scope.go).
    "browser-tab-id",
    "accountkey",
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
    "accountno",
    "accountkey",
}

# Lowercase: keys are matched case-insensitively.
SENSITIVE_JSON_KEYS = {
    "authorization",
    "token",
    "accesstoken",
    "access_token",
    "refreshtoken",
    "refresh_token",
    "cookie",
    "cookies",
    "deviceid",
    "sessionid",
    "browsertabid",
    "name",
    "nickname",
    "username",
    "userid",
    "userprofileid",
    "phonenumber",
    "email",
    "accountno",
    "accountnumber",
    "accountname",
    "accountid",
    "accountkey",
    "primarykey",
    # accountList entries carry the account key as a bare "key".
    "key",
    "residentregistrationnumber",
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


# Headers whose value is a URL: their query strings leak the same keys as the
# request URL does.
URL_HEADERS = {"location", "referer", "origin", "content-location"}


def sanitize_headers(items: list[dict[str, Any]]) -> None:
    for header in items:
        name = str(header.get("name", "")).lower()
        if name in SENSITIVE_HEADERS:
            header["value"] = "<REDACTED>"
        elif name in URL_HEADERS and isinstance(header.get("value"), str):
            header["value"] = sanitize_url(header["value"])
        elif isinstance(header.get("value"), str):
            header["value"] = redact_string(header["value"])


def sanitize_cookies(items: list[dict[str, Any]]) -> None:
    for cookie in items:
        cookie["value"] = "<REDACTED>"


FORM_BODY = re.compile(r"[^\s=&<>{}]+=[^\s&]*(?:&[^\s=&]+=[^\s&]*)*")


def sanitize_form(text: str) -> str:
    items = []
    for key, value in parse_qsl(text, keep_blank_values=True):
        lowered = key.lower()
        if lowered in SENSITIVE_QUERY_KEYS or lowered in SENSITIVE_JSON_KEYS:
            items.append((key, "<REDACTED>"))
        else:
            items.append((key, redact_string(value)))
    return urlencode(items)


def sanitize_body(text: str, mime_type: str) -> str:
    """Key-mask a body that is JSON or form-encoded (by mime type or by shape);
    regex-only redaction otherwise."""
    try:
        payload = json.loads(text)
    except ValueError:
        if "x-www-form-urlencoded" in mime_type.lower() or FORM_BODY.fullmatch(text.strip()):
            return sanitize_form(text)
        return redact_string(text)
    if "json" not in mime_type.lower() and not isinstance(payload, (dict, list)):
        # A bare scalar ("123", "true") in a non-JSON body is not JSON data.
        return redact_string(text)
    return json.dumps(sanitize_object(payload), ensure_ascii=False)


def sanitize_post_data(post_data: dict[str, Any]) -> None:
    text = post_data.get("text")
    if isinstance(text, str):
        post_data["text"] = sanitize_body(text, str(post_data.get("mimeType", "")))


def sanitize_content(content: dict[str, Any]) -> None:
    text = content.get("text")
    if not isinstance(text, str):
        return
    mime = str(content.get("mimeType", ""))
    if content.get("encoding") != "base64":
        content["text"] = sanitize_body(text, mime)
        return
    try:
        decoded = base64.b64decode(text, validate=True).decode("utf-8")
    except (binascii.Error, UnicodeDecodeError):
        # Binary (or broken) payload we cannot inspect: drop it rather than leak it.
        del content["text"]
        return
    content["text"] = base64.b64encode(sanitize_body(decoded, mime).encode("utf-8")).decode("ascii")


def sanitize_object(value: Any) -> Any:
    if isinstance(value, dict):
        sanitized = {}
        for key, item in value.items():
            if isinstance(key, str) and key.lower() in SENSITIVE_JSON_KEYS:
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

    if isinstance(response.get("redirectURL"), str):
        response["redirectURL"] = sanitize_url(response["redirectURL"])

    # Account data lives in response bodies: apply the same key masking as
    # request payloads instead of regex-only redaction.
    if isinstance(response.get("content"), dict):
        sanitize_content(response["content"])


def sanitize_har(document: dict[str, Any]) -> dict[str, Any]:
    log = document.get("log", {})
    for entry in log.get("entries", []):
        sanitize_entry(entry)
    # Page titles are often the page URL, query string included.
    for page in log.get("pages", []):
        if isinstance(page, dict) and isinstance(page.get("title"), str) and "://" in page["title"]:
            page["title"] = sanitize_url(page["title"])
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
