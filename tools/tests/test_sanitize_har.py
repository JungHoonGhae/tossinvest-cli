"""Network-free regression tests for tools/sanitize_har.py."""

import json
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

import sanitize_har as S  # noqa: E402


def entry_with(request=None, response=None):
    entry = {"request": request or {}, "response": response or {}}
    return {"log": {"entries": [entry]}}


class TestSanitizeUrl(unittest.TestCase):
    def test_sensitive_query_keys_redacted_case_insensitively(self):
        url = "https://x.test/api?token=abc&Token=def&TOKEN=ghi&ok=1"
        out = S.sanitize_url(url)
        self.assertNotIn("abc", out)
        self.assertNotIn("def", out)
        self.assertNotIn("ghi", out)
        self.assertIn("ok=1", out)

    def test_bearer_with_base64_tail_fully_redacted(self):
        text = S.redact_string("Bearer abcDEF123+/==")
        self.assertEqual(text, "Bearer <REDACTED_TOKEN>")


class TestQueryStringArray(unittest.TestCase):
    def test_query_string_array_mirrors_url_query(self):
        doc = entry_with(
            request={
                "url": "https://x.test/api?token=abc",
                "queryString": [
                    {"name": "token", "value": "abc"},
                    {"name": "SESSIONID", "value": "sess-1"},
                    {"name": "plain", "value": "keep"},
                ],
            }
        )
        S.sanitize_har(doc)
        qs = doc["log"]["entries"][0]["request"]["queryString"]
        by_name = {p["name"]: p["value"] for p in qs}
        self.assertEqual(by_name["token"], "<REDACTED>")
        self.assertEqual(by_name["SESSIONID"], "<REDACTED>")
        self.assertEqual(by_name["plain"], "keep")
        self.assertNotIn("abc", doc["log"]["entries"][0]["request"]["url"])

    def test_post_data_params_array_sanitized(self):
        doc = entry_with(
            request={
                "postData": {
                    "mimeType": "application/x-www-form-urlencoded",
                    "params": [{"name": "session_id", "value": "s3cr3t"}],
                }
            }
        )
        S.sanitize_har(doc)
        params = doc["log"]["entries"][0]["request"]["postData"]["params"]
        self.assertEqual(params[0]["value"], "<REDACTED>")


class TestResponseBody(unittest.TestCase):
    def test_response_json_keys_masked(self):
        body = json.dumps({"accountNumber": "123-45", "balance": 100, "nested": {"name": "홍길동"}})
        doc = entry_with(response={"content": {"mimeType": "application/json", "text": body}})
        S.sanitize_har(doc)
        text = doc["log"]["entries"][0]["response"]["content"]["text"]
        parsed = json.loads(text)
        self.assertEqual(parsed["accountNumber"], "<REDACTED>")
        self.assertEqual(parsed["nested"]["name"], "<REDACTED>")
        self.assertEqual(parsed["balance"], 100)

    def test_response_non_json_falls_back_to_regex(self):
        doc = entry_with(response={"content": {"mimeType": "text/html", "text": "Bearer abcDEF123"}})
        S.sanitize_har(doc)
        text = doc["log"]["entries"][0]["response"]["content"]["text"]
        self.assertNotIn("abcDEF123", text)


if __name__ == "__main__":
    unittest.main()
