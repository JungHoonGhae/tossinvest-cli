"""Network-free regression tests for tools/sanitize_har.py."""

import base64
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


class TestTossctlTraffic(unittest.TestCase):
    """Fields tossctl actually sends/receives (dummy values only)."""

    def test_account_and_user_json_keys_masked_case_insensitively(self):
        body = json.dumps({
            "result": {
                "accountList": [{"accountNo": "000-00-000000", "AccountName": "DUMMY", "key": "k1"}],
                "primaryKey": "dummy-primary",
                "userName": "DUMMY USER",
                "USERID": "dummy-user-id",
                "accessToken": "dummy-access",
                "totalAmount": 1234,
            }
        })
        doc = entry_with(response={"content": {"mimeType": "application/json", "text": body}})
        S.sanitize_har(doc)
        text = doc["log"]["entries"][0]["response"]["content"]["text"]
        for leaked in ("000-00-000000", "DUMMY", "dummy-primary", "dummy-user-id", "dummy-access"):
            self.assertNotIn(leaked, text)
        # Amounts are intentionally kept.
        self.assertEqual(json.loads(text)["result"]["totalAmount"], 1234)

    def test_tossctl_headers_redacted_case_insensitively(self):
        doc = entry_with(request={"headers": [
            {"name": "Browser-Tab-Id", "value": "dummy-tab"},
            {"name": "accountKey", "value": "dummy-key"},
            {"name": "ACCOUNTKEY", "value": "dummy-key2"},
            {"name": "Accept", "value": "application/json"},
        ]})
        S.sanitize_har(doc)
        headers = {h["name"]: h["value"] for h in doc["log"]["entries"][0]["request"]["headers"]}
        self.assertEqual(headers["Browser-Tab-Id"], "<REDACTED>")
        self.assertEqual(headers["accountKey"], "<REDACTED>")
        self.assertEqual(headers["ACCOUNTKEY"], "<REDACTED>")
        self.assertEqual(headers["Accept"], "application/json")

    def test_account_no_query_key_redacted(self):
        doc = entry_with(request={
            "url": "https://x.test/api?AccountNo=000-00-000001&ok=1",
            "queryString": [{"name": "accountNo", "value": "000-00-000001"}],
        })
        S.sanitize_har(doc)
        request = doc["log"]["entries"][0]["request"]
        self.assertNotIn("000-00-000001", request["url"])
        self.assertIn("ok=1", request["url"])
        self.assertEqual(request["queryString"][0]["value"], "<REDACTED>")

    def test_base64_response_decoded_sanitized_reencoded(self):
        raw = json.dumps({"accountNo": "000-00-000002", "balance": 5})
        content = {"mimeType": "application/json", "encoding": "base64",
                   "text": base64.b64encode(raw.encode()).decode()}
        doc = entry_with(response={"content": content})
        S.sanitize_har(doc)
        decoded = json.loads(base64.b64decode(content["text"]))
        self.assertEqual(decoded["accountNo"], "<REDACTED>")
        self.assertEqual(decoded["balance"], 5)

    def test_undecodable_base64_body_dropped(self):
        content = {"mimeType": "image/png", "encoding": "base64",
                   "text": base64.b64encode(b"\xff\xfe\x00binary").decode()}
        doc = entry_with(response={"content": content})
        S.sanitize_har(doc)
        self.assertNotIn("text", content)

    def test_post_body_json_by_mime_variant_or_content(self):
        for mime in ("application/vnd.toss+json;charset=UTF-8", "text/plain", ""):
            post = {"mimeType": mime, "text": json.dumps({"accountNo": "000-00-000003", "qty": 1})}
            doc = entry_with(request={"postData": post})
            S.sanitize_har(doc)
            parsed = json.loads(post["text"])
            self.assertEqual(parsed["accountNo"], "<REDACTED>", mime)
            self.assertEqual(parsed["qty"], 1)

    def test_redirect_url_sanitized(self):
        doc = entry_with(response={"redirectURL": "https://x.test/cb?token=dummy-tok&accountNo=000-00-000004"})
        S.sanitize_har(doc)
        url = doc["log"]["entries"][0]["response"]["redirectURL"]
        self.assertNotIn("dummy-tok", url)
        self.assertNotIn("000-00-000004", url)


if __name__ == "__main__":
    unittest.main()
