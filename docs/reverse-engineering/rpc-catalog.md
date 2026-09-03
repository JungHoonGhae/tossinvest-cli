# Toss Securities RPC Catalog

Verified from public web traffic and public page navigation on 2026-03-11.

This file is the source of truth for endpoint discovery. It should grow before the Go client grows.

## Status Legend

- `public`: works without login
- `guest`: works before authenticated account state, but may depend on browser bootstrap
- `auth`: requires a logged-in web session
- `blocked`: excluded from CLI scope
- `unknown`: not captured yet

## Hostnames

| Hostname | Role | Notes |
| --- | --- | --- |
| `wts-api.tossinvest.com` | core web runtime and session bootstrap | likely holds login and user-setting paths |
| `wts-info-api.tossinvest.com` | market and UI data | strong candidate for read-only quote and stock detail data |
| `wts-cert-api.tossinvest.com` | certified or sensitive read paths | comments, indicators, some overview widgets |
| `cdn-api.tossinvest.com` | refresh and static coordination | low direct CLI value so far |
| `tuba-static.tossinvest.com` | static variables | not a CLI target |
| `log.tossinvest.com` | telemetry | blocked from CLI scope |

## Bootstrap and Runtime

| Status | Method | Host | Path | Purpose | Observed shape | CLI mapping | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `guest` | `GET` | `wts-api.tossinvest.com` | `/api/v3/init?tabId=...` | browser tab bootstrap | `.result` is boolean `true` in public capture | none | useful for reproducing minimal browser session behavior |
| `public` | `GET` | `wts-api.tossinvest.com` | `/api/v1/time` | server time | object under `.result` | none | likely helpful for request signing or freshness checks later |
| `guest` | `GET` | `wts-api.tossinvest.com` | `/api/v1/user-setting` | current user or guest settings | object under `.result` | none | seen without login |
| `public` | `GET` | `wts-api.tossinvest.com` | `/api/v2/system/trading-hours/integrated` | trading-hours metadata | object under `.result` | future metadata | useful for quote context |
| `blocked` | `POST` | `log.tossinvest.com` | `/api/v1/perf-log/bulk` | telemetry | not relevant | none | never call from CLI |
| `blocked` | `POST` | `log.tossinvest.com` | `/api/v2/log/bulk` | telemetry | not relevant | none | never call from CLI |

## Login and Session Discovery

| Status | Method | Host | Path | Purpose | Observed shape | CLI mapping | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `guest` | `GET` | `www.tossinvest.com` | `/signin?redirectUrl=%2Faccount` | login page | HTML form with phone and QR flows | `auth login` entry | visiting `/account` without auth redirects here |
| `guest` | `POST` | `wts-api.tossinvest.com` | `/api/v2/login/wts/toss/cert-init` | login flow bootstrap | request body still undocumented | `auth login` helper only | observed both before and after login redirect |
| `guest` | `POST` | `wts-api.tossinvest.com` | `/api/v2/login/wts/toss/qr` | start QR-based login | request body still undocumented | `auth login` helper only | observed in successful QR flow |
| `guest` | `GET` | `wts-api.tossinvest.com` | `/api/v2/login/wts/toss/status` | poll QR login state | object under `.result` | `auth login` helper only | repeated polling until approval |
| `guest` | `POST` | `wts-api.tossinvest.com` | `/api/v2/login/wts/toss` | finalize Toss login | request body still undocumented | `auth login` helper only | observed after status polling |
| `guest` | `POST` | `wts-api.tossinvest.com` | `/api/v3/login/ticket` | obtain post-login ticket | request body still undocumented | `auth login` helper only | likely bridges login flow into WTS session |
| `auth` | `mixed` | browser cookies and storage | session persistence state | authenticated session reuse | cookies plus local/session storage | `auth status`, `auth login` | state-save capture showed both cookies and storage keys matter |
| `auth` | `GET` | `wts-api.tossinvest.com` | `/api/v1/openapi/client` | Open API key metadata and allowed IP list | `.result.allowedIps[]`; secret intentionally discarded | `openapi status`, `openapi ip list` | verified live; never map `clientSecret` |
| `auth` | `POST` | `wts-api.tossinvest.com` | `/api/v1/openapi/client/allowed-ips` | add one allowed IP | body `{"ip":"..."}` | `openapi ip replace-current` | verified from WTS bundle 2026-09-02; preview/confirm/rollback service |
| `auth` | `DELETE` | `wts-api.tossinvest.com` | `/api/v1/openapi/client/allowed-ips/{ip}` | remove one allowed IP | empty success body | `openapi ip replace-current` | verified from WTS bundle and browser flow 2026-09-02 |

## Market Overview

| Status | Method | Host | Path | Purpose | Observed shape | CLI mapping | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `public` | `GET` | `wts-info-api.tossinvest.com` | `/api/v1/dashboard/wts/overview/trading-info` | dashboard trading-hours cards | `.result.data[]` with `key`, `name`, `marketOpen`, `currentMarketTradingHour` | none | useful reference data, not first-class CLI target |
| `public` | `GET` | `wts-info-api.tossinvest.com` | `/api/v1/dashboard/wts/overview/exchange-rates` | exchange-rate summary | object under `.result` | none | may support quote context |
| `public` | `GET` | `wts-cert-api.tossinvest.com` | `/api/v1/dashboard/wts/overview/indicator/index?market=kr` | market indicators | `.result.majorIndicatorInfos` | none | public page dependency |
| `public` | `GET` | `wts-cert-api.tossinvest.com` | `/api/v1/dashboard/wts/overview/calendar/economic-events` | calendar snippets | object under `.result` | none | public page dependency |
| `public` | `POST` | `wts-cert-api.tossinvest.com` | `/api/v2/dashboard/wts/overview/ranking` | overview ranking widgets | object under `.result` | none | body contract still needs capture |
| `public` | `POST` | `wts-info-api.tossinvest.com` | `/api/v1/dashboard/intelligences/all` | dashboard cards | object under `.result` | none | body contract still needs capture |
| `public` | `POST` | `wts-info-api.tossinvest.com` | `/api/v2/dashboard/wts/overview/signals` | signal cards on stock detail/home | object under `.result` | none | body contract still needs capture |
| `auth` | `GET` | `wts-cert-api.tossinvest.com` | `/api/v2/reasoning/personalized` | personalized AI briefing | `.result.items[]` with asset, holding/watchlist context, return, reasoning title, news, and related stocks | `market briefing` | current WTS bundle + masked live schema verified 2026-09-02; replaces the thinner v1 CLI contract |
| `auth` | `GET` | `wts-cert-api.tossinvest.com` | `/api/v1/dashboard/wts/overview/ai-signals/latest?nationCode=KOR\|USA` | latest national AI briefing | `.result.items[]` with the same rich signal/news shape as personalized briefing | `market briefing --scope kr\|us` | exact nation enum from current bundle; read-only live schema verified 2026-09-03 |
| `auth` | `GET` | `wts-info-api.tossinvest.com` | `/api/v1/dashboard/wts/overview/ai-signals/detail?productCode=&productType=` | current AI signal detail | nullable `.result` with full reasoning, issue facts, keywords, source news, relationship graph and terms | `market signal <symbol> [--type]` | `productType=STOCKS\|EQUITY_ETF`; null is a valid no-current-signal state; static callsite + read-only live schema verified 2026-09-03 |
| `auth` | `GET` | `wts-info-api.tossinvest.com` | `/api/v2/dashboard/wts/overview/tics/{id}/simple` | current sector movement | `.result.{ticsId,name,summary,imageUrl,changeRate,duration}` | `market sector <id>` | read-only live schema verified 2026-09-03 |
| `auth` | `GET` | `wts-info-api.tossinvest.com` | `/api/v2/dashboard/wts/overview/tics/{id}/overview` | sector overview | `.result.{ticsId,name,summary,description,depth,companyCount,etfCount,relatedTics[]}` | `market sector <id>` | recursively preserves the related-sector hierarchy; combined with the following three paged dependencies; read-only live schema verified 2026-09-03 |
| `auth` | `POST` | `wts-info-api.tossinvest.com` | `/api/v2/dashboard/wts/overview/tics/{id}/stocks` | sector constituents | body `{}` → `.result.stocks[]` with price, valuation, volume and analyst opinion | `market sector <id>` | read-only live schema verified 2026-09-03 |
| `auth` | `POST` | `wts-info-api.tossinvest.com` | `/api/v2/dashboard/wts/overview/tics/{id}/etfs` | related sector ETFs | body `{}` → `.result.etfs[]` with price, expense ratio, leverage and top holding | `market sector <id>` | read-only live schema verified 2026-09-03 |
| `auth` | `GET` | `wts-info-api.tossinvest.com` | `/api/v2/dashboard/wts/overview/tics/{id}/news` | sector news | `.result.body[]` with headline, summary, source, timestamps and images | `market sector <id>` | read-only live schema verified 2026-09-03 |
| `auth` | `GET` | `wts-cert-api.tossinvest.com` | `/api/v1/calendar/ai-summary/key-events` | curated key earnings and economic releases | `.result.earnings[]`, `.result.eci.indicators[]` with estimates, actuals, surprise, and previous values | `market key-events` | current WTS bundle + read-only live schema verified 2026-09-02 |
| `auth` | `GET` | `wts-cert-api.tossinvest.com` | `/api/v1/earning-call/events/{eventId}/info` | one earnings-call report and published media | `.result` with company/stock/report metadata plus nullable audio, transcript, slides and surprise fields | `market earnings <event-id>` | exact path-only callsite + read-only live schema verified 2026-09-03; event IDs come from `market earnings` |

## Quote and Symbol Detail

| Status | Method | Host | Path | Purpose | Observed shape | CLI mapping | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `public` | `GET` | `wts-info-api.tossinvest.com` | `/api/v2/stock-infos/{code}` | symbol metadata | `.result` object with `symbol`, `name`, `market`, `currency`, `isinCode`, `status` | `quote get` | best starting point for product metadata |
| `public` | `GET` | `wts-info-api.tossinvest.com` | `/api/v1/stock-detail/ui/{code}/common` | stock detail UI metadata | `.result` object with `symbol`, `name`, `badges`, `notices`, `memoCount` | `quote get` | likely useful for enriched quote view |
| `public` | `GET` | `wts-info-api.tossinvest.com` | `/api/v1/product/stock-prices?meta=true&productCodes=...` | bulk price lookup | `.result[]` with `productCode`, `base`, `close`, `currency`, `exchange`, `volume` | `quote get`, watchlist | strong candidate for quote batch retrieval |
| `public` | `GET` | `wts-info-api.tossinvest.com` | `/api/v1/c-chart/kr-s/{code}/day:1?...` | chart candles | `.result` with `candles`, `exchange`, `exchangeRate`, `nextDateTime` | `quote chart` | 캡처 2026-06-03 |
| `public` | `GET` | `wts-info-api.tossinvest.com` | `/api/v2/stock-prices/{code}/ticks?viewType=krx_all&investMode=krx&count=N` | executed ticks (체결) | `.result[]` with `time`, `price`, `base`, `volume`, `tradeType` (BUY/SELL), `cumulativeVolume` | `quote trades` | KR=`krx_all`/`krx`, 그 외=`unified`/`unified`. 캡처 2026-06-03 |
| `public` | `GET` | `wts-info-api.tossinvest.com` | `/api/v2/stock-prices/{code}/upper-lower` | daily price band (상/하한가) | `.result` with `date`, `upperLimit`, `lowerLimit` | `quote limits` | 캡처 2026-06-03 |
| `public` | `GET` | `wts-info-api.tossinvest.com` | `/api/v1/stock-infos/{code}/wts-badges` | buy-caution badges (매수 유의) | `.result[]` (badge 객체, 정상 종목은 빈 배열) | `quote warnings` | badge shape 동적 — client 가 type/title/text/level 매핑 + raw 보존. 캡처 2026-06-03 |
| `public` | `GET` | `wts-api.tossinvest.com` | `/api/v2/system/trading-hours/integrated` | trading session windows (장 운영 시간) | `.result` with `kr`/`us` × `{prevBizDay, today, nextBizDay}` × `{startTime, endTime, ...}` | `market hours` | `today` 가 null = 휴장 (예: 선거일). 캡처 2026-06-03 |
| `public` | `GET` | `wts-cert-api.tossinvest.com` | `/api/v4/comments?subjectType=STOCK&subjectId=...` | community comments | object under `.result` | none | exclude from first release due to identity and moderation concerns |

## Rankings and Watch Surface

| Status | Method | Host | Path | Purpose | Observed shape | CLI mapping | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `public` | `GET` | `wts-info-api.tossinvest.com` | `/api/v1/rankings/realtime/stock?size=N` | realtime popularity ranking | `.result.data[]` (stock-info 객체, 순위순) | `market ranking` | 공식 API 에 없음. 캡처 2026-06-03 |
| `public` | `GET` | `wts-info-api.tossinvest.com` | `/api/v1/stock-infos?codes=...` | bulk metadata lookup | object under `.result` | future watchlist | useful companion to bulk price lookup |
| `public` | `GET` | `wts-info-api.tossinvest.com` | `/api/v1/stock-infos/trade/trend/trading-trend?productCode=&size=N` | investor net flows (수급) | `.result.body[]` with `baseDate`, `net{Individuals,Foreigner,Institution}BuyVolume` | `quote flows` | KRX 전용. 공식 API 에 없음. 캡처 2026-06-03 |
| `public` | `GET` | `wts-cert-api.tossinvest.com` | `/api/v1/dashboard/wts/overview/indicator/index` | market indices (지수) | `.result.majorIndicatorInfos[]` with `displayName`, `nation`, `price.{latestPrice,basePrice}` | `market index` | 코스피·나스닥·VIX 등. 공식 API 에 없음. 캡처 2026-06-03 |
| `public` | `GET` | `wts-info-api.tossinvest.com` | `/api/v2/index-infos/{code}` | index price-feed and trading-session metadata | `.result` with `priceFeedType.{code,description}`, `tradingStartAt`, `tradingEndAt`, `isMarketOpen` | `market index <code\|name>` | index price detail과 병렬 호출; 정적 계약과 typed fixture 검증 2026-09-03 |
| `public` | `GET` | `wts-info-api.tossinvest.com` | `/api/v2/reasoning-contents/interest` | Toss AI signals (AI 시그널) | `.result.{label,data[]}` with `assetName`, `title`, `keyword`, `fluctuationPhrase` | `market signals` | hero 정합. 공식 API 에 없음. 캡처 2026-06-03 |
| `public` | `GET` | `wts-cert-api.tossinvest.com` | `/api/v2/screener/presets/common?useCustom=true` | screener presets (조건검색 프리셋) | `.result[]` with `id`, `name`, `description`, `filters` | `market screener` | 캡처 2026-06-03 |
| `public` | `POST` | `wts-cert-api.tossinvest.com` | `/api/v2/screener/screen` | run screen | body `{pagingParam,filters,nation}` → `.result.{stocks[],totalCount}` | `market screener [id]`/`--filter` | filters 는 preset 또는 raw passthrough. body 는 fetch 후킹 리버싱. 캡처 2026-06-03 |

## Watchlist Management (mutation — new-watchlists)

토스 web 의 `channel=chrome`(실제 Chrome) 캡처 + 자기계좌 경험적 검증으로 리버싱.
모든 쓰기에 `X-XSRF-TOKEN`(= XSRF-TOKEN 쿠키값) 필요 — `applySession` 이 자동 적용.
비금융 설정 변경이라 거래 권한 게이트와 별개다. 생성·이름 변경·종목 추가·제거는
반대 작업으로 되돌릴 수 있지만 폴더 삭제는 순서와 구성까지 정확히 복구할 수 없어 불가역이다.
모든 쓰기는 기본 preview이며 현재 WTS 세션과 서버 상태에 묶인 5분짜리 confirm token을
요구한다. 삭제는 별도의 irreversible acknowledgement도 필요하고 preview에 삭제될 종목의
코드·심볼·이름을 표시한다. 이 token은 서버측 single-use/idempotency key가 아니므로 같은
preview의 동시 실행은 지원하지 않는다.

| Status | Method | Host | Path | Purpose | Body | CLI mapping |
| --- | --- | --- | --- | --- | --- | --- |
| `auth` | `GET` | `wts-cert-api.tossinvest.com` | `/api/v1/new-watchlists?includePrice=true&lazyLoad=false` | 전체 폴더의 종목을 한 번에 조회 | — | `watchlist list --all` |
| `auth` | `GET` | `wts-cert-api.tossinvest.com` | `/api/v1/new-watchlists/groups/simple?includeItemInfo=true` | 폴더 메타데이터와 종목 수 조회 | — | `watchlist groups` |
| `auth` | `GET` | `wts-cert-api.tossinvest.com` | `/api/v1/new-watchlists/groups?ids={id}&includePrice=true` | 선택한 폴더와 종목 조회 | — | `watchlist list {id}`, 쓰기 preview·사후 검증 |
| `auth` | `POST` | `wts-cert-api.tossinvest.com` | `/api/v1/new-watchlists/groups` | 폴더 생성 | `{"name":"..."}` → `.result.{id,name,...}` | `watchlist group create` |
| `auth` | `PATCH` | `wts-cert-api.tossinvest.com` | `/api/v1/new-watchlists/groups/{id}` | 폴더 리네임 | `{"name":"..."}` | `watchlist group rename` |
| `auth` | `DELETE` | `wts-cert-api.tossinvest.com` | `/api/v1/new-watchlists/groups/{id}` | 폴더 삭제 | — | `watchlist group delete` |
| `auth` | `POST` | `wts-cert-api.tossinvest.com` | `/api/v1/new-watchlists/items` | 종목 추가 | `{"watchlistIds":[id],"items":[{"code":"...","itemType":"STOCK"}]}` | `watchlist add` |
| `auth` | `POST` | `wts-cert-api.tossinvest.com` | `/api/v1/new-watchlists/items/remove` | 종목 제거 | `{"watchlistId":id,"items":[{"code":"...","itemType":"STOCK"}]}` | `watchlist remove` |

> **주의**: add 는 `watchlistIds` (복수, `[]int64`), remove 는 `watchlistId` (단수, `int64`). 서버 API 가 비일관적이지만 이것이 실제 스펙이다 (브라우저 DevTools 캡처로 확인).

## Account, Portfolio, Orders, Watchlist

These are approved CLI targets. Initial authenticated discovery happened on 2026-03-11 from the `/account` page after QR login.

| Status | Method | Host | Path | Purpose | Observed shape | CLI mapping | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `auth` | `GET` | `wts-api.tossinvest.com` | `/api/v1/account/list` | account list and primary account key | `.result.accountList`, `.result.primaryKey` | `account list` | high-value first endpoint; sanitize account identifiers |
| `auth` | `POST` | `wts-info-api.tossinvest.com` | `/api/v1/dashboard/all-accounts` | all-account asset rollup including minor accounts | body `{"sections":["SUMMARY_WITH_MINOR"]}` → `.result[0].data.{accountOverviews,minorAccountOverviews,totalAssetAmount}` | `account overview` | Android 5.275.0 contract + masked live schema verified 2026-09-02; account numbers are masked by default |
| `auth` | `GET` | `wts-cert-api.tossinvest.com` | `/api/v1/asset-snapshot/all-accounts/chart/ONE_MONTH/DAY` | one-month daily valuation trend across all Securities accounts | `.result.{hasKrStock,hasKrStockInRange,hasProduct,hasProductInRange,evaluatedAmountDiff,maxEvaluated,minEvaluated,points[]}` | `portfolio performance` | exact bundle call plus read-only live schema verified 2026-09-03; no current web UI; only the verified range/step pair is exposed |
| `auth` | `GET` | `wts-cert-api.tossinvest.com` | `/api/v1/asset-snapshot/chart/ONE_MONTH/DAY` | one-month daily valuation trend for one Securities account | same chart shape as all-accounts | `portfolio performance --account` | requires `accountKey`; output uses an opaque session-bound account scope instead of the key; verified 2026-09-03 |
| `auth` | `GET` | `wts-cert-api.tossinvest.com` | `/api/v1/asset-snapshot/all-accounts/page?pageSize=&cursorKey=` | cursor-paged dated valuations across all Securities accounts | `.result.{body[],nextCursorKey}` | `portfolio snapshots` | `pageSize` must be positive; `cursorKey` is optional; terminal cursor is null and a realtime point can be additional to pageSize; verified 2026-09-03 |
| `auth` | `GET` | `wts-cert-api.tossinvest.com` | `/api/v1/asset-snapshot/page?pageSize=&cursorKey=` | cursor-paged dated valuations for one Securities account | same page shape as all-accounts | `portfolio snapshots --account` | requires `accountKey`; output omits the raw key; verified 2026-09-03 |
| `auth` | `GET` | `wts-cert-api.tossinvest.com` | `/api/v1/asset-snapshot/all-accounts/detail-by-date?baseDate=YYYY-MM-DD` | full dated valuation across all Securities accounts | `.result` total plus `kr`, `option`, `us`, `bond` sections and each section's `items[]` | `portfolio snapshot <date>` | market summaries and holding-level quantity, purchase/evaluated amounts, and P/L are preserved; verified 2026-09-03 |
| `auth` | `GET` | `wts-cert-api.tossinvest.com` | `/api/v1/asset-snapshot/detail-by-date?baseDate=YYYY-MM-DD` | full dated valuation for one Securities account | same detail shape as all-accounts | `portfolio snapshot <date> --account` | requires `accountKey`; output omits the raw key; verified 2026-09-03 |
| `auth` | `GET` | `wts-cert-api.tossinvest.com` | `/api/v3/my-assets/summaries/markets/all/overview` | total assets and profit summary | `.result.accountNo`, `totalAssetAmount`, `evaluatedProfitAmount`, `profitRate`, `overviewByMarket` | `account summary`, `portfolio allocation` | account number appears in response |
| `auth` | `GET` | `wts-api.tossinvest.com` | `/api/v1/my-assets/summaries/markets/kr/withdrawable-amount` | KRW withdrawable amounts | `.result.amount0..amount3`, `.result.date0..date3` | `account summary` | public account summary dependency |
| `auth` | `GET` | `wts-api.tossinvest.com` | `/api/v1/my-assets/summaries/markets/us/withdrawable-amount` | USD withdrawable amounts | `.result.amount0..amount3`, `.result.date0..date3` | `account summary` | public account summary dependency |
| `auth` | `GET` | `wts-cert-api.tossinvest.com` | `/api/v1/trading/orders/histories/all/pending` | pending order history | `.result` list | `orders list` | initial capture returned an empty list |
| `auth` | `GET` | `wts-cert-api.tossinvest.com` | `/api/v1/dashboard/common/cached-orderable-amount` | orderable buying power | `.result.orderableAmountKr`, `.result.orderableAmountUs` | `account summary` | useful for summary view |
| `auth` | `POST` | `wts-cert-api.tossinvest.com` | `/api/v1/dashboard/asset/sections/all` | account dashboard sections | body `{"types":["MIDDLE"]}` (and others) | dashboard middle banner | filter required since 2026-05-13 (#29) |
| `auth` | `POST` | `wts-cert-api.tossinvest.com` | `/api/v2/dashboard/asset/sections/all` | account dashboard sections v2 | body `{"types":["SORTED_OVERVIEW"\|"WATCHLIST"\|...]}` | `portfolio positions`, `watchlist list` | **2026-05-13: empty `{}` body now returns empty sections + `pollIntervalMillis`. Must pass `types` filter.** |
| `auth` | `POST` | `wts-cert-api.tossinvest.com` | `/api/v2/dashboard/asset/sections/all` | user-defined portfolio folder overview | body `{"types":["FOLDER_OVERVIEW_V2"]}` → `.result.sections[].data.{folders[],hiddenStock}` with fee-aware summaries and holding items | `portfolio folders [--account]` | account-scoped; raw account/folder/item keys are omitted from output; typed fixture verified 2026-09-03 |
| `auth` | `POST` | `wts-cert-api.tossinvest.com` | `/api/v1/profit/overview` | profit overview widget | body contract unknown | `portfolio allocation` | body still needs capture |
| `auth` | `GET` | `wts-api.tossinvest.com` | `/api/v1/autotrade/open-banking/info/find` | open-banking account used for stock-accumulation funding | `.result.{name,connectedOpenBankingAccount,savingCount}` plus linked/registrable arrays | `accumulate funding-status` | current WTS bundle + masked live schema verified 2026-09-02; identity is masked by default and internal `openBankingId` is never emitted |
| `auth` | `GET` | `wts-api.tossinvest.com` | `/api/v1/autotrade/open-banking/creatable` | whether a new stock-accumulation funding connection can be created | `.result` boolean | `accumulate funding-status` | WTS call site + read-only live schema verified 2026-09-03 |
| `auth` | `GET` | `wts-api.tossinvest.com` | `/api/v1/autotrade/open-banking/need-registration` | whether stock-accumulation funding registration is required | `.result` boolean | `accumulate funding-status` | WTS call site + read-only live schema verified 2026-09-03 |
| `auth` | `GET` | `wts-cert-api.tossinvest.com` | `/api/v1/trading/open-banking/auto-trading` | automated-order funding registration within Securities | `.result.{connectedAccountBankCode,isRegistered}` | `accumulate funding-status` | exact WTS call site + read-only live schema verified 2026-09-03; not general Toss Banking |
| `auth` | `GET` | `wts-api.tossinvest.com` | `/api/v1/trade-purpose-verification/my-data/account/exists` | possible MyData link signal for the Securities trade-purpose verification flow | observed boolean `200` or `400` under the same valid session | not callable | deferred 2026-09-03: exact route is static, but a required state/header or stable response contract remains unknown; not general MyData access |
| `auth` | `GET` | `wts-api.tossinvest.com` | `/api/v1/user/last-login-info` | last Toss Securities login environment | `.result.{channel,osName,agentName,timestamp}` | `account access-status` | exact WTS call site + read-only live schema verified 2026-09-03; user-global signal |
| `auth` | `GET` | `wts-cert-api.tossinvest.com` | `/api/v1/margin/cert/frozen-account` | account-specific margin-trading freeze state and dates | `.result.{isFrozen,startDate,endDate}` | `account access-status [--account]` | requires `accountKey`; exact WTS call site + read-only live schema verified 2026-09-03 |
| `auth` | `GET` | `wts-api.tossinvest.com` | `/api/v2/account/unlock/accident-account/count` | account-specific accident-account count | `.result` number | `account access-status [--account]` | requires `accountKey`; exact WTS call site + read-only live schema verified 2026-09-03; command never invokes unlock |
| `auth` | `GET` | `wts-cert-api.tossinvest.com` | `/api/v1/trading/settings/simple-trade` | account-scoped simple-trade preference | `.result` boolean | `account trading-settings [--account]` | `accountKey` header; WTS call site + read-only live schema verified 2026-09-03 |
| `auth` | `GET` | `wts-cert-api.tossinvest.com` | `/api/v2/trading/settings/investor-exchange-choice-type` | selected KRX/NXT execution venue policy | `.result` string | `account trading-settings` | WTS call site + read-only live schema verified 2026-09-03 |
| `auth` | `GET` | `wts-cert-api.tossinvest.com` | `/api/v1/users/settings/me/ats-notification` | ATS notification preference | `.result` boolean | `account trading-settings` | WTS call site + read-only live schema verified 2026-09-03 |
| `auth` | `GET` | `wts-cert-api.tossinvest.com` | `/api/v1/member-subscriptions/get-option-real-time-tick` | option real-time tick request/service/billing flags | `.result.{requested,serviced,shouldCharged}` booleans | `account trading-settings` | field names preserved without inferring billing semantics; verified 2026-09-03 |
| `auth` | `GET` | `wts-cert-api.tossinvest.com` | `/api/v1/securities-transfer/my-accounts` | own accounts offered by the Securities stock-transfer flow | `.result[]` with `bankCode`, `accountNo`, `accountId` | `account transfer-accounts [--account]` | `accountKey` header; numbers masked by default and internal `accountId` never emitted; verified 2026-09-03 |
| `auth` | `GET` | `wts-cert-api.tossinvest.com` | `/api/v1/securities-transfer/recent-accounts` | recent destination accounts offered by the Securities stock-transfer flow | `.result[]` with `bankCode`, `accountNo` | `account transfer-accounts [--account]` | `accountKey` header; read-only lookup only; does not initiate a transfer; verified 2026-09-03 |
| `auth` | `GET` | `wts-cert-api.tossinvest.com` | `/api/v1/lending/revenue/account/top-revenue` | anonymized share-lending revenue ranking | `.result.items[]` with `userName`, native revenue and KRW revenue | `lending top` | current bundle + read-only live schema verified 2026-09-03; distinct from the signed-in account's expected revenue |
| `auth` | `GET` | `wts-cert-api.tossinvest.com` | `/api/v1/user-alimies` | WTS notification preferences | `.result[]` with `id`, `type`, `enabled`, timestamps | `notifications list` | current WTS bundle + read-only live schema verified 2026-09-02; internal `userId` discarded |
| `auth` | `GET` | `wts-cert-api.tossinvest.com` | `/api/v1/inbox-alimies/has-unread` | whether the Securities inbox has unread content | `.result.unread` boolean | `notifications status` | exact WTS call site + value-free live schema verified 2026-09-03 |
| `auth` | `GET` | `wts-cert-api.tossinvest.com` | `/api/v1/reasoning/agreement` | AI reasoning agreement state | `.result` boolean | `notifications status` | exact GET call site + value-free live schema verified 2026-09-03; read only, distinct from POST agreement mutation |
| `auth` | `GET` | `wts-cert-api.tossinvest.com` | `/api/v1/reasoning-news/count` | global AI reasoning-news content count | `.result` number | deprecated JSON/CSV field in `notifications status` | not account notification state; actual value retained while numeric output contract exists, hidden from table |

`notifications status` derives AI issue, FOMC, and reasoning-subscription flags from the canonical
`/user-alimies` response, so their specialized GETs are redundant and are not callable operations or
monitor dependencies. `reasoning-news/count` is a global UI content count rather than an account
preference; its real value remains in the deprecated `reasoning_news_count` JSON/CSV slot while the
numeric output contract exists, and it is not shown in table output.

## Transactions Ledger

Captured via `/my-assets` navigation on 2026-04-19. Covers trades, cash flow, dividends, and stock in/out per market.

| Status | Method | Host | Path | Purpose | Observed shape | CLI mapping | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `auth` | `GET` | `wts-api.tossinvest.com` | `/api/v3/my-assets/transactions/markets/{market}` | paginated transaction ledger | `.result.body[]` with `type`, `transactionType.{code,displayName}`, `stockCode`, `stockName`, `quantity`, `amount`, `adjustedAmount`, `commissionAmount`, `totalTaxAmount`, `balanceAmount`, `date`, `dateTime`, `settlementDate`, `referenceType`, `referenceId`, `compositeKey` | `transactions list` | `market` = `kr` or `us`. Query params: `size`, `filters` (0=all, 1=trades, 2=cash/dividend, 3=stock in-out, 6=alt cash; 4/5/7 return 500), `range.from`, `range.to`. `size` is honored; `range.from` and `number` are silently ignored — Toss returns up to `size` entries within the tail of `range.to`. Items are grouped by `type` ASC (1 = trade records, 2 = cash-flow records), then DESC by `dateTime`/`date` inside each group. US `type=1` trades populate only `settlementDate` (T+2); client range-filter falls back to `compositeKey.orderDate` to match execution day. Client pages older data by re-issuing with `range.to` set to the earliest date seen, dedupes by SortKey (derived from `compositeKey`), and filters items to the caller's `[from, to]` window. Max range = 200 days (client-side guard). |
| `auth` | `GET` | `wts-api.tossinvest.com` | `/api/v3/my-assets/transactions/markets/{market}/overview` | cash overview per market | `.result` with `orderableAmount`, `withdrawableAmount.amount0..3`, `depositAmount.amount0..3`, `estimateSettlementAmount.day1..2`, `withdrawableAmountBottomSheet` | `transactions overview` | `depositAmount` buckets represent upcoming settlement credits; `estimateSettlementAmount` shows buy/sell amounts clearing on each upcoming settlement date. |

## Paper trading — experimental, callable after opt-in

WTS builds contain a US-options paper environment. Static analysis first identified the route
families below; on 2026-09-03 the dedicated paper balance, deposit, prepare/create, single-cancel,
bulk-cancel, pending-order, and completed-order flows were also exercised against an ordinary
brokerage session with no live options/derivatives account:

- enrollment/initialization: `POST /api/v1/paper/init`; the call has no body and follows selection
  of an options account
- simulated balance: `GET /api/v1/paper/cash-balance`, `POST /api/v1/paper/deposit`
- education: lecture and summary reads plus session open/heartbeat/close/complete and a mobile-app
  redirect push
- portfolio/orders: asset sections, pending/completed history, sellable quantity, cost basis, and
  per-order available actions
- cancellation: prepare then execute for one order, plus bulk prepare/execute; exact bodies and
  `X-Order-Key` propagation are recorded in `wts-endpoints.json`

The implementation is intentionally **experimental**, not part of default CLI/MCP discovery.
Enabling `experimental.paper_trading` exposes typed CLI commands and eight ops/MCP operations.
All mutations preview by default and require explicit `simulation_execute`; the client accepts only
dedicated `/paper/` routes and never imports live trading config or a live confirmation token.
Deposit and cancellation executions perform a paper-ledger post-read where the route supports an
unambiguous check.

The rollout is not stable yet. `/api/v1/paper/init` returns an unresolved 500, while deposit,
prepare/create, single cancellation, and bulk cancellation succeed even when the education and
overseas-derivative eligibility flags are false. This proves the current account can use those
paper routes; it does not prove general availability or a permanent upstream contract. Current
build presence, probe history, live observations, and promotion blockers are tracked under
`rolling_features.paper-trading-us-options` in `wts-endpoints.json` and in
[`ADR 0005`](../adr/0005-isolate-paper-execution-and-gate-rollout.md).

## Admission policy

The Go client admits an endpoint only when it is:

- observed in this catalog
- supported by an exact method/host/request/response contract
- assigned an explicit read or mutation policy
- mapped to an approved typed command or operation

Writes additionally require the inventory and guardrails in
[`mutation-inventory.md`](mutation-inventory.md). The following classes stay blocked:

- WTS live-order placement, modification, or cancellation in ops/MCP. The human-oriented top-level
  `order place|cancel|amend` CLI retains its separately captured WTS path, but never retries a write
  across official and WTS backends.
- paper education-session completion and any paper route whose payload or ledger isolation is not
  verified; the admitted experimental subset is restricted to dedicated `/paper/` routes
- account administration, funds transfer, identity, password, legal terms, and regulated applications
- telemetry endpoints
- comment posting or social actions

## Next Catalog Work

1. Capture authenticated account flows with a clean browser session.
2. Record request bodies for `cert-init`, ranking, and signals endpoints.
3. Promote quote-related endpoints into typed Go client methods.
4. Add stable fixture names for every supported endpoint family.
