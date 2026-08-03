# Toss Securities Web Capture Workflow

Verified against the public web surface on 2026-03-11.

## Goal

Capture Toss Securities web traffic in a way that is:

- repeatable
- safe to store
- useful for a read-only CLI
- explicit about what is still unknown

This workflow is for reverse engineering the web product, not for bypassing login or automating trading.

## Scope

Priority screens for Milestone 1:

1. `/account`
2. a stock detail page such as `/stocks/A005930/order`
3. watchlist and holdings views after manual login

The first pass should focus on read-only data only:

- account summary
- positions
- orders history
- watchlist
- quotes

## Safe Workflow

### 1. Start with public pages

Use public pages to identify:

- API hostnames
- bootstrap endpoints
- stock-detail endpoints
- chart and ranking endpoints
- login page redirects

This work can happen before any authenticated capture.

### 2. Use a clean browser profile

When capturing authenticated traffic:

- use a fresh browser profile
- log in only with a test or personal account you control
- avoid keeping other tabs open
- avoid mixing mobile and web login flows in the same capture

### 3. Capture one screen at a time

For each screen:

- open the screen
- wait for data to settle
- export a HAR
- record the screen name and date
- describe the user-visible action that triggered the requests

Good capture units:

- account overview page load
- positions tab load
- orders history tab load
- watchlist page load
- symbol detail page load

### 4. Sanitize before commit

Never commit raw captures.

Store raw files outside git tracking, then sanitize them into `fixtures/har/` or convert them into smaller JSON fixtures under `fixtures/responses/`.

Use:

```bash
python3 tools/sanitize_har.py path/to/raw.har fixtures/har/account-overview.har
```

### 5. Build the RPC catalog first

Before writing Go client code:

- list the endpoint
- classify it as public, guest, or authenticated
- capture the request method
- note required params and headers
- map it to a CLI capability

## What to Record Per Endpoint

- hostname
- method
- path
- query parameters
- auth requirement
- request body shape if present
- response top-level shape
- whether the endpoint appears safe for read-only use
- which CLI command depends on it

## Redaction Rules

Always remove or replace:

- `cookie`
- `set-cookie`
- `authorization`
- `x-csrf-token`
- `x-xsrf-token`
- `x-device-id`
- `x-session-id`
- `x-request-id`
- `phoneNumber`
- `name`
- `residentRegistrationNumber`
- account numbers
- order IDs that can be tied to a real user
- comments or community text tied to a logged-in identity

Masking rule:

- preserve structure
- replace secrets with stable placeholders such as `<REDACTED_COOKIE>`
- do not delete entire objects unless there is no safe way to keep their shape

## Public Observations Captured on 2026-03-11

Public web navigation exposed these routes:

- `/`
- `/feed/recommended`
- `/screener`
- `/account`
- `/signin?redirectUrl=%2Faccount`

Visiting `/account` without an authenticated session redirected to `/signin?redirectUrl=%2Faccount`.

Observed API hostnames:

- `wts-api.tossinvest.com`
- `wts-info-api.tossinvest.com`
- `wts-cert-api.tossinvest.com`
- `cdn-api.tossinvest.com`
- `tuba-static.tossinvest.com`
- `log.tossinvest.com`

Observed public or guest-accessible endpoints included:

- `GET /api/v3/init`
- `GET /api/v1/time`
- `GET /api/v1/user-setting`
- `GET /api/v2/system/trading-hours/integrated`
- `GET /api/v1/dashboard/wts/overview/trading-info`
- `GET /api/v1/dashboard/wts/overview/exchange-rates`
- `GET /api/v1/rankings/realtime/stock`
- `GET /api/v2/stock-infos/{code}`
- `GET /api/v1/stock-detail/ui/{code}/common`
- `GET /api/v1/c-chart/...`
- `GET /api/v1/product/stock-prices`

These observations are enough to start a read-only catalog. Authenticated captures are still needed for account, holdings, and order history.

## Next Milestone 1 Outputs

- `rpc-catalog.md`
- `auth-notes.md`
- first sanitized HAR captures
- small JSON fixtures for stock detail and quotes

---

## 신규 기능 발굴 실전 플레이북 (2026-07 정립)

accumulate·profit·tax 기능을 붙이며 정립한, **웹 세션으로 호출 가능한 기능을
발굴→검증→구현**하는 반복 절차. `/browse` 스킬(헤드리스 Chromium)만으로 대부분 된다.

### 0. 세션 주입 (매 세션 시작)

`tossctl` 이 저장한 세션 쿠키를 브라우저에 심는다. `session.json` 의 `cookies` 는
평면 dict(`{name: value}`)라, 도메인·httpOnly·secure 를 채워 `cookie-import` 로 넣어야
`.tossinvest.com` 하위 API 가 인증된다 (정확 호스트 `www.` 로만 넣으면 `.tossinvest.com`
스코프 쿠키가 안 붙어 401).

```
session.json cookies → [{name,value,domain:'.tossinvest.com',path:'/',
  httpOnly: name in {BTK,FTK,LTK,SESSION,UTK}, secure:true, sameSite:'Lax'}]
→ $B cookie-import  → $B goto /account (리다이렉트 없으면 인증 성공)
```

### 1. 후보 발굴: 카탈로그가 아니라 JS 번들에서

`wts-endpoints.json` 의 candidate 경로는 `{id}` 로 정규화돼 있어 **그대로 호출하면
404 난다.** 정확한 경로·메서드는 프로덕션 번들에서 뽑는다:

```
GET / → buildId 추출 → _buildManifest.js → 청크 URL 수집 → 전부 fetch 후 concat
번들에서: path:"/api/vN/..." + 근처 method:"GET|POST" 정규식으로 정확한 정의 추출
```

주의: minified 변수명(eP, o, c…)은 청크마다 재사용돼 **호출부(요청 바디) 정적 추적은
신뢰도 낮다.** 경로·메서드까지만 번들로 얻고, 바디는 라이브로 확인.

### 2. 라이브 프로브 (Promise.all + 타임아웃)

`$B eval` 은 top-level await 결과를 문자열로 못 뽑으니, `window.__x` 전역에 담고
폴링하거나 `(async()=>{...window.__r=...})()` 로 감싼다. 순차 18개는 hang 위험 →
`Promise.all` + `Promise.race(fetch, timeout(8000))`.

- **404** = 경로 추측 틀림 (번들에서 정확 경로 재확인)
- **400 MissingServletRequestParameter** = 파라미터만 넣으면 됨 (거의 되는 것)
- **200 + 빈약한 데이터** = 탈락 후보 (margin=전부 false, news/count=숫자 하나 등)

### 3. POST 바디 알아내기

profit 계열처럼 POST 이고 파라미터가 필요하면:

1. **메타 엔드포인트 먼저** — `readable-tab` 처럼 "어떤 값이 가능한지" 주는 것을 찾는다.
2. **GA 로그 URL이 힌트** — `$B network` 에 찍힌 google-analytics `collect?...&dl=`
   URL에 페이지 쿼리스트링(`?productType=us&profitType=sales`)이 들어 있어 **필드명
   단서**가 된다 (단, 쿼리 파라미터명 ≠ POST 바디 필드명일 수 있음 — 확인 필요).
3. **빈 바디 `{}` 부터** — overview 류는 빈 바디로 전체를 주는 경우가 많다.
4. 안 되면 **실제 웹 요청 바디를 잡는다** — `node tools/capture_post_bodies.mjs <경로>`.
   아래 "첫 로드 POST 바디 캡처" 참고. (예전엔 여기가 막혀 있었다.)

### 4. 웹 UI 유무 판정 = "모바일 전용" 분류의 유일한 기준

**카탈로그 candidate ≠ 모바일 전용.** 반드시 라이브로 웹 라우트를 열어 판정:

```
$B goto /account/<feature> → 리다이렉트 없이 실제 화면이 뜨고 관련 텍스트가 보이면
→ 웹 UI 있음 (일반 조회) / 뜨면 안 뜨거나 signin 리다이렉트면 → 웹 UI 없음 (모바일 전용)
```

실측: accumulate=웹UI 없음(진짜 모바일 전용), profit·transfer-income=웹UI 있음.
README 의 "📱 모바일 앱 전용" 서브섹션엔 **웹UI 없는 것만.** 토스가 UI 추가하면 일반
표로 옮긴다 (판정을 주간 모니터가 추적하도록 하는 게 이상적 — TODO).

### 5. 구현 = 기존 RE 흐름 그대로

domain → client(`getJSON`/`postJSON`, 페이징은 aggregate) → output(테이블/JSON/CSV)
→ cmd → `internal/ops` 등록(+probe) → 계약테스트(httptest, **더미 데이터**) →
`wts_endpoints.py` IMPLEMENTED 패턴 + 카탈로그 재생성 → README(+🆕)/CHANGELOG.
**실계좌로 라이브 검증하되 그 값은 커밋 금지** (테스트는 합성 더미로).

### 캡처 3단 구성 (2026-08-03 정립)

발굴은 **세 도구가 서로 다른 층**을 덮는다. 하나만 쓰면 그 층만큼만 보인다.

| 도구 | 무엇을 알아내나 | 세션 |
|---|---|---|
| `tools/wts_endpoints.py` | **어떤 경로가 존재하나** — 번들+라우트에서 경로 수집 | 불필요 |
| `tools/probe_candidates.py` | **살아있나** — GET/POST 로 찔러 200/400/403/404 등급화 | 필요 |
| `tools/capture_post_bodies.mjs --sweep` | **어떻게 부르나** — 실제 요청의 파라미터 키와 **호스트** | 필요 |

```bash
python3 tools/wts_endpoints.py                          # 경로 수집·분류
python3 tools/probe_candidates.py                       # 살아있는지 등급화
node tools/capture_post_bodies.mjs --sweep --get        # 파라미터·호스트 관측
```

세 결과는 카탈로그의 `status` / `probe` / `observed` 로 각각 남고, **재생성해도 보존된다**
(`probe`·`observed` 는 추출기가 다시 만들 수 없는 관측값이다 — 보존을 빼먹으면 주간
모니터가 매주 월요일 지운다).

#### `observed` 가 호스트 문제를 없앤다

토스는 `wts-api`/`wts-info-api`/`wts-cert-api` 를 섞어 쓰고 경로만 보고는 알 수 없다.
스윕이 실제 요청에서 본 호스트를 기록하므로, 구현 전에 카탈로그만 보면 된다:

```bash
jq -r '.endpoints["/api/v3/stock-prices"].observed' docs/reverse-engineering/wts-endpoints.json
# {"method":"GET","host":"wts-info-api","query":["meta","productCodes"], ...}
```

#### 스윕이 못 덮는 것

페이지 **로드 시점**의 요청만 잡는다. 탭 클릭·스크롤로 발생하는 요청은 안 잡히므로,
`needs-params` 가 전부 해소되지는 않는다(첫 실행: 154개 중 7개). 나머지는 해당 화면을
직접 열어 `capture_post_bodies.mjs <path>` 를 단건으로 돌리는 편이 빠르다.

**값은 어떤 모드에서도 카탈로그에 들어가지 않는다** — 키 이름과 호스트만 기록한다.

### 놓치기 쉬운 방식 (2026-08-03 에 넷 다 밟았다)

증시 캘린더를 발굴하며 **같은 날 네 번 틀렸다.** 각각이 별개의 방법론 결함이라 적어둔다.

#### 1. 번들 수집이 초기·공유 청크만 본다

`/` 와 `_buildManifest.js` 만 보면 **지연 로딩되는 페이지 전용 청크가 통째로 안 잡힌다.**
`/calendar` 화면의 API(`/api/v4/calendar/monthly/{month}`, `/api/v1/nova-calendar/...`)가
카탈로그 949개 어디에도 없었다. `/calendar` HTML 을 받아보니 루트에 없는 청크가 10개 더 있었다.

**고침**: `wts_endpoints.py` 가 번들에서 라우트를 뽑아 각 라우트 HTML 의 `<script>` 를 걷는다.
브라우저 없이 순수 HTTP 라 CI 에서 돈다. 949 → 966개, 청크 26 → 74개.

**라우트 목록을 손으로 적지 말 것.** 처음에 9개를 적었는데 실제로는 43개를 더 놓치고 있었다.

#### 2. GET 만 찔러보면 POST 조회를 사장시킨다

토스는 **조회에도 POST 를 자주 쓴다** (`profit/overview`, `dashboard/wts/news`,
`calendar/monthly`). GET 만 보내는 스윕에서 34개가 `405` 로 묻혔고, POST `{}` 로 재시도하니
**29개가 살아있는 엔드포인트**였다. 그중 하나가 월간 캘린더다.

**고침**: `probe_candidates.py` 가 405 를 받으면 빈 바디 POST 로 한 번 재시도한다.
쓰기 유발을 막으려고 **405 를 받은 경로에만, 빈 바디로만** 보낸다(쓰기는 보통 GET 에
401/403/400 을 준다).

#### 3. 파라미터는 쿼리스트링만 있는 게 아니다

`from`/`to`/`date`/`month`/`size` 를 찍어보고 전부 무시되길래 "기간 파라미터가 없다" 고
문서·CHANGELOG·코드 주석에 박았다. 실제로는 **경로 세그먼트**였다 — `/monthly/2026-09`.

**고침**: "파라미터 없음" 이라고 쓰기 전에 카탈로그에서 **형제 경로에 자리표시자가 있는지**
본다(`/api/v4/calendar/monthly` 는 정규화된 형태다). 쿼리만 보고 단정하지 않는다.

#### 4. 200 이 나왔다고 그게 그 기능의 정식 API 인 건 아니다

`overview/calendar/economic-events` 가 200 을 주길래 바로 구현했다. 그건 **대시보드 위젯용
티저**로, 고정 10일치에 실적도 예상치도 없고 웹은 그중 3건만 잘라 쓴다. 진짜 기능은
`/calendar` 페이지에 있었고 월 단위에 실적·예상치까지 준다.

**고침**: 구현 전에 **그 기능의 전용 화면이 무엇을 부르는지** 확인한다. 대시보드 위젯이
부르는 것과 전용 페이지가 부르는 것은 다르다. 번들에서 `href:"/..."` 로 링크된 화면을 찾아
그쪽 네트워크를 본다.

### 스크리너 필터 어휘 — 번들 스크랩은 실패했다 (2026-08-04)

README 는 필터 어휘(`배당_수익률` 같은 한글 id)가 "토스 번들에만 있어 공개돼 있지
않다" 고 적어뒀다. 번들에서 뽑아 카탈로그로 내보내려 했고 **실패했다. 재시도하지 말 것.**

필터는 번들에 `new C({id:VAR, nation:[...], name, description, conditionMap})` 로
선언돼 있고 한글 id 는 변수에 담겨 있다. 변수를 해석해 뽑아봤다:

| 시도 | 프리셋 실제 id 와 일치 |
|---|---|
| 한글 문자열 변수만 매핑 | 14/17 (82%) — 미해결 변수 6개 |
| ASCII 식별자까지 매핑 확대 | **10/17 (58%)** — 변수는 풀렸는데 **틀린 문자열**에 붙음 |

넓힐수록 나빠진 이유는 이 문서가 위에서 이미 경고한 것이다 — **minified 변수명은
청크마다 재사용된다.** 같은 `ea` 가 다른 청크에서 다른 문자열이라, 정적 해석은
구조적으로 신뢰할 수 없다.

**신뢰할 수 있는 출처는 프리셋이다.** `market screener --output json` 이 각 프리셋의
`filters` 배열을 그대로 내보낸다(v0.29.0). 거기 든 id 는 서버가 준 것이라 확실하다.

#### 대신 확실히 알아낸 호출 시그니처

번들의 **호출부**(변수가 아니라 리터럴 경로)는 신뢰할 수 있다. 호스트는 전부 CERT:

```
POST /api/v1/screener/filters/base   {filterId, nation}   → basedAt
POST /api/v1/screener/filters/range  {filterId, nation}   (400 — 바디 미확정)
POST /api/v1/screener/screen/count   []                   (400 — 바디 미확정)
GET  /api/v1/screener/presets/user?useCustom=true         → 사용자 저장 프리셋
GET  /api/v2/screener/presets/common?useCustom=true       (구현됨)
POST /api/v2/screener/screen                              (구현됨)
```

`filters/base` 는 `{filterId:"배당_수익률", nation:"kr"}` 로 보내면 파싱을 통과해
도메인 에러(`screener.bad-request.based-at`)가 난다 — **바디 형식은 맞다.**

`presets/user` 는 이 계정에서 빈 배열이라 항목 구조를 못 봤다. 저장된 프리셋이 생기면
그때 구현할 것.

### 막힌 방법 (다음에 시간 낭비 말 것)

- **안드로이드 앱 트래픽 캡처**: 갤럭시(루팅X)에 mitmproxy 인증서까지 설치 성공해도,
  **토스 앱은 인증서 핀닝**으로 통신 거부 (삼성 인터넷은 됨 = 프록시는 정상, 앱만 막힘).
  루팅 없이는 APK 재패키징이 유일한데 Play Integrity 로 로그인 거부됨. **불가.**
- **iOS 앱 캡처**: 인증서 신뢰가 안드로이드보다 쉽고 핀닝도 앱마다 달라 **성공 가능성
  있음** — 필요하면 iOS 기기로 시도.
- ~~**`/browse` addInitScript 로 SPA 첫 요청 바디 잡기**~~ / ~~**React Query 캐시**~~
  → **해결됨 (2026-07-25).** 아래 "첫 로드 POST 바디 캡처" 참고.

  기록용 경위: `Page.addScriptToEvaluateOnNewDocument` 를 CDP allowlist 에 추가했으나
  (로컬 빌드) browse 의 goto 가 세션을 재생성해 심은 스크립트가 안 따라갔고, React Query
  캐시 탓에 탭 클릭·pushState 로는 재요청도 안 걸렸다. **두 막힘의 원인은 하나였다 —
  브라우저를 우리가 소유하지 않았다는 것.** 컨텍스트를 직접 만들면 JS 주입 자체가
  불필요하고(Network 도메인이 postData 를 그대로 준다), 신선한 프로필이라 캐시도 없다.


### 첫 로드 POST 바디 캡처 (`tools/capture_post_bodies.mjs`)

웹 UI 에만 있는 기능의 POST 바디는 라이브 요청을 봐야 안다. 브라우저 컨텍스트를 직접
소유하고 **내비게이션 전에** `Network.enable` 을 켜면 첫 로드 요청이 통째로 잡힌다.
JS 주입(addInitScript)도, fetch 몽키패치도 필요 없다.

```bash
tossctl auth status          # Live Check: valid 여야 함. 아니면 tossctl auth login
node tools/capture_post_bodies.mjs /account/profit
node tools/capture_post_bodies.mjs /account/profit --wait 10   # 느린 화면
node tools/capture_post_bodies.mjs /account/profit --all       # 텔레메트리까지
node tools/capture_post_bodies.mjs /feed/news --get            # GET 도 (조회 발굴)
```

기본은 non-GET 만 잡는다(POST 바디가 원래 목적). **조회 기능을 발굴할 땐 `--get`** 을
써야 한다 — 뉴스·랭킹처럼 GET 으로만 오는 표면이 많고, 그런 건 기본 출력에 안 나온다.
실제로 `/feed/news` 의 `dashboard/wts/news` 는 `--get` 을 붙이기 전까지 보이지 않았다.

출력은 **값이 마스킹된 형태**다 — 구현에 필요한 건 키와 타입이지 실계좌 값이 아니다.
`/account/profit` 실측(2026-07-25), 텔레메트리 제외 14건 중 일부:

```
── POST https://wts-cert-api.tossinvest.com/api/v3/profit/readable-tab
{ "rangeType": "<string>", "startDate": "<string>", "endDate": "<string>" }

── POST https://wts-cert-api.tossinvest.com/api/v1/profit/wts/daily/market
{ "currency": "<string>", "startDate": "<string>", "endDate": "<string>",
  "page": "<number>", "size": "<number>" }

── POST https://wts-info-api.tossinvest.com/api/v1/dashboard/intelligences/all  (×2)
{ "types": ["<string>"], "variable": { "isBrowserPushEnabled": "<boolean>" } }
```

**호스트를 그대로 보여준다.** 토스는 `wts-api` / `wts-info-api` / `wts-cert-api` 를
섞어 쓰고 `client.Config` 도 셋을 따로 받으므로, 경로만 보면 어느 BaseURL 에 붙일지
알 수 없다. 위 예시에서도 profit 은 cert, intelligences 는 info 다.

`log/bulk`·`perf-log`·`tuba/*`·`wts-login-device` 는 텔레메트리라 기본 제외된다
(출력의 절반을 차지한다). 필요하면 `--all`.

원본이 꼭 필요하면 `--raw` 를 쓰되 **그 출력은 커밋·PR·이슈에 남기지 말 것**
(CLAUDE.md "공개 출력·테스트·문서에 실제 계좌 데이터 금지").

동작 요약 — 스크립트가 하는 일:

1. Playwright 캐시의 **Chrome for Testing** 을 임시 프로필로 기동
   (사용자의 실제 Chrome 프로필/기본 브라우저를 건드리지 않는다)
2. `Network.enable` → `Network.setCookies`(session.json 의 쿠키) → `Page.navigate` 순서
3. `Network.requestWillBeSent` 에서 non-GET `/api/` 요청의 `postData` 수집
4. 종료 시 브라우저·임시 프로필 정리

**아무것도 안 잡히면**: 세션 만료(가장 흔함), 해당 라우트에 웹 UI 가 없음(= 모바일 전용),
또는 화면이 느려서 `--wait` 을 늘려야 하는 경우다.

전제: Node 18+ (내장 `WebSocket` 사용, npm 설치 불필요) 와 Playwright 브라우저 캐시.
