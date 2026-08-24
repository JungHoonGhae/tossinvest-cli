#!/usr/bin/env python3
"""Track the Toss WTS web API surface and classify each endpoint.

Toss's web app has no public spec, so we extract every `/api/vN/...` path from
the production JS bundles and classify it:

  implemented — tossctl already exposes this (mapped to a command)
  excluded    — intentionally out of scope (onboarding/KYC/promo/telemetry/UI)
  candidate   — not yet implemented; a lead for a future tossctl feature

Run with no args to refresh docs/reverse-engineering/wts-endpoints.json and
print a summary + any endpoints added/removed since the committed catalog.
Exit code 0 always; the workflow decides what to do with the diff.

stdlib only (runs in CI without deps).
"""
import concurrent.futures
import datetime
import json
import os
import re
import sys
import urllib.request

BASE = "https://www.tossinvest.com"
UA = ("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 "
      "(KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36")
CATALOG = os.path.join("docs", "reverse-engineering", "wts-endpoints.json")

# ── classification rules ──────────────────────────────────────────────────
# implemented: a path is "implemented" if it matches any of these (these mirror
# the endpoints internal/client/*.go actually calls).
IMPLEMENTED = [
    r"^/api/v1/account/list$",
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
    r"^/api/v4/dashboard/wts/overview/indicator$",                     # market halt
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
    r"^/api/v2/dashboard/asset/sections",
    r"^/api/v1/exchange/(current-quote|usd/base-exchange-rate)",
    r"^/api/v\d+/trading/my-orders/",
    r"^/api/v1/trading/orders/calculate/[^/]+/(orderable-quantity|cost-basis-elements|average-price)",
    r"^/api/v2/trading/orders/calculate/[^/]+/cost-basis-elements",
    r"^/api/v1/trading/orders/histories/all/pending",
    r"^/api/v2/wts/trading/order/(create|prepare|cancel|correct)",
    r"^/api/v1/trading/settings/toggle",
    r"^/api/v2/system/trading-hours",
    r"^/api/v1/session/expired-at",
    r"^/api/v1/wts-login-extend/",
    r"^/api/v2/reasoning-contents/interest",
    r"^/api/v1/dashboard/wts/overview/rankings/by-investors$",  # market investors
    r"^/api/v1/earning-call/upcoming$",                          # market earnings
    r"^/api/v1/earning-call/home$",                              # market earnings --major
    r"^/api/v1/community/top-rankings/",                         # community rankings
    r"^/api/v1/dashboard/wts/overview/ai-signals/personalized$", # market briefing
    r"^/api/v1/dividends/accounts/annual/history",               # portfolio dividends
    r"^/api/v1/prime/users/(info|benefits)$",                    # account prime
    r"^/api/v1/tics/all$",                                        # market sectors
    r"^/api/v1/tics/rankings$",                                   # market themes
    r"^/api/v1/index-prices$",                                    # market index <code> (지수 상세)
    r"^/api/v2/autotrade/plan/find$",                             # accumulate list
    r"^/api/v1/growth/autotrade/plan/stock$",                     # accumulate status
    r"^/api/v1/profit/overview$",                                 # profit
    r"^/api/v3/profit/readable-tab$",                             # profit (tab meta)
    r"^/api/v1/dashboard/wts/news$",                             # market news
    r"^/api/v1/lens/issues$",                                    # market issues
    r"^/api/v3/trading/auto-trading/histories$",                  # order autotrade
    r"^/api/v4/calendar/monthly$",                                # market calendar
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
    (r"^/api/v\d+/login", "login flow (handled by auth-helper)"),
    (r"^/api/v\d+/common/auth/", "auth/KYC plumbing (handled by auth-helper)"),
    (r"^/api/v\d+/tuba", "telemetry/AB"),
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
    # 삼중 정의가 있는 것은 그쪽이 진실이다 — 경로가 안 잘리고 호스트·메서드까지 온다.
    # 한 경로가 여러 메서드를 선언하는 일이 있다(REST 리소스면 자연스럽다). 하나만
    # 남기면 어느 쪽을 골라도 거짓이므로 전부 정렬해 기록한다.
    methods, hosts = {}, {}
    for token, method, path in TRIPLE_RE.findall(blob):
        p = _normalize(path)
        methods.setdefault(p, set()).add(method)
        if host := HOST_TOKEN.get(token):
            hosts[p] = host
    meta = {}
    for p, ms in methods.items():
        meta[p] = {"method": ",".join(sorted(ms))}
        if p in hosts:
            meta[p]["host"] = hosts[p]

    raw = set(re.findall(r"/api/v[0-9]+/[a-zA-Z0-9/_.\-]+", blob))
    norm = set(meta)
    # 삼중 경로를 `[` 에서 자른 형태는 스크레이프에도 잡힌다. 그건 실재하는 엔드포인트가
    # 아니라 같은 정의의 잘린 그림자이므로 버린다 — 남기면 프로브가 그걸 때려 404 를 쌓는다.
    shadows = {p.split("{")[0].rstrip("/") for p in meta if "{" in p} - set(meta)
    for p in raw:
        p = _normalize(p)
        if p in shadows:
            continue
        norm.add(p)
    return build_id, len(chunks), norm, meta


def _normalize(p):
    """동적 세그먼트를 `{name}` 으로, 숫자 id 를 `{id}` 로 통일한다."""
    p = re.sub(r"\[([^\]]*)\]", lambda m: "{" + (m.group(1) or "id") + "}", p)
    p = re.sub(r"/[0-9]{3,}(?=/|$)", "/{id}", p)
    return p.rstrip("/.")


def _legacy_key(p):
    """잘린 옛 카탈로그 키. `/api/v1/profit/{profitType}/{key}` → `/api/v1/profit`."""
    return p.split("/{")[0].rstrip("/") if "/{" in p else p


def classify(path, overrides):
    ov = overrides.get(path) or overrides.get(_legacy_key(path))
    if ov:
        return ov["status"], ov.get("note", "")
    for pat in IMPLEMENTED:
        if re.search(pat, path):
            return "implemented", ""
    for pat, reason in EXCLUDED:
        if re.search(pat, path):
            return "excluded", reason
    return "candidate", ""


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

    # 이번 추출에서 없어진 옛 키 — 잘린 그림자가 정식 경로로 승격되면 여기 들어온다.
    gone = prev_eps - paths
    today = os.environ.get("WTS_DATE") or datetime.date.today().isoformat()
    endpoints, counts = {}, {"implemented": 0, "candidate": 0, "excluded": 0}
    next_count = 0
    for p in sorted(paths):
        status, note = classify(p, overrides)
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
        json.dump({"added": added, "removed": removed,
                   "new_candidates": [p for p in added if endpoints[p]["status"] == "candidate"]},
                  open(os.environ["WTS_DIFF_OUT"], "w"))
    return 0


if __name__ == "__main__":
    sys.exit(main())
