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
    (r"^/api/v\d+/kyc", "KYC"),
    (r"^/api/v\d+/promotion", "marketing/promotion"),
    (r"^/api/v\d+/minor", "minor-account flow"),
    (r"^/api/v\d+/pension", "pension account flow"),
    (r"^/api/v\d+/lending/(?!revenue)", "stock lending product"),
    (r"^/api/v\d+/(auto-transfer|transfer-income|rename-documents)", "transfer/document admin"),
    (r"^/api/v\d+/terms", "legal terms"),
    (r"^/api/v\d+/login", "login flow (handled by auth-helper)"),
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


def collect_paths():
    idx = fetch("/")
    m = re.search(r'"buildId":"([^"]+)"', idx)
    build_id = m.group(1) if m else ""
    chunks = set(re.findall(r"/assets/v2/_next/static/chunks/[^\"']+\.js", idx))
    if build_id:
        bm = fetch(f"/assets/v2/_next/static/{build_id}/_buildManifest.js")
        for f in re.findall(r'"(chunks/[^"]+\.js)"', bm):
            chunks.add("/assets/v2/_next/static/" + f)
        for f in re.findall(r"/assets/v2/_next/static/chunks/[^\"']+\.js", bm):
            chunks.add(f)
    with concurrent.futures.ThreadPoolExecutor(max_workers=12) as ex:
        blob = "\n".join(ex.map(fetch, chunks))
    raw = set(re.findall(r"/api/v[0-9]+/[a-zA-Z0-9/_.\-]+", blob))
    norm = set()
    for p in raw:
        p = re.sub(r"/[0-9]{3,}(?=/|$)", "/{id}", p).rstrip("/.")
        norm.add(p)
    return build_id, len(chunks), norm


def classify(path, overrides):
    if path in overrides:
        return overrides[path]["status"], overrides[path].get("note", "")
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

    build_id, n_chunks, paths = collect_paths()
    if not paths:
        # Never overwrite the catalog on a failed/empty fetch — that would
        # look like "every endpoint was removed". Bail loudly instead.
        print("ERROR: no endpoints extracted (fetch failed?)", file=sys.stderr)
        return 1

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
        entry["first_seen"] = prev_eps_map.get(p, {}).get("first_seen", today)
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
