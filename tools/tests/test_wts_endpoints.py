"""tools/wts_endpoints.py 회귀 테스트 — stdlib only, 네트워크 없음.

여기 있는 테스트는 전부 **실제로 났던 사고**를 겨냥한다. 이 도구가 만드는
카탈로그는 "무엇을 구현했고 무엇이 다음 후보인가" 의 단일 진실 원천이라,
도구가 틀리면 잘못된 판단이 연쇄된다.

    python3 -m unittest discover -s tools/tests
"""

import json
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

import wts_endpoints as W  # noqa: E402


def triple(host, method, path):
    return f'(0,n.m)({{host:"{host}",method:"{method}",path:"{path}"}})'


def prepare_inventory_fixture(repo):
    for relative in W.GO_WTS_SOURCE_ROOTS:
        target = os.path.join(repo, relative)
        if relative.endswith(".go"):
            os.makedirs(os.path.dirname(target), exist_ok=True)
            with open(target, "w", encoding="utf-8") as out:
                out.write("package ops\n")
        else:
            os.makedirs(target, exist_ok=True)


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
    def test_capture_sweep_preserves_existing_source_evidence(self):
        repo = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
        with open(
            os.path.join(repo, "docs", "reverse-engineering", "wts-endpoints.json"),
            encoding="utf-8",
        ) as source:
            catalog = json.load(source)
        self.assertIn(
            "bundle-literal:",
            catalog["endpoints"]["/api/v1/bond-infos"]["observed"]["source"],
            "the generated catalog must retain hand-audited bundle-call evidence",
        )

    def test_override_beats_everything(self):
        ov = {"/api/v1/account/list": {"status": "candidate", "note": "손으로 뒤집음"}}
        status, note = W.classify("/api/v1/account/list", ov)
        self.assertEqual(status, "candidate")
        self.assertEqual(note, "손으로 뒤집음")

    def test_verified_override_metadata_survives_regeneration(self):
        entry = {"status": "implemented"}
        override = {
            "status": "implemented",
            "method": "DELETE,POST",
            "host": "wts-api",
            "evidence": "verified",
        }

        W.apply_override_metadata(entry, override)

        self.assertEqual(entry["method"], "DELETE,POST")
        self.assertEqual(entry["host"], "wts-api")
        self.assertEqual(entry["evidence"], "verified")

    def test_bundle_metadata_wins_over_curated_override_metadata(self):
        entry = {"status": "implemented", "method": "GET", "host": "wts-info-api"}
        override = {
            "status": "implemented",
            "method": "DELETE,POST",
            "host": "wts-api",
            "evidence": "verified",
        }

        W.apply_override_metadata(entry, override)

        self.assertEqual(entry["method"], "GET")
        self.assertEqual(entry["host"], "wts-info-api")
        self.assertEqual(entry["evidence"], "verified")

    def test_live_concrete_parent_override_does_not_leak_into_templated_child(self):
        parent = "/api/v3/stock-prices"
        child = parent + "/{code}/quotes"
        overrides = {parent: {"status": "candidate", "note": "batch endpoint only"}}
        status, _ = W.classify(child, overrides, {parent, child})
        self.assertEqual(status, "implemented")

    def test_implemented_beats_excluded(self):
        # account/list 는 IMPLEMENTED 이고 account 계열 일부는 EXCLUDED 다.
        status, _ = W.classify("/api/v1/account/list", {})
        self.assertEqual(status, "implemented")

    def test_preference_endpoints_are_implemented(self):
        for path in [
            "/api/v1/user-price-alimy/{stockCode}",
            "/api/v1/user-price-alimy/{stockCode}/{currency}/{targetPrice}",
            "/api/v1/my-assets/hidden-stocks/hide",
            "/api/v1/my-assets/hidden-stocks/show",
            "/api/v2/hidden-stocks",
        ]:
            with self.subTest(path=path):
                status, _ = W.classify(path, {})
                self.assertEqual(status, "implemented")

    def test_account_services_batch_endpoints_are_implemented(self):
        for path in [
            "/api/v1/autotrade/open-banking/creatable",
            "/api/v1/autotrade/open-banking/need-registration",
            "/api/v1/trading/settings/simple-trade",
            "/api/v2/trading/settings/investor-exchange-choice-type",
            "/api/v1/users/settings/me/ats-notification",
            "/api/v1/member-subscriptions/get-option-real-time-tick",
            "/api/v1/securities-transfer/my-accounts",
            "/api/v1/securities-transfer/recent-accounts",
        ]:
            with self.subTest(path=path):
                status, _ = W.classify(path, {})
                self.assertEqual(status, "implemented")

    def test_asset_snapshot_contracts_are_implemented_without_claiming_truncated_paths(self):
        for path in [
            "/api/v1/asset-snapshot/all-accounts/chart/{range}/{stepUnit}",
            "/api/v1/asset-snapshot/all-accounts/chart/ONE_MONTH/DAY",
            "/api/v1/asset-snapshot/all-accounts/detail-by-date",
            "/api/v1/asset-snapshot/all-accounts/page",
            "/api/v1/asset-snapshot/chart/{range}/{stepUnit}",
            "/api/v1/asset-snapshot/chart/ONE_MONTH/DAY",
            "/api/v1/asset-snapshot/detail-by-date",
            "/api/v1/asset-snapshot/page",
        ]:
            with self.subTest(path=path):
                status, _ = W.classify(path, {})
                self.assertEqual(status, "implemented")

        for truncated in [
            "/api/v1/asset-snapshot/all-accounts/chart",
            "/api/v1/asset-snapshot/chart",
        ]:
            with self.subTest(path=truncated):
                status, _ = W.classify(truncated, {})
                self.assertNotEqual(status, "implemented")

    def test_asset_snapshot_probe_normalizes_to_bundle_template(self):
        for path in [
            "/api/v1/asset-snapshot/all-accounts/chart/ONE_MONTH/DAY",
            "/api/v1/asset-snapshot/chart/ONE_MONTH/DAY",
        ]:
            with self.subTest(path=path):
                self.assertEqual(
                    W._probe_inventory_path(path),
                    path.removesuffix("/ONE_MONTH/DAY") + "/{range}/{stepUnit}",
                )

    def test_ai_signal_detail_and_sector_simple_are_implemented(self):
        for path in [
            "/api/v1/dashboard/wts/overview/ai-signals/detail",
            "/api/v2/dashboard/wts/overview/tics/{id}/simple",
        ]:
            with self.subTest(path=path):
                status, _ = W.classify(path, {})
                self.assertEqual(status, "implemented")

    def test_recommendation_preserves_verified_note(self):
        priority, note = W.recommendation(
            "/api/v1/earning-call/user-based",
            "candidate",
            "verified blocker",
            {},
        )
        self.assertEqual(priority, "next")
        self.assertEqual(note, "verified blocker")

    def test_recommendation_can_be_explicitly_deferred(self):
        priority, note = W.recommendation(
            "/api/v1/r-chart",
            "candidate",
            "no current consumer",
            {"priority": "deferred"},
        )
        self.assertEqual(priority, "deferred")
        self.assertEqual(note, "no current consumer")

    def test_dynamic_price_alert_delete_contract_is_curated(self):
        path = "/api/v1/user-price-alimy/{stockCode}/{currency}/{targetPrice}"
        self.assertEqual(W.CURATED_CONTRACTS[path]["method"], "DELETE")
        self.assertEqual(W.CURATED_CONTRACTS[path]["host"], "wts-api")

    def test_price_alert_probe_normalizes_to_catalog_template(self):
        self.assertEqual(
            W._probe_inventory_path("/api/v1/user-price-alimy/A005930"),
            "/api/v1/user-price-alimy/{stockCode}",
        )

    def test_sector_probe_normalizes_small_numeric_id_to_catalog_template(self):
        self.assertEqual(
            W._probe_inventory_path(
                "/api/v2/dashboard/wts/overview/tics/1/stocks"
            ),
            "/api/v2/dashboard/wts/overview/tics/{id}/stocks",
        )

    def test_earning_detail_probe_normalizes_to_bundle_template(self):
        self.assertEqual(
            W._probe_inventory_path(
                "/api/v1/earning-call/events/228692/info"
            ),
            "/api/v1/earning-call/events/{eventId}/info",
        )

    def test_diff_reports_build_and_chunk_changes(self):
        previous = {"build_id": "old", "chunk_count": 10}
        current = {
            "build_id": "new",
            "chunk_count": 11,
            "endpoints": {"/api/v1/new": {"status": "candidate"}},
        }
        diff = W.build_diff(previous, current, ["/api/v1/new"], [])
        self.assertTrue(diff["build_changed"])
        self.assertTrue(diff["chunk_count_changed"])
        self.assertEqual(diff["previous_build_id"], "old")
        self.assertEqual(diff["current_build_id"], "new")
        self.assertEqual(diff["new_candidates"], ["/api/v1/new"])

    def test_mass_inventory_shrink_is_rejected(self):
        self.assertTrue(W.suspicious_inventory_shrink(1112, 326))
        self.assertFalse(W.suspicious_inventory_shrink(1112, 1000))
        self.assertFalse(W.suspicious_inventory_shrink(0, 0))

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

    def test_sensitive_account_admin_and_auth_paths_are_excluded(self):
        for path in [
            "/api/v1/accounts/ssn-verification/check",
            "/api/v1/accounts/ssn-verification/mark-ssn-verified",
            "/api/v2/accounts/close/password",
            "/api/v3/accounts/close",
            "/api/v3/accounts/close/pre-check",
            "/api/v1/session/refresh",
        ]:
            with self.subTest(path=path):
                status, _ = W.classify(path, {})
                self.assertEqual(status, "excluded")

    def test_legal_and_feedback_plumbing_is_excluded(self):
        for path in [
            "/api/v1/portal/agreement-modules/{moduleCode}",
            "/api/v1/nova-feedback/user-feedbacks",
        ]:
            with self.subTest(path=path):
                status, _ = W.classify(path, {})
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

    def test_catalog_summary_counts_match_endpoint_entries(self):
        repo = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
        with open(os.path.join(repo, W.CATALOG), encoding="utf-8") as source:
            catalog = json.load(source)
        endpoints = catalog["endpoints"]
        status_counts = {
            status: sum(entry.get("status") == status for entry in endpoints.values())
            for status in ("implemented", "candidate", "excluded")
        }
        self.assertEqual(catalog["total"], len(endpoints))
        for status, count in status_counts.items():
            self.assertEqual(catalog["counts"][status], count, status)
        self.assertEqual(
            catalog["counts"]["candidate_next"],
            sum(
                entry.get("status") == "candidate" and entry.get("priority") == "next"
                for entry in endpoints.values()
            ),
        )
        self.assertEqual(
            catalog["counts"]["meaningful"],
            status_counts["implemented"] + status_counts["candidate"],
        )

    def test_every_production_go_endpoint_is_owned_by_the_inventory(self):
        repo = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
        exposures = W.discover_go_exposures(repo)
        unowned = {
            path: sources
            for path, sources in exposures.items()
            if W.classify(path, {})[0] != "implemented"
        }
        self.assertFalse(
            unowned,
            "production Go paths must be classified as implemented; otherwise the "
            f"weekly refresh can silently reverse ownership: {unowned}",
        )

    def test_production_interest_endpoint_composed_from_constant_is_discovered(self):
        repo = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
        exposures = W.discover_go_exposures(repo)
        self.assertIn(
            "/api/v1/interest/accounts/annual/history/by-payment-date",
            exposures,
            "the scanner must resolve string constants passed through fmt.Sprintf",
        )
        self.assertIn(
            "/api/v1/interest/accounts/annual/history/years",
            exposures,
        )
        self.assertNotIn(
            "/api/v1/interest/accounts/annual/history",
            exposures,
            "a composition-only prefix must not be reported as a callable endpoint",
        )

    def test_production_dividend_endpoint_variants_are_discovered(self):
        repo = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
        exposures = W.discover_go_exposures(repo)
        self.assertIn("/api/v1/dividends/accounts/annual/history", exposures)
        self.assertIn(
            "/api/v1/dividends/accounts/annual/history/by-payment-date",
            exposures,
            "conditional endpoint variants must remain explicit inventory facts",
        )

    def test_production_watchlist_item_variants_are_finite(self):
        repo = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
        exposures = W.discover_go_exposures(repo)
        self.assertIn("/api/v1/new-watchlists/items", exposures)
        self.assertIn("/api/v1/new-watchlists/items/remove", exposures)
        self.assertNotIn(
            "/api/v1/new-watchlists{param}",
            exposures,
            "a finite helper endpoint set must not collapse into a fictitious wildcard path",
        )

    def test_probe_host_and_method_match_known_inventory_facts(self):
        repo = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
        probes = W.discover_go_probes(repo)
        with open(os.path.join(repo, W.CATALOG), encoding="utf-8") as source:
            endpoints = json.load(source)["endpoints"]

        mismatches = W.probe_inventory_mismatches(probes, endpoints)
        self.assertFalse(mismatches, f"probe/catalog drift: {mismatches}")

    def test_probe_inventory_mismatches_fail_closed(self):
        probes = [
            {"name": "missing", "path": "/api/v1/missing", "host": "wts-api", "method": "GET"},
            {"name": "host", "path": "/api/v1/host", "host": "wts-info-api", "method": "GET"},
            {"name": "method", "path": "/api/v1/method", "host": "wts-api", "method": "POST"},
            {"name": "facts", "path": "/api/v1/facts", "host": "wts-api", "method": "GET"},
        ]
        endpoints = {
            "/api/v1/host": {"host": "wts-api", "method": "GET"},
            "/api/v1/method": {"host": "wts-api", "method": "GET"},
            "/api/v1/facts": {"status": "implemented"},
        }
        got = W.probe_inventory_mismatches(probes, endpoints)
        self.assertEqual(
            [item[1] for item in got],
            ["inventory", "host", "method", "host", "method"],
        )

    def test_go_wts_source_manifest_covers_current_endpoint_owners(self):
        self.assertEqual(
            set(W.GO_WTS_SOURCE_ROOTS),
            {
                "internal/client",
                "internal/push",
                "internal/monitor",
                "internal/ops/wts_operations.go",
            },
        )

    def test_discover_go_exposures_includes_raw_string_literals(self):
        with tempfile.TemporaryDirectory() as repo:
            prepare_inventory_fixture(repo)
            target = os.path.join(repo, "internal", "client")
            with open(os.path.join(target, "example.go"), "w", encoding="utf-8") as out:
                out.write('package client\nconst endpoint = `/api/v1/raw-path`\n')
            got = W.discover_go_exposures(repo)
            self.assertIn("/api/v1/raw-path", got)

    def test_discover_go_exposures_uses_literals_embedded_in_templates_and_all_wts_packages(self):
        with tempfile.TemporaryDirectory() as repo:
            prepare_inventory_fixture(repo)
            client = os.path.join(repo, "internal", "client")
            push = os.path.join(repo, "internal", "push")
            with open(os.path.join(client, "trading.go"), "w", encoding="utf-8") as out:
                out.write(
                    'package client\nimport "fmt"\n'
                    'func f(base, id, query string) string { return fmt.Sprintf('
                    '"%s/api/v3/trading/order/%s/available-actions?%s", base, id, query) }\n'
                    'func g(base, code string) string { return base + '
                    '"/api/v1/growth/autotrade/plan/stock/" + code }\n'
                )
            with open(os.path.join(push, "listen.go"), "w", encoding="utf-8") as out:
                out.write(
                    'package push\nconst endpoint = '
                    '"https://sse-message.tossinvest.com/api/v1/wts-notification"\n'
                )

            got = W.discover_go_exposures(repo)
            self.assertIn("/api/v3/trading/order/{param}/available-actions", got)
            self.assertIn("/api/v1/growth/autotrade/plan/stock/{param}", got)
            self.assertIn("/api/v1/wts-notification", got)
            self.assertEqual(
                W.classify("/api/v1/growth/autotrade/plan/stock/{param}", {})[0],
                "implemented",
            )

    def test_discover_go_exposures_resolves_string_constants_in_sprintf(self):
        with tempfile.TemporaryDirectory() as repo:
            prepare_inventory_fixture(repo)
            client = os.path.join(repo, "internal", "client")
            with open(os.path.join(client, "interest.go"), "w", encoding="utf-8") as out:
                out.write(
                    'package client\nimport "fmt"\n'
                    'const basePath = "/api/v1/interest/accounts/annual/history"\n'
                    'func f(base string, year int) string { return fmt.Sprintf('
                    '"%s%s/by-payment-date?year=%d", base, basePath, year) }\n'
                )

            got = W.discover_go_exposures(repo)
            self.assertIn(
                "/api/v1/interest/accounts/annual/history/by-payment-date",
                got,
            )

    def test_discover_go_exposures_fails_when_a_configured_root_is_missing(self):
        with tempfile.TemporaryDirectory() as repo:
            prepare_inventory_fixture(repo)
            os.rmdir(os.path.join(repo, "internal", "monitor"))
            with self.assertRaisesRegex(RuntimeError, "internal/monitor"):
                W.discover_go_exposures(repo)

    def test_discover_go_probes_includes_shared_and_hand_listed_probes(self):
        repo = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
        probes = W.discover_go_probes(repo)
        names = {probe["name"] for probe in probes}
        self.assertEqual(len(probes), len(names), "runtime probe names must be unique")
        self.assertTrue(
            {
                "account-list",
                "quote-stock-infos",
                "quote-trades",
                "quote-orderbook",
                "quote-price-limits",
                "market-trading-hours",
            }.issubset(names),
            f"hand-listed runtime probes are missing from discovery: {names}",
        )

    def test_probe_inventory_path_templates_fixed_symbols(self):
        self.assertEqual(
            W._probe_inventory_path("/api/v2/stock-prices/A005930/ticks"),
            "/api/v2/stock-prices/{code}/ticks",
        )
        self.assertEqual(
            W._probe_inventory_path("/api/v2/stock-infos/A005930"),
            "/api/v2/stock-infos/{code}",
        )
        self.assertEqual(
            W._probe_inventory_path("/api/v4/calendar/monthly/2026-09"),
            "/api/v4/calendar/monthly/{month}",
        )

    def test_known_host_aliases_are_exact_and_path_scoped(self):
        path = "/api/v1/earning-call/home"
        self.assertTrue(W.hosts_compatible(path, "wts-cert-api", "wts-info-api"))
        self.assertFalse(W.hosts_compatible(path, "wts-api", "wts-info-api"))
        self.assertFalse(W.hosts_compatible("/api/v1/other", "wts-cert-api", "wts-info-api"))

    def test_inventory_lookup_does_not_inherit_concrete_parent(self):
        endpoints = {
            "/api/v1/foo": {"status": "implemented", "host": "wts-api", "method": "GET"},
            "/api/v1/items/{id}": {"status": "implemented", "host": "wts-api", "method": "GET"},
        }
        self.assertIsNone(W.find_inventory_entry(endpoints, "/api/v1/foo/typo"))
        self.assertEqual(
            W.find_inventory_entry(endpoints, "/api/v1/items/42"),
            endpoints["/api/v1/items/{id}"],
        )


class TestLegacyKey(unittest.TestCase):
    def test_strips_from_first_placeholder(self):
        self.assertEqual(
            W._legacy_key("/api/v1/profit/{profitType}/{key}"), "/api/v1/profit"
        )

    def test_path_without_placeholder_is_unchanged(self):
        self.assertEqual(W._legacy_key("/api/v1/account/list"), "/api/v1/account/list")


if __name__ == "__main__":
    unittest.main()
