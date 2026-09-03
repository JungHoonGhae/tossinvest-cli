#!/usr/bin/env python3
"""Track the Toss WTS web API surface and classify each endpoint.

Toss's web app has no public spec, so we extract every `/api/vN/...` path from
the production JS bundles and classify it:

  implemented — tossctl already exposes this (mapped to a command)
  excluded    — intentionally out of scope (onboarding/KYC/promo/telemetry/UI)
  candidate   — not yet implemented; a lead for a future tossctl feature

Run with no args to refresh docs/reverse-engineering/wts-endpoints.json and
print a summary + any endpoints added/removed since the committed catalog.
Exit code 0 reports a complete scan; collection failure or a suspicious mass
shrink exits nonzero without overwriting the existing catalog.

stdlib only (runs in CI without deps).
"""
import concurrent.futures
import datetime
import json
import os
import re
import subprocess
import sys
import urllib.request

BASE = "https://www.tossinvest.com"
UA = ("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 "
      "(KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36")
CATALOG = os.path.join("docs", "reverse-engineering", "wts-endpoints.json")
GO_WTS_SOURCE_ROOTS = (
    "internal/client",
    "internal/push",
    "internal/monitor",
    "internal/ops/wts_operations.go",
)
GO_FMT_VERB_RE = re.compile(
    r"%(?:\[[0-9]+\])?[-+#0 ']*[0-9]*(?:\.[0-9]+)?[vTtbcdoOqxXUeEfgGsxp]"
)

# The bundle currently declares these reads on cert, while live WTS captures
# used by the production client verified the same paths on info. Both hosts are
# therefore accepted for these exact paths; every other host mismatch remains a
# CI failure. Keep this list narrow so it cannot turn host checking back into
# the old "try every host" guesswork.
KNOWN_HOST_ALIASES = {
    "/api/v1/earning-call/upcoming": {"wts-cert-api", "wts-info-api"},
    "/api/v1/earning-call/home": {"wts-cert-api", "wts-info-api"},
    "/api/v1/dashboard/wts/overview/ai-signals/personalized": {"wts-cert-api", "wts-info-api"},
    "/api/v1/community/top-rankings/INFLUENCER": {"wts-cert-api", "wts-info-api"},
    "/api/v1/lens/issues": {"wts-cert-api", "wts-info-api"},
    "/api/v4/calendar/monthly": {"wts-cert-api", "wts-info-api"},
}

# ── classification rules ──────────────────────────────────────────────────
# implemented: a path is "implemented" if it matches any of these (these mirror
# the endpoints internal/client/*.go actually calls).
IMPLEMENTED = [
    r"^/api/v1/account/list$",
    r"^/api/v1/dashboard/all-accounts$",                         # account overview (Android + live verified)
    r"^/api/v1/openapi/client$",                                  # openapi status / doctor
    r"^/api/v1/openapi/client/allowed-ips(?:/[^/]+)?$",            # openapi IP allowlist
    r"^/api/v1/autotrade/open-banking/info/find$",                 # banking status (read-only)
    r"^/api/v1/calendar/ai-summary/key-events$",                   # current key events
    r"^/api/v1/user-alimies$",                                     # notification settings
    r"^/api/v1/user-price-alimy/[^/]+$",                           # price alert list/create
    r"^/api/v1/user-price-alimy/[^/]+/[^/]+/[^/]+$",               # price alert delete
    r"^/api/v1/my-assets/hidden-stocks/(hide|show)$",               # hidden holding write
    r"^/api/v2/hidden-stocks$",                                     # hidden holding list
    r"^/api/v2/reasoning/personalized$",                           # enriched personalized briefing
    r"^/api/v1/interest/accounts/annual/history",       # account interest
    r"^/api/v1/ria-calculator/(report|limit|tax-savings/optimized)$",  # tax ria
    r"^/api/v1/usa-market/get-option-biz-day-by-overtime$",            # market option-hours
    # 2026-08-04~07 추가분. **여기에 등록해야 보존된다** — endpoints[].status 를 손으로
    # 고치면 다음 주간 모니터가 자동 분류로 덮어써 implemented 가 candidate 로 되돌아간다
    # (2026-08-10 에 11건이 그렇게 뒤집혔다).
    r"^/api/v1/crypto-prices$",                                        # quote crypto
    r"^/api/v2/reasoning/stocks",                                      # quote reasoning
    r"^/api/v1/dashboard/wts/overview/signals$",                       # quote signals
    r"^/api/v1/search-all/wts-auto-complete$",                         # search
    r"^/api/v1/margin/cert/notice/receivable$",                        # account receivable
    r"^/api/v1/option-(maturity-date|both-chain)/get-all$",            # quote options
    r"^/api/v1/screener/filters/(base|range)$",                        # market filters
    r"^/api/v1/tics/rankings$",                                        # market themes
    r"^/api/v2/trading/order/buy-control/required-deposit-amount$",    # order funding
    r"^/api/v1/boards/popular-follower$",                              # community boards
    r"^/api/v1/trade-purpose-verification/status$",                    # account detail (거래목적 심사)
    # v2 만 구현했다 — v1 은 미국옵션 티어를 늘 null 로 준다 (2026-08-04 라이브 확인).
    r"^/api/v2/trading/commission-info$",
    r"^/api/v4/dashboard/wts/overview/indicator$",                     # market halt · market anomalies
    r"^/api/v1/dashboard/wts/overview/ai-signals$",                     # quote reasons (batch)
    r"^/api/v1/prime/users/benefits/cumulative$",                       # account prime (누적)
    r"^/api/v1/dashboard/common/stocks/mini-chart$",                    # quote charts (batch)
    r"^/api/v\d+/my-assets/summaries/",
    r"^/api/v\d+/my-assets/transactions/",
    r"^/api/v1/product/stock-prices",
    r"^/api/v\d+/stock-prices/[^/]+/(ticks|upper-lower|quotes|details)",
    r"^/api/v3/stock-prices/details",
    r"^/api/v\d+/stock-infos/[^/]+$",
    r"^/api/v1/stock-infos/[^/]+/wts-badges",
    r"^/api/v1/stock-infos/trade/trend/trading-trend",
    r"^/api/v1/stock-detail/ui/[^/]+/common",
    r"^/api/v2/search/stocks",
    r"^/api/v1/c-chart/",
    r"^/api/v1/rankings/realtime/stock",
    r"^/api/v\d+/new-watchlists",
    r"^/api/v2/screener/",
    r"^/api/v1/dashboard/wts/overview/(exchange-rates|indicator/index)",
    r"^/api/v1/dashboard/common/cached-orderable-amount",
    r"^/api/v1/lending/revenue/account/expected$",
    r"^/api/v1/lending/revenue/account/top-revenue$",
    r"^/api/v2/dashboard/asset/sections",
    # 정확히 부르는 것만 건다. 접두사로 두면 부르지도 않는 형제 경로까지
    # implemented 가 된다 — `/usd/base-exchange-rate/{date}`(응답 스키마가 다른
    # 별개 API)와 `current-quote`·`/for-sell` 이 그렇게 잘못 표시돼 있었다.
    r"^/api/v1/exchange/current-quote/for-buy$",
    r"^/api/v1/exchange/usd/base-exchange-rate$",
    r"^/api/v\d+/trading/my-orders/",
    r"^/api/v1/trading/orders/calculate/[^/]+/(orderable-quantity|cost-basis-elements|average-price)",
    r"^/api/v2/trading/orders/calculate/[^/]+/cost-basis-elements",
    r"^/api/v1/trading/orders/histories/all/pending",
    r"^/api/v2/wts/trading/order/(create|prepare|cancel|correct)",
    r"^/api/v3/wts/trading/order/cancel/[^/]+/[^/]+$",
    r"^/api/v3/trading/order/[^/]+/available-actions$",
    r"^/api/v1/trading/settings/toggle",
    r"^/api/v2/system/trading-hours",
    r"^/api/v1/session/expired-at",
    r"^/api/v1/wts-login-extend(?:/.*)?$",
    r"^/api/v2/reasoning-contents/interest",
    r"^/api/v1/dashboard/wts/overview/rankings/by-investors$",  # market investors
    r"^/api/v1/earning-call/upcoming$",                          # market earnings
    r"^/api/v1/earning-call/home$",                              # market earnings --major
    r"^/api/v1/community/top-rankings(?:/[^/]+)?$",              # community rankings
    r"^/api/v1/dashboard/wts/overview/ai-signals/personalized$", # market briefing
    r"^/api/v1/dashboard/wts/overview/ai-signals/latest$",       # market briefing --scope kr|us
    r"^/api/v1/dashboard/wts/overview/ai-signals/detail$",       # market signal <symbol>
    r"^/api/v1/dividends/accounts/annual/history",               # portfolio dividends
    r"^/api/v1/prime/users/(info|benefits)$",                    # account prime
    r"^/api/v1/tics/all$",                                        # market sectors
    r"^/api/v2/dashboard/wts/overview/tics/[^/]+/(simple|overview|stocks|etfs|news)$", # market sector
    r"^/api/v1/tics/rankings$",                                   # market themes
    r"^/api/v1/index-prices(?:/[^/]+)?$",                          # market index <code> (지수 상세)
    r"^/api/v2/autotrade/plan/find$",                             # accumulate list
    r"^/api/v1/growth/autotrade/plan/stock(?:/[^/]+)?$",          # accumulate status
    r"^/api/v1/profit/overview$",                                 # profit
    r"^/api/v3/profit/readable-tab$",                             # profit (tab meta)
    r"^/api/v1/dashboard/wts/news$",                             # market news
    r"^/api/v1/lens/issues$",                                    # market issues
    r"^/api/v3/trading/auto-trading/histories$",                  # order autotrade
    r"^/api/v4/calendar/monthly(?:/[^/]+)?$",                     # market calendar
    r"^/api/v1/nova-calendar/ai/summary/weekly$",                 # market calendar (AI 요약)
    r"^/api/v1/account/detail$",                                  # account detail
    r"^/api/v1/transfer/withdrawable-status$",                    # account detail
    r"^/api/v1/dashboard/wts/overview/margin$",                   # account detail
    r"^/api/v1/margin/cert/differential-margin/enabled$",         # account detail
    r"^/api/v1/trade-purpose-verification/transfer-limit-restricted$",  # account detail
    r"^/api/v1/rights/us/dividend-option/account-give-type$",     # account detail (US dividend option)
    r"^/api/v1/profit/type/overview$",                            # profit summary
    r"^/api/v1/profit/wts/daily/market$",                         # profit daily
    r"^/api/v1/my-assets/transfer-income/overseas$",              # tax overseas
    r"^/api/v1/wts-notification$",                                # push SSE stream
]

# recommended: candidates worth implementing next (data/discovery features that
# fit tossctl's read surface). Tagged priority="next" so the catalog/monitor can
# surface "good to add next" separately from the long tail of candidates.
RECOMMENDED = [
    (r"^/api/v1/dividends/", "배당 내역/캘린더"),
    (r"^/api/v1/earning-call/", "실적발표(어닝콜) 일정"),
    (r"^/api/v1/crypto-prices", "가상자산 시세"),
    (r"^/api/v\d+/dashboard/wts/overview/ai-signals", "AI 시그널 확장"),
    (r"^/api/v\d+/dashboard/wts/overview/rankings/by-investors", "투자자별 랭킹(수급 discovery)"),
    (r"^/api/v1/companies/tics/rankings", "업종(TICS) 랭킹"),
    (r"^/api/v\d+/dashboard/wts/overview/tics", "업종(TICS) 개요·랭킹"),
    (r"^/api/v1/community/top-rankings", "커뮤니티 랭킹(인플루언서/수익률)"),
    (r"^/api/v1/r-chart", "실시간 차트"),
    (r"^/api/v\d+/prime/users/(benefits|info)", "토스프라임 혜택·구독 상태"),
    (r"^/api/v\d+/lending/revenue", "대주(주식대여) 수익"),
]

# excluded: out of scope. (pattern, reason)
EXCLUDED = [
    (r"^/api/v\d+/(account-open|multi-account-open)", "account opening flow"),
    (r"^/api/v\d+/account/additional-account-open", "account opening flow"),
    (r"^/api/v\d+/account/frontend/(terms|product-eligibility|opening|pension|ria|minor|mip|contracts|test|is-test)", "onboarding/eligibility UI"),
    (r"^/api/v\d+/account/(fatca|investment-propensity|report|product-detail|locked-status|change-account|detail)", "account admin / tax / KYC"),
    # 권리 행사(exercises) = 배당 수령 방식 변경 같은 계좌 설정 쓰기. 라이브에서
    # POST 403 이고, `account detail` 이 계좌 변경 동작을 노출하지 않는 것과 같은
    # 기준으로 제외한다. 조회 쪽(dividend-option/account-give-type)만 구현.
    (r"^/api/v\d+/rights/[^/]+/exercises/", "account-setting write (권리 행사)"),
    (r"^/api/v\d+/kyc", "KYC"),
    # 단수 `account/` 만 걸러내고 있었다 — 토스는 같은 성격의 KYC·계좌관리 API 를
    # **복수** `accounts/` 에도 둔다. 2026-08-24 스윕에서 40여건이 그대로 candidate 로
    # 새어 백로그를 부풀리고 있었다.
    (r"^/api/v\d+/accounts/(fatca|investment-propensity|contracts|closeable|password|differential-margin|detail|auto-trade/(auth|event))", "account admin / KYC"),
    (r"^/api/v\d+/accounts/(?:ssn-verification|close)(?:/|$)", "sensitive account administration"),
    (r"^/api/v\d+/multi-account", "multi-account opening/terms"),
    (r"^/api/v\d+/open-banking", "open-banking linkage"),
    (r"^/api/v\d+/risk-taker", "quiz/marketing"),
    (r"^/api/v\d+/giphy", "GIF search (community composer)"),
    (r"^/api/v\d+/dashboard/common/ongoing-events", "events/promotion"),
    (r"^/api/v\d+/community/terms-agreement", "legal terms"),
    (r"^/api/v\d+/promotion", "marketing/promotion"),
    (r"^/api/v\d+/minor", "minor-account flow"),
    (r"^/api/v\d+/pension", "pension account flow"),
    (r"^/api/v\d+/lending/(?!revenue)", "stock lending product"),
    (r"^/api/v\d+/(auto-transfer|transfer-income|rename-documents)", "transfer/document admin"),
    (r"^/api/v\d+/terms", "legal terms"),
    (r"^/api/v\d+/portal/agreement-modules", "legal terms UI"),
    (r"^/api/v\d+/login", "login flow (handled by auth-helper)"),
    (r"^/api/v\d+/session/refresh$", "auth/session plumbing"),
    (r"^/api/v\d+/common/auth/", "auth/KYC plumbing (handled by auth-helper)"),
    (r"^/api/v\d+/settings/password/", "account security flow"),
    (r"^/api/v\d+/tuba", "telemetry/AB"),
    (r"^/api/v\d+/nova-feedback/", "product feedback UI"),
    (r"^/api/v\d+/(user-profiles|personalize|settings|user-setting)", "UI personalization/prefs"),
    (r"^/api/v\d+/(memo|forum|comments|feed)", "community/UGC"),
    (r"^/api/v\d+/product-eligibility", "product eligibility gating"),
    (r"^/api/v\d+/(perf-log|log)/", "telemetry"),
    (r"^/api/v\d+/wts-login-device", "device registration"),
]


def fetch(path):
    try:
        req = urllib.request.Request(BASE + path, headers={"User-Agent": UA})
        return urllib.request.urlopen(req, timeout=25).read().decode("utf-8", "ignore")
    except Exception:
        return ""


# 앱 라우트별로 청크가 갈린다. `/` 와 `_buildManifest.js` 만 보면 **초기·공유 청크만**
# 잡히고, 지연 로딩되는 페이지 전용 청크는 통째로 안 보인다.
#
# 2026-08-03 에 이걸로 월간 증시 캘린더(`/api/v4/calendar/monthly/{month}`,
# `/api/v1/nova-calendar/ai/summary/weekly`)를 놓쳤다 — 카탈로그 949개 어디에도 없었고,
# `/calendar` HTML 을 받아보니 루트에 없는 청크가 10개 더 딸려 나왔다.
#
# 라우트 목록을 손으로 적으면 같은 실수가 규모만 줄어든 채 반복된다(처음 9개를 적었을
# 때 실제로는 43개를 더 놓치고 있었다). 그래서 **번들에서 라우트를 뽑아** 훑는다 —
# 토스가 화면을 추가하면 모니터가 알아서 따라간다.
#
# 각 라우트의 SSR HTML 에 그 라우트의 <script> 가 박혀 있으므로 브라우저 없이
# 순수 HTTP 로 수집된다(CI 에서 그대로 동작).

CHUNK_RE = r"/assets/v2/_next/static/chunks/[^\"']+\.js"

# 토스 번들은 엔드포인트를 `host:"cert",method:"GET",path:"/api/v1/..."` 삼중으로 박아둔다.
# 경로만 정규식으로 긁으면 두 가지를 통째로 잃는다:
#
#   1. **동적 세그먼트가 잘린다.** 경로 스크레이프 정규식은 `[` 에서 멈추므로
#      `/api/v1/asset-snapshot/chart/[range]/[stepUnit]` 이 `/api/v1/asset-snapshot/chart`
#      로 저장된다. 그 잘린 경로를 프로브하면 당연히 404 다 — 2026-08-03 스윕에서
#      85개가 잘린 키로 들어갔고 그중 33개가 그렇게 `not-found` 로 사장됐다.
#      재확인(2026-08-24)해보니 `trading/stocks/{stockCode}/average-price` 는 실제로는
#      `invalid.stock-code` 400 을 주는 살아있는 엔드포인트였다.
#
#   2. **호스트를 짐작하게 된다.** 토스는 wts-api/wts-info-api/wts-cert-api 를 섞어 쓴다.
#      프로브가 호스트를 순회하며 추측하느라 틀린 호스트의 404 를 정답으로 기록했다.
#
# 삼중을 그대로 읽으면 둘 다 사라진다.
TRIPLE_RE = re.compile(r'host:"([a-z\-]+)",method:"([A-Z]+)",path:"(/api/v\d+/[^"]+)"')

# 번들 토큰 → 실제 호스트. 두 개의 독립 관측으로 확정(2026-08-24):
# `/api/v1/account/list` 는 토큰이 launcher 인데 wts-api 로 나가고,
# `/api/v1/profit/overview` 는 토큰이 cert 인데 wts-cert-api 로 나간다.
HOST_TOKEN = {"launcher": "wts-api", "cert": "wts-cert-api", "info": "wts-info-api"}

# 잘린 형태인데 **실재하는** 엔드포인트. 여기 넣으려면 두 가지를 확인할 것:
#   1. 라이브로 200 이 오고, `/{param}` 버전과 **응답 스키마가 다르다** (같으면 그냥 그림자다)
#   2. 우리 코드가 그 경로를 그대로 부른다
#
# IMPLEMENTED 패턴으로 자동 판정하려 했으나 그건 가족 접두사라서 과복원한다 —
# 코드가 v3 만 부르는 my-assets/transactions/markets 의 v1 형태까지 되살아났다.
REAL_SHADOWS = {
    # 번들은 `/{date}` 만 선언하지만 날짜 없는 쪽이 별개 API 다. `rate` 필드는
    # 여기에만 있고 주문 경로가 그걸 읽는다. `/{date}` 는 usdMidRate 만 준다.
    # (2026-08-25 라이브 확인)
    "/api/v1/exchange/usd/base-exchange-rate",
}

# Contracts verified from a concrete call-site/static bundle trace but assembled
# dynamically enough that the generic string extractor cannot recover them.
# Keeping these in the generated inventory lets the weekly monitor retain and
# diff every endpoint tossctl actually calls, including safe write surfaces.
CURATED_CONTRACTS = {
    "/api/v1/user-price-alimy/{stockCode}/{currency}/{targetPrice}": {
        "method": "DELETE",
        "host": "wts-api",
        "evidence": "verified",
        "note": "Exact DELETE contract verified by WTS static analysis on 2026-09-03. Exposed through preview/confirm/post-read verification.",
    },
}

# 라우트가 아닌 것들: 에러 페이지, 정적 자산.
_ROUTE_SKIP = re.compile(r"^/(?:\d{3}|_|api/|assets/|static/)|\.(?:js|css|png|svg|json|webp|ico)$")

# 동적 세그먼트(`/stocks/[code]`)는 그대로 받을 수 없지만, **아무 값이나 넣어도 그
# 라우트의 청크는 그대로 내려온다** (2026-08-03 측정: `/stocks/ZZZZZZ` 가 실제 종목과
# 같은 12개를 준다). 그래서 건너뛰지 않고 치환해서 받는다 — 안 그러면 종목 상세·채권·
# 커뮤니티 글처럼 동적 라우트에만 있는 API 를 통째로 놓친다.
_ROUTE_PARAM = re.compile(r"\[[^\]]*\]")
_ROUTE_TOKEN = "1"


def discover_routes(blob):
    """번들 문자열에서 앱 라우트 후보를 뽑는다. 동적 세그먼트는 치환한다."""
    routes = {"/"}
    for m in re.finditer(r'href:"(/[^"?#]{1,40})"', blob):
        routes.add(m.group(1))
    for m in re.finditer(r'"(/[a-z0-9][a-z0-9\-]{1,25}(?:/[a-z0-9\-\[\]]{1,25}){0,3})"', blob):
        routes.add(m.group(1))
    out = set()
    for r in routes:
        if _ROUTE_SKIP.search(r):
            continue
        out.add(_ROUTE_PARAM.sub(_ROUTE_TOKEN, r))
    return sorted(out)


def collect_paths():
    idx = fetch("/")
    m = re.search(r'"buildId":"([^"]+)"', idx)
    build_id = m.group(1) if m else ""
    chunks = set(re.findall(CHUNK_RE, idx))
    if build_id:
        bm = fetch(f"/assets/v2/_next/static/{build_id}/_buildManifest.js")
        for f in re.findall(r'"(chunks/[^"]+\.js)"', bm):
            chunks.add("/assets/v2/_next/static/" + f)
        for f in re.findall(CHUNK_RE, bm):
            chunks.add(f)

    # 1차: 초기 청크를 읽어 라우트 목록을 알아낸다.
    with concurrent.futures.ThreadPoolExecutor(max_workers=12) as ex:
        seed = "\n".join(ex.map(fetch, chunks))
    routes = discover_routes(seed)

    # 2차: 각 라우트 HTML 에서 그 페이지 전용 청크를 걷는다.
    with concurrent.futures.ThreadPoolExecutor(max_workers=8) as ex:
        for html in ex.map(fetch, [r for r in routes if r != "/"]):
            chunks.update(re.findall(CHUNK_RE, html))

    # 정렬해서 받는다 — `chunks` 는 set 이라 순회 순서가 실행마다 달라지고, 그러면
    # 같은 번들에서 매번 다른 카탈로그가 나온다(한 경로가 PATCH/DELETE 를 둘 다 선언할 때
    # 먼저 읽힌 쪽이 이겼다). CI 가 매 실행 diff 를 만들어낸다.
    ordered = sorted(chunks)
    with concurrent.futures.ThreadPoolExecutor(max_workers=12) as ex:
        blob = "\n".join(ex.map(fetch, ordered))
    globals()["_ROUTE_COUNT"] = len(routes)
    norm, meta = derive_paths(blob)
    return build_id, len(chunks), norm, meta



def derive_paths(blob):
    """번들 텍스트에서 (정규화된 경로 집합, 경로별 메타) 를 뽑는다.

    네트워크와 분리된 순수 함수다. 이 안의 두 규칙 — 삼중 정의 우선, 그림자 제거 —
    에서 실제로 사고가 두 번 났다(실재 경로 삭제, 실행마다 다른 결과). 테스트가
    가능하려면 fetch 와 떨어져 있어야 한다.

    **입력 순서에 의존하지 않는다.** 청크를 읽는 순서가 달라도 결과가 같아야 한다 —
    한 경로가 여러 메서드를 선언할 때 먼저 읽힌 쪽이 이기면 같은 번들에서 매번
    다른 카탈로그가 나온다.
    """
    methods, hosts = {}, {}
    for token, method, path in TRIPLE_RE.findall(blob):
        p = _normalize(path)
        methods.setdefault(p, set()).add(method)
        if host := HOST_TOKEN.get(token):
            hosts[p] = host
    meta = {}
    for p, ms in sorted(methods.items()):
        meta[p] = {"method": ",".join(sorted(ms))}
        if p in hosts:
            meta[p]["host"] = hosts[p]

    raw = set(re.findall(r"/api/v[0-9]+/[a-zA-Z0-9/_.\-]+", blob))
    norm = set(meta)
    # 삼중 경로를 `[` 에서 자른 형태는 스크레이프에도 잡힌다. 대개는 실재하는
    # 엔드포인트가 아니라 같은 정의의 잘린 그림자이므로 버린다 — 남기면 프로브가
    # 그걸 때려 404 를 쌓는다.
    #
    # **단, REAL_SHADOWS 는 예외다.** 잘린 형태가 진짜 엔드포인트인 경우가 있다 —
    # 그렇게 지웠다가 실제 구현이 카탈로그에서 사라진 적이 있다(2026-08-25).
    shadows = {p.split("{")[0].rstrip("/") for p in meta if "{" in p} - set(meta) - REAL_SHADOWS
    for p in raw:
        p = _normalize(p)
        if p in shadows:
            continue
        norm.add(p)
    return norm, meta


def _normalize(p):
    """Go 템플릿·동적 세그먼트·숫자 id 를 하나의 identity 로 통일한다."""
    p = GO_FMT_VERB_RE.sub("{param}", p)
    p = re.sub(r"\[([^\]]*)\]", lambda m: "{" + (m.group(1) or "id") + "}", p)
    p = re.sub(r"/[0-9]{3,}(?=/|$)", "/{id}", p)
    return p.rstrip("/.")


def _legacy_key(p):
    """잘린 옛 카탈로그 키. `/api/v1/profit/{profitType}/{key}` → `/api/v1/profit`."""
    return p.split("/{")[0].rstrip("/") if "/{" in p else p


def find_override(path, overrides, known_paths=None):
    """Resolve an exact override, or a safe legacy-key override."""
    ov = overrides.get(path)
    legacy = _legacy_key(path)
    if not ov and (known_paths is None or legacy not in known_paths):
        ov = overrides.get(legacy)
    return ov


def apply_override_metadata(entry, override):
    """Fill bundle metadata gaps with independently verified override facts.

    Bundle-derived facts remain authoritative when present. Curated metadata is
    only a fallback for contracts such as write endpoints whose request method
    is assembled in a way the bundle extractor cannot recover.
    """
    if not override:
        return
    for key in ("method", "host", "evidence"):
        if override.get(key) and not entry.get(key):
            entry[key] = override[key]


def classify(path, overrides, known_paths=None):
    ov = find_override(path, overrides, known_paths)
    if ov:
        return ov["status"], ov.get("note", "")
    for pat in IMPLEMENTED:
        if re.search(pat, path):
            return "implemented", ""
    for pat, reason in EXCLUDED:
        if re.search(pat, path):
            return "excluded", reason
    return "candidate", ""


def _run_go_inventory(repo_root, mode):
    project_root = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
    command = [
        "go", "run", "./tools/wtsinventory",
        "-mode", mode,
        "-root", os.path.abspath(repo_root),
    ]
    if mode == "exposures":
        command.extend(["-roots", ",".join(GO_WTS_SOURCE_ROOTS)])
    result = subprocess.run(
        command,
        cwd=project_root,
        check=False,
        capture_output=True,
        text=True,
    )
    if result.returncode != 0:
        raise RuntimeError(f"Go inventory {mode} failed: {result.stderr.strip()}")
    return json.loads(result.stdout)


def discover_go_exposures(repo_root):
    """Return production WTS endpoint literals and the Go files that own them.

    This is the Go exposure adapter for the inventory module. It deliberately
    ignores tests and query strings: classification owns endpoint identity,
    while request fixtures and query values remain implementation details.
    Dynamic suffixes still classify through the existing anchored family
    patterns (for example ``/wts-login-extend/doc/``).
    """
    exposures = {}
    for raw, owners in _run_go_inventory(repo_root, "exposures").items():
        path = _normalize(raw)
        exposures.setdefault(path, set()).update(owners)
    return {path: sorted(owners) for path, owners in exposures.items()}


def discover_go_probes(repo_root):
    """Read the exact probe list returned by the Go monitor runtime."""
    probes = _run_go_inventory(repo_root, "probes")
    for probe in probes:
        probe["path"] = _normalize(probe["path"])
    return probes


def _probe_inventory_path(path):
    """Turn fixed probe symbols into the reusable endpoint template they verify."""
    path = re.sub(
        r"(^/api/v2/dashboard/wts/overview/tics/)[0-9]+(?=/)",
        r"\1{id}",
        path,
        count=1,
    )
    for prefix in ("stock-infos", "stock-prices", "index-prices"):
        path = re.sub(
            rf"(^/api/v[0-9]+/{prefix}/)[A-Z][A-Z0-9]{{5,}}",
            rf"\1{{code}}",
            path,
            count=1,
        )
    path = re.sub(
        r"(^/api/v1/user-price-alimy/)[A-Z][A-Z0-9]{5,}$",
        r"\1{stockCode}",
        path,
        count=1,
    )
    path = re.sub(r"(^/api/v4/calendar/monthly/)[^/]+$", r"\1{month}", path)
    return path


def find_inventory_entry(endpoints, path):
    """Find an exact or explicitly templated inventory entry."""
    normalized = _normalize(path.split("?", 1)[0])
    if normalized in endpoints:
        return endpoints[normalized]

    for candidate, entry in endpoints.items():
        pattern = re.escape(candidate)
        pattern = re.sub(r"\\\{[^}]+\\\}", r"[^/]+", pattern)
        if re.fullmatch(pattern, normalized):
            return entry

    return None


def hosts_compatible(path, actual, inventory):
    if actual == inventory:
        return True
    aliases = KNOWN_HOST_ALIASES.get(_normalize(path.split("?", 1)[0]))
    return aliases == {actual, inventory}


def probe_inventory_mismatches(probes, endpoints):
    """Return every missing or contradictory probe/inventory fact."""
    mismatches = []
    for probe in probes:
        entry = find_inventory_entry(endpoints, probe["path"])
        if not entry:
            mismatches.append((probe["name"], "inventory", probe["path"], "missing"))
            continue
        observed = entry.get("observed", {})
        known_host = entry.get("host") or observed.get("host")
        known_method = entry.get("method") or observed.get("method")
        if not known_host:
            mismatches.append((probe["name"], "host", probe["host"], "missing"))
        elif not hosts_compatible(probe["path"], probe["host"], known_host):
            mismatches.append((probe["name"], "host", probe["host"], known_host))
        if not known_method:
            mismatches.append((probe["name"], "method", probe["method"], "missing"))
        elif probe["method"] not in known_method.split(","):
            mismatches.append((probe["name"], "method", probe["method"], known_method))
    return mismatches


def main():
    prev = {}
    if os.path.exists(CATALOG):
        prev = json.load(open(CATALOG, encoding="utf-8"))
    overrides = prev.get("overrides", {})
    prev_eps_map = prev.get("endpoints", {})
    prev_eps = set(prev_eps_map.keys())

    build_id, n_chunks, paths, meta = collect_paths()
    if not paths:
        # Never overwrite the catalog on a failed/empty fetch — that would
        # look like "every endpoint was removed". Bail loudly instead.
        print("ERROR: no endpoints extracted (fetch failed?)", file=sys.stderr)
        return 1
    if suspicious_inventory_shrink(len(prev_eps), len(paths)):
        # A partial route/chunk fetch once collapsed 1,112 known paths to 325
        # while the WTS build id itself was unchanged. Treat that as collection
        # failure, not as a mass endpoint deletion, and preserve the catalog.
        print(
            f"ERROR: extracted endpoint count collapsed from {len(prev_eps)} to {len(paths)}; "
            "refusing to overwrite the catalog",
            file=sys.stderr,
        )
        return 1

    # The web bundle sometimes omits a reusable dynamic template even though
    # monitor.Probes executes a concrete representative URL. Merge those
    # runtime contracts into the generated inventory so host/method checks do
    # not depend on a hard-coded stock symbol surviving bundle extraction.
    repo_root = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
    for probe in discover_go_probes(repo_root):
        path = _probe_inventory_path(probe["path"])
        paths.add(path)
        facts = meta.setdefault(path, {})
        methods = set(facts.get("method", "").split(",")) - {""}
        methods.add(probe["method"])
        facts["method"] = ",".join(sorted(methods))
        facts.setdefault("host", probe["host"])

    for path, contract in CURATED_CONTRACTS.items():
        paths.add(path)
        facts = meta.setdefault(path, {})
        for key, value in contract.items():
            facts.setdefault(key, value)

    # 이번 추출에서 없어진 옛 키 — 잘린 그림자가 정식 경로로 승격되면 여기 들어온다.
    gone = prev_eps - paths
    today = os.environ.get("WTS_DATE") or datetime.date.today().isoformat()
    endpoints, counts = {}, {"implemented": 0, "candidate": 0, "excluded": 0}
    next_count = 0
    for p in sorted(paths):
        status, note = classify(p, overrides, paths)
        entry = {"status": status}
        if note:
            entry["note"] = note
        # priority="next": curated high-value candidates worth adding next.
        if status == "candidate":
            for pat, why in RECOMMENDED:
                if re.search(pat, p):
                    entry["priority"] = "next"
                    entry["note"] = why
                    next_count += 1
                    break
        # first_seen lifecycle: preserve prior date so churn is visible.
        # 번들 삼중에서 온 호스트·메서드. 프로브가 호스트를 추측하지 않도록 남긴다.
        if m := meta.get(p):
            entry.update(m)
        # 일부 쓰기 계약은 번들에서 메서드가 동적으로 조립돼 추출기가 놓친다.
        # 별도로 검증해 override 에 남긴 사실은 빈칸만 보완하고, 새 번들 관측값은
        # 절대 덮어쓰지 않는다.
        apply_override_metadata(entry, find_override(p, overrides, paths))
        # first_seen lifecycle: 잘린 옛 키(`/api/v1/profit`)에서 정식 키
        # (`/api/v1/profit/{profitType}/{key}`)로 옮겨온 것은 이력을 이어받는다.
        legacy = _legacy_key(p)
        prior = prev_eps_map.get(p) or (prev_eps_map.get(legacy) if legacy in gone else None) or {}
        entry["first_seen"] = prior.get("first_seen", today)
        # probe: the live-sweep verdict from tools/probe_candidates.py. It is
        # human/agent triage state, not something this extractor can rederive,
        # so it must survive regeneration — otherwise the weekly monitor wipes
        # the backlog triage every Monday and candidates stay undifferentiated
        # forever, which is the problem the sweep exists to fix.
        # 프로브는 사람/에이전트의 트리아지 상태라 재생성을 견뎌야 한다. 다만 **번들이
        # 알려준 호스트와 다른 호스트로 잰 기록은 버린다** — 그건 다른 URL 을 잰 값이다.
        # 2026-08-03 스윕은 호스트를 순회하며 추측했고, 잘린 경로까지 겹쳐 33건이
        # 위양성 `not-found` 로 남았다. 재확인(2026-08-24) 6건 중 5건이 실제로는
        # 살아있는 엔드포인트였다.
        if prior_probe := prior.get("probe"):
            known = entry.get("host")
            if not known or prior_probe.get("host") == known:
                entry["probe"] = prior_probe
        # observed: capture_post_bodies.mjs --sweep 이 기록한 실제 요청의 파라미터
        # 키와 호스트. probe 와 같은 이유로 보존해야 한다 — 이 추출기가 다시
        # 만들어낼 수 없는 관측값이다.
        if prior_obs := prior.get("observed"):
            entry["observed"] = prior_obs
        endpoints[p] = entry
        counts[status] = counts.get(status, 0) + 1
    counts["candidate_next"] = next_count
    # meaningful = real read/trade surface, excluding onboarding/KYC/promo/
    # telemetry noise. This is the honest denominator for "official API covers
    # only a fraction of WTS" — not the raw total.
    counts["meaningful"] = counts["implemented"] + counts["candidate"]

    added = sorted(paths - prev_eps)
    removed = sorted(prev_eps - paths)

    out = {
        "source": "tossinvest.com web bundles",
        "build_id": build_id,
        "chunk_count": n_chunks,
        "total": len(paths),
        "counts": counts,
        "overrides": overrides,
        "endpoints": endpoints,
    }
    # updated_at stamped by caller (CI) to keep runs deterministic; default today
    out["updated_at"] = os.environ.get("WTS_DATE") or datetime.date.today().isoformat()

    os.makedirs(os.path.dirname(CATALOG), exist_ok=True)
    with open(CATALOG, "w", encoding="utf-8") as f:
        json.dump(out, f, ensure_ascii=False, indent=2)
        f.write("\n")

    print(f"WTS endpoints: {len(paths)} total "
          f"(implemented {counts['implemented']}, candidate {counts['candidate']}, "
          f"excluded {counts['excluded']}) · build {build_id} · {n_chunks} chunks")
    if added:
        print(f"\n+ {len(added)} NEW since catalog:")
        for p in added:
            print("   +", p, "->", endpoints[p]["status"])
    if removed:
        print(f"\n- {len(removed)} removed:")
        for p in removed:
            print("   -", p)
    # machine-readable diff for CI
    if os.environ.get("WTS_DIFF_OUT"):
        json.dump(build_diff(prev, out, added, removed),
                  open(os.environ["WTS_DIFF_OUT"], "w"))
    return 0


def build_diff(previous, current, added, removed):
    """Return the stable machine payload consumed by the monitor workflow."""
    previous_build = previous.get("build_id", "")
    current_build = current.get("build_id", "")
    previous_chunks = previous.get("chunk_count")
    current_chunks = current.get("chunk_count")
    return {
        "added": added,
        "removed": removed,
        "new_candidates": [
            path for path in added
            if current["endpoints"][path]["status"] == "candidate"
        ],
        "build_changed": bool(previous_build and current_build and previous_build != current_build),
        "previous_build_id": previous_build,
        "current_build_id": current_build,
        "chunk_count_changed": previous_chunks is not None and previous_chunks != current_chunks,
        "previous_chunk_count": previous_chunks,
        "current_chunk_count": current_chunks,
    }


def suspicious_inventory_shrink(previous_count, current_count):
    """Flag a likely partial-fetch result without blocking normal API churn."""
    return previous_count >= 100 and current_count < previous_count * 0.75


if __name__ == "__main__":
    sys.exit(main())
