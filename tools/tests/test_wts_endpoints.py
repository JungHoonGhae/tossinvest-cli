"""tools/wts_endpoints.py 회귀 테스트 — stdlib only, 네트워크 없음.

여기 있는 테스트는 전부 **실제로 났던 사고**를 겨냥한다. 이 도구가 만드는
카탈로그는 "무엇을 구현했고 무엇이 다음 후보인가" 의 단일 진실 원천이라,
도구가 틀리면 잘못된 판단이 연쇄된다.

    python3 -m unittest discover -s tools/tests
"""

import os
import sys
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

import wts_endpoints as W  # noqa: E402


def triple(host, method, path):
    return f'(0,n.m)({{host:"{host}",method:"{method}",path:"{path}"}})'


class TestNormalize(unittest.TestCase):
    def test_dynamic_segment_becomes_named_placeholder(self):
        # 예전엔 `[` 에서 잘려 경로가 통째로 짧아졌고, 프로버가 그 잘린 경로를
        # 때려 위양성 404 를 33건 쌓았다.
        self.assertEqual(
            W._normalize("/api/v1/asset-snapshot/chart/[range]/[stepUnit]"),
            "/api/v1/asset-snapshot/chart/{range}/{stepUnit}",
        )

    def test_numeric_id_is_normalized(self):
        self.assertEqual(W._normalize("/api/v1/boards/123456"), "/api/v1/boards/{id}")

    def test_unnamed_segment_falls_back_to_id(self):
        self.assertEqual(W._normalize("/api/v1/x/[]"), "/api/v1/x/{id}")


class TestDerivePaths(unittest.TestCase):
    def test_triple_supplies_host_and_method(self):
        norm, meta = W.derive_paths(triple("cert", "GET", "/api/v1/foo"))
        self.assertIn("/api/v1/foo", norm)
        self.assertEqual(meta["/api/v1/foo"]["host"], "wts-cert-api")
        self.assertEqual(meta["/api/v1/foo"]["method"], "GET")

    def test_launcher_token_maps_to_wts_api(self):
        # 두 개의 독립 관측으로 확정한 매핑이다. 틀리면 프로버가 엉뚱한 호스트를
        # 때려 404 를 정답으로 기록한다.
        _, meta = W.derive_paths(triple("launcher", "GET", "/api/v1/account/list"))
        self.assertEqual(meta["/api/v1/account/list"]["host"], "wts-api")

    def test_truncated_shadow_is_dropped(self):
        blob = triple("cert", "GET", "/api/v1/thing/[id]/detail")
        norm, _ = W.derive_paths(blob)
        self.assertIn("/api/v1/thing/{id}/detail", norm)
        self.assertNotIn("/api/v1/thing", norm)

    def test_real_shadow_survives(self):
        # 2026-08-25: 그림자 제거가 실재하는 엔드포인트를 지웠다. 날짜 없는 쪽은
        # 응답 스키마가 다른 별개 API 이고 주문 경로가 그걸 부른다.
        real = "/api/v1/exchange/usd/base-exchange-rate"
        self.assertIn(real, W.REAL_SHADOWS, "REAL_SHADOWS 에서 빠지면 다시 지워진다")
        norm, _ = W.derive_paths(triple("launcher", "GET", real + "/[date]"))
        self.assertIn(real, norm)
        self.assertIn(real + "/{date}", norm)

    def test_output_does_not_depend_on_input_order(self):
        # 2026-08-24: 청크를 정렬 없이 순회해 같은 번들에서 매번 다른 카탈로그가
        # 나왔다. 한 경로가 두 메서드를 선언하면 먼저 읽힌 쪽이 이겼다.
        a = triple("cert", "PATCH", "/api/v1/dual")
        b = triple("cert", "DELETE", "/api/v1/dual")
        first = W.derive_paths(a + "\n" + b)
        second = W.derive_paths(b + "\n" + a)
        self.assertEqual(first, second, "삽입 순서가 결과를 바꾸면 안 된다")

        # 위 비교만으로는 부족하다. 파이썬 set 순회는 **한 프로세스 안에서는**
        # 해시 시드가 고정이라 삽입 순서와 무관하게 같은 값이 나온다 — 실제
        # 사고는 프로세스 간(= CI 실행 간) 차이였다. 정렬돼 있다는 것이
        # 시드와 무관하게 안정적임을 보장하는 진짜 불변식이다.
        methods = first[1]["/api/v1/dual"]["method"].split(",")
        self.assertEqual(methods, sorted(methods), "메서드는 정렬돼 있어야 시드와 무관하다")
        self.assertEqual(methods, ["DELETE", "PATCH"])


class TestClassify(unittest.TestCase):
    def test_override_beats_everything(self):
        ov = {"/api/v1/account/list": {"status": "candidate", "note": "손으로 뒤집음"}}
        status, note = W.classify("/api/v1/account/list", ov)
        self.assertEqual(status, "candidate")
        self.assertEqual(note, "손으로 뒤집음")

    def test_implemented_beats_excluded(self):
        # account/list 는 IMPLEMENTED 이고 account 계열 일부는 EXCLUDED 다.
        status, _ = W.classify("/api/v1/account/list", {})
        self.assertEqual(status, "implemented")

    def test_kyc_is_excluded(self):
        status, reason = W.classify("/api/v1/kyc/status", {})
        self.assertEqual(status, "excluded")
        self.assertTrue(reason)

    def test_auth_plumbing_is_excluded(self):
        # 인증 plumbing 29건이 통째로 candidate 로 새던 것을 막은 패턴.
        status, _ = W.classify("/api/v1/common/auth/sms/verify", {})
        self.assertEqual(status, "excluded")

    def test_plural_accounts_namespace_is_excluded(self):
        # 단수 account/ 만 걸러내고 있어 복수형 40여건이 새고 있었다.
        status, _ = W.classify("/api/v1/accounts/fatca", {})
        self.assertEqual(status, "excluded")

    def test_unknown_path_is_candidate(self):
        status, _ = W.classify("/api/v1/brand-new-thing", {})
        self.assertEqual(status, "candidate")

    def test_implemented_patterns_are_not_over_broad(self):
        # exchange 계열 패턴이 접두사라서 부르지도 않는 형제 경로까지
        # implemented 로 잡던 것을 정확 경로로 좁혔다.
        for path in [
            "/api/v1/exchange/usd/base-exchange-rate/{date}",
            "/api/v1/exchange/current-quote",
            "/api/v1/exchange/current-quote/for-sell",
        ]:
            with self.subTest(path=path):
                status, _ = W.classify(path, {})
                self.assertNotEqual(status, "implemented", f"{path} 는 호출하지 않는다")


class TestLegacyKey(unittest.TestCase):
    def test_strips_from_first_placeholder(self):
        self.assertEqual(
            W._legacy_key("/api/v1/profit/{profitType}/{key}"), "/api/v1/profit"
        )

    def test_path_without_placeholder_is_unchanged(self):
        self.assertEqual(W._legacy_key("/api/v1/account/list"), "/api/v1/account/list")


if __name__ == "__main__":
    unittest.main()
