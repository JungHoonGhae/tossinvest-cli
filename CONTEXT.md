# CONTEXT — tossinvest-cli 도메인 용어

이 프로젝트에서 말이 갈리기 쉬운 용어를 한 곳에 모은다. 코드·PR·이슈에서 아래 낱말은
여기 정의된 뜻으로만 쓴다. 되돌리기 어려운 결정은 `docs/adr/` 에 기록한다.

## 기능 영역과 접근 경로

`증권`과 `WTS`를 같은 축에서 나누지 않는다. 전자는 **무슨 기능인가**이고, 후자는
**어떤 통로로 호출하는가**다. `플랫폼`은 두 뜻을 섞어 버리므로 제품 도메인 이름으로
사용하지 않는다. 결정 배경은
[ADR 0003](docs/adr/0003-separate-product-domain-from-access-channel.md)에 기록한다.

### 기능 영역 (`Operation.Domain`)

| 값 | 뜻 | 현재 상태 |
|---|---|---|
| `securities` | 토스증권 계좌·시세·주문·주식모으기 | 구현됨 |
| `banking` | 일반 토스뱅크/은행 계좌·잔액·거래 | 모바일 계약만 부분 확인, connector 없음 |
| `mydata` | 카드 결제·월간 소비·외부 금융자산 | 모바일 계약만 부분 확인, connector 없음 |
| `system` | 인증·Open API IP·변경 감시 | 구현됨 |

일반 토스 앱 안에는 증권 모바일 화면(MTS 성격)과 Banking/MyData 화면이 함께 있지만,
같은 앱에 있다는 사실이 같은 API·토큰·동의를 뜻하지 않는다. 특히 일반 Banking/MyData에는
대응하는 WTS 웹앱이 없다. 정식 명령 `accumulate funding-status`는
**토스증권 주식모으기 자금연결 상태**이므로 `securities + wts`다. 이전 이름
`banking status`는 호환용 deprecated alias이며 새 문서와 자동화에서는 사용하지 않는다.

### 접근 경로 (`Operation.Backend` / CLI `source`)

**공식 Open API (official)** — 토스증권이 공개한 정식 API. `internal/official`.
`tossctl openapi login` 으로 발급한 자격증명이 필요하다. 계약이 안정적이고 주문
집행의 우선 경로다. ops/MCP 주문과 조건주문은 이 경로만 사용한다.

**WTS** — 토스증권 웹 트레이딩 시스템의 내부 API. `internal/client`.
`tossctl auth login` 으로 얻은 웹 세션 쿠키를 재사용한다. 공식 API 에 없는 조회(인기
순위·수급·AI 시그널·스크리너·업종·어닝·브리핑·배당 등)를 제공한다. 최상위 CLI의 일반
주문(`place/cancel/amend`)도 명시적으로 WTS를 선택하거나 공식 경로를 사용할 수 없을 때
이 경로를 사용할 수 있지만 **비공식이라 예고 없이 바뀔 수 있다**.

**mobile** — 일반 Toss Android/iOS 앱의 내부 API 경로. 증권 모바일 화면과
Banking/MyData 화면을 포함할 수 있지만 각각의 client·interceptor·동의 범위가 다르다.
현재는 정적 감사 대상일 뿐, 인증 connector가 없으므로 operation backend로 노출하지 않는다.
웹 UI가 없다는 사실만으로 `mobile`이라 판정하지 않는다. APK의 base host·client binding·
인증 및 cipher interceptor 소유권으로 접근 경로를 판정한다.

**hybrid routing** — 하나의 조회를 두 백엔드 중 어디로 보낼지 런타임에 정하는 것.
정책은 **backend preference**와 fallback 허용 여부로 구성된다:

**backend preference** — hybrid routing의 시작 경로. canonical 값은 `auto`, `openapi`,
`wts` 세 가지다. `auto`와 `openapi`는 공식 경로를 먼저 시도하고, 실제 WTS 재시도 여부는
fallback 설정이 정한다. `official`은 입력 호환용 deprecated 별칭이며 `openapi`로 정규화된다.

- 읽기: official 을 먼저 시도하고, 폴백 대상 실패(전송·인증·IP·레이트리밋·서버 오류)
  면 WTS 로 재시도한다. 도메인 오류(404 등)는 폴백하지 않고 그대로 돌려준다.
- 쓰기(일반 주문): 최상위 `order` CLI는 시작 시 official 또는 WTS 한 경로를 고르고
  **절대 교차 재시도하지 않는다.** `ops call`과 MCP 주문은 operation 계약대로 official-only다.
  어느 경로든 transport error 뒤 결과는 불명확할 수 있으므로 상태 확인 전 재시도하지 않는다.
- `wts`는 우선순위 힌트가 아니라 **공식 경로 비활성화**다. ops의 직접 공식 호출과 WTS
  우선 조회의 역방향 폴백까지 같은 정책을 따른다.

CLI 와 MCP 는 조회용 typed contract와 hybrid 정책을 공유한다. 다만 호출 표면도 독립된
분류 축이다. 사람용 최상위 `order` CLI만 기존 WTS 일반주문 경로를 유지하고, operation
catalog를 실행하는 `ops call`과 MCP 주문은 `backend=""`인 official-only 계약을 따른다.

## 오퍼레이션

**operation** — 카탈로그에 등록된 하나의 API 동작. `internal/ops` 가 단일 레지스트리이며
`ID`, 호환용 `Aliases`, `Domain`, `Params`, `Backend`, 그리고 typed client 를 호출하는 `handler` 로 이루어진다.
`internal/mcp`(에이전트용 3-tool 카탈로그)와 `internal/monitor`(헬스 probe)가 여기서
파생된다. alias는 이전 자동화 입력만 받아들이며 별도 operation·probe·카운트로 노출하지 않는다.

**backend (오퍼레이션 필드)** — 그 오퍼레이션이 요구하는 자격증명. `Catalog.Call` 이
디스패치 전에 검사한다. 같은 경계에서 `Params` 에 선언된 primitive 타입을 정규화하고,
선언되지 않은 인자·소수 정수·정밀도를 잃는 큰 정수는 handler 전에 거부한다. handler 는
인자 수송 형식(JSON 의 `float64`·`[]any`)이 아니라 정규화된 값만 받는다.

| 값 | 뜻 | 필요한 것 |
|---|---|---|
| `""` (기본) | official 전용 | 공식 자격증명 |
| `"wts"` | WTS 전용 | 웹 세션 |
| `"auto"` | hybrid 라우터가 서빙 | **둘 중 하나면 충분** |
| `"none"` | 인증 불필요 | 없음 (예: `auth_status`) |

`"auto"` 는 official 과 WTS 양쪽에 **시그니처가 동일한** 대응 메서드가 있어 적응 코드
없이 라우터에 얹을 수 있는 오퍼레이션에만 붙인다.

**domain (오퍼레이션 필드)** — 기능의 제품 소유권. `backend`와 독립이다. 예를 들어
`price_alert_add`는 `domain=securities`, `backend=wts`이고 Open API IP 교체는
`domain=system`, `backend=wts`다. 향후 일반 은행 거래를 붙인다면
`domain=banking`, `backend=mobile`이 되며 WTS로 표기하지 않는다.

## 종목 데이터 표면

**quote vs metadata vs universe** — 셋은 서로 대체하지 않는다.

- `quote get`·`quote batch`는 현재가·등락·거래량 같은 **시세**다.
- `quote metadata`는 사용자가 이미 아는 심볼(최대 200개)의 ISIN·시장·증권 유형·상장
  상태·상장주식수·KRX/NXT 상태를 채우는 공식 API **참조 데이터**다. 정확히 대응하는
  WTS 계약이 없으므로 official 전용이며 폴백하지 않는다.
- `market stocks`는 시장 하나의 거래 가능 종목을 나열하는 **유니버스**다. 심볼을
  모를 때 발견하는 상류 기능이고, 개별 종목의 전체 메타데이터를 주는 명령이 아니다.

이 구분 때문에 메타데이터를 `quote batch`의 플래그로 섞거나 `market stocks`의 별칭으로
두지 않는다. 가격과 참조 데이터는 갱신 주기·누락 의미·출력 계약이 다르다.

**probe** — 오퍼레이션에 선택적으로 붙는 모니터링 명세. typed client 를 **일부러
우회**해서 raw method/URL/body 로 찌른다. 클라이언트 코드가 서버 변경과 함께 움직여도
계약 변화를 잡아내기 위함이다. 선별적으로만 단다(CLI 표면당 대표 엔드포인트 하나).

**write operation** — 상태를 바꾸는 동작. 일반·조건주문의 생성·취소·정정은 공용
`trading.Service`에서 config 옵트인 + execute·confirm 토큰으로 이중 게이팅되며 official
전용 브로커로만 라우팅된다. 되돌릴 수 있는 증권 설정(목표가 알림, 보유종목 숨김)은 별도
도메인 서비스가 preview + 현재 상태에 결합된 confirm 토큰 + 실행 후 재조회 검증을 맡는다.
typed 명령의 `mutating: true` 에이전트 금지 표시는 실계좌 거래 주문에만 유지하고,
비거래 변경은 `writes_state: true`로 구분한다. 관심종목 폴더·종목은 별도 서비스가
세션 결합 5분 token, 불가역 삭제 추가 승인, 사후 재조회 검증을 맡는다. 여러 operation을 디스패치하는 `ops call`은
둘 다 실행할 수 있으므로 `mutating: true`, `writes_state: possible`로 보수적으로 표시한다.

## 변경 승인

**상태 확인 (state confirmation)** — 미리보기의 정확한 intent와 현재 서버 상태를 함께
결합하는 확인. 설정 변경의 기본 방식이며 적용 뒤 상태가 달라지면 이전 token이 무효가 된다.
이는 프로세스 간 single-use 보장은 아니다. 서버 idempotency나 조건부 쓰기가 확인되지 않은
WTS 변경은 같은 preview를 동시에 실행하지 않는다. 관심종목 token은 추가로 WTS 세션에
묶이고 5분 뒤 만료된다. machine value는 `state_confirmation`이다.

**의도 확인 (intent confirmation)** — 정확한 주문 intent만 결합하는 실수 방지 확인.
현재 주문 token은 결정적이고 만료·single-use가 아니므로 같은 token을 실행하면 같은 주문을
다시 제출할 수 있다. machine value는 `intent_confirmation`이며
`requires_fresh_confirmation=false`로 이 차이를 공개한다.
_피할 말_: `permission`, `one-shot`, `confirm`만 단독으로 사용

**자동화 위임 (automation mandate)** — 사용자가 미리 정한 대상·행동·금액 한도·유효기간
안에서 반복 변경을 허용하는 제한된 위임. 전역 우회 권한이 아니며 범위 밖 요청에는 효력이 없다.
_피할 말_: bypass, skip permissions, unlimited automation

**서버 동의 (server consent)** — 주문·신청 도중 토스가 별도로 요구하는 환전·상품·약관 등의
확인. 자동 수락은 자동화 위임에 이름이 명시된 동의 종류에만 적용한다.
_피할 말_: confirmation (상태/의도 확인과 구분되지 않음)

**킬스위치 (kill switch)** — 이미 부여된 자동화 위임의 실행을 즉시 멈추는 상태. 위임을
새로 만들거나 범위를 넓히는 승인이 아니다.

**모의투자 (paper trading)** — 실계좌 원장과 분리된 것으로 검증된 가상 잔고·주문 영역.
경로 이름에 `paper`가 있다는 사실만으로 분리가 증명되지는 않는다.
_피할 말_: test account (테스트용 실계좌와 혼동됨)

## 인증 상태

**auth snapshot** (`AuthStatus`) — 백엔드별 연결 여부와 만료 시각. **비밀값을 담지
않는다** — 불리언과 타임스탬프뿐이라 에이전트에게 그대로 돌려줘도 안전하다.

hybrid 라우터는 세션이 없어도 구성되므로(임베드된 WTS 클라이언트가 non-nil 이어야 함),
**"웹 세션이 있는가" 의 판정 기준은 포인터의 nil 여부가 아니라 이 스냅샷이다.**
`Catalog.Call` 의 게이팅이 이 값을 본다.

## 실현손익 (realized profit)

**cumulative vs period-scoped** — 두 가지를 구분한다. `profit`(=`profit/overview`)은
**누적 전체**로 모든 카테고리를 한 번에 준다. `profit summary`(=`profit/type/overview`)는
**기간 지정**으로 카테고리 하나를 준다. 같은 낱말을 쓰지만 축이 다르므로 섞어 쓰지 않는다.

**profitType** — 실현손익 카테고리. `sales`(매매손익) · `dividend`(배당) ·
`lending`(주식대여) · `account-interest`(예탁금이자). 서버는 이 외의 값에 400 을 준다.
로컬에서 검증하므로 토스가 5번째를 추가하면 우리가 먼저 거부한다 — 주간 모니터는
카탈로그 변화는 잡지만 **enum 값 변화는 못 잡는다**는 점을 감수한 선택이다.

**rangeType** — 실질적으로 **2상태 플래그**다. `all` 이면 날짜를 무시하고 전체 기간을,
그 외 값이면 `startDate`~`endDate` 를 쓴다. 라이브 측정: 동일 날짜에서
`day`/`week`/`month`/`year` 의 응답이 **바이트 단위로 같고** `all` 만 다르다.
사용자에게 노출하지 않는다 — 의미 없는 축인데다 **인식 못 하는 값에 서버가 500 을
반환**하기 때문이다. 날짜 유무로 우리가 결정한다.

**rate basis (수익률 기준 통화)** — `profit daily` 의 `currency` 는 **필터가 아니다.**
`KRW`/`USD` 어느 쪽이든 **같은 행 집합**이 오고 `profitRate` 만 달라진다. 해외 종목의
원화 수익률에는 환율 변동이 섞이고 달러 기준에는 섞이지 않기 때문이다. 따라서 두 통화를
합쳐 조회하면 **모든 행이 중복된다** — 호출은 한 번만 한다.

**날짜 표기** — 요청은 `YYYYMMDD`. 응답의 `baseDate` 는 이를 되돌려주지 않고 표시용
`YY.M.D`(월·일 패딩 없음)로 온다. `formatBaseDate` 가 `YYYY-MM-DD` 로 정규화한다.
**미래 endDate 는 400** 이므로 로컬에서 먼저 막는다.

**호스트** — profit 계열은 전부 `wts-cert-api` 다(`CertBaseURL`). 같은 화면의
`dashboard/intelligences/all` 은 `wts-info-api` 로, 화면 하나가 여러 호스트를 섞어 쓴다.
새 엔드포인트를 붙일 때 **경로만 보고 호스트를 짐작하지 말 것.**

## 뉴스 (market news)

**briefing vs news** — 둘 다 뉴스지만 다른 것이다. `market briefing`
(`ai-signals/personalized`)은 **AI 가 테마별로 묶어 준 요약**이고, `market news`
(`dashboard/wts/news`)는 **원문 목록 + 종목 연결**이다. 후자만 각 기사의 관련 종목과
현재 등락률을 준다.

**news scope** — 뉴스 범위. 서버 enum 이 CLI 어휘로 나쁘고 하나는 오해를 부른다:
`HOT` 은 급상승이 아니라 **"최신 뉴스"** 이고, 급상승은 `SOARING_STOCK` 이다.
그래서 별칭을 둔다 (`communityRankingTypes` 선례와 동일, raw enum 통과 허용):

| 별칭 | 서버 enum | 서버가 주는 title |
|---|---|---|
| `all`(기본) | `ALL_HIGHLIGHT` | 모든 주요 뉴스 |
| `recommended` | `PERSONALIZED` | 추천 뉴스 |
| `watchlist` | `PERSONALIZE_WATCH` | 관심 뉴스 |
| `holdings` | `PERSONALIZE_HOLD` | 보유 뉴스 |
| `latest` | `HOT` | 최신 뉴스 |
| `soaring` | `SOARING_STOCK` | 급상승 주식 뉴스 |

**관련 종목은 범위에 따라 붙는다** — `all` 은 일반 시장 뉴스라 종목 연결이 없고,
`watchlist`·`soaring`·`recommended` 처럼 종목 기준 범위에만 붙는다. 비어 있다고
버그가 아니다.

**서버가 title 을 준다** — 범위 라벨("모든 주요 뉴스")을 응답에 담아 주므로 한글
라벨을 코드에 박지 않는다. 토스가 이름을 바꾸면 자동으로 따라간다.

**페이지네이션·검색 없음** — `size` 는 먹지만 **50 이 상한**이고 `page`·
`pagingParam`·`after` 는 무시된다(첫 항목이 그대로다). `query`/`keyword`/`q`/
`searchWord` 도 전부 무시된다. 즉 **최신 ≤50건만** 닿을 수 있다.

## 계좌관리 (account management)

웹의 계좌관리 화면(`/account/settings`)은 **조회와 변경이 섞여 있다.** CLI 는 조회만
가져간다:

- **조회 (구현됨, `account detail`)** — 계좌 신원(`account/detail`), 출금 가능액·한도
  (`transfer/withdrawable-status`), 미수거래 상태(`dashboard/wts/overview/margin`,
  `margin/cert/differential-margin/enabled`), 송금한도 제한 여부.
- **변경 (의도적 제외)** — 계좌 해지, 계좌 비밀번호 변경, 달러 가져오기/보내기.
  자금 이동과 계좌 해지는 주문보다 되돌리기 어렵다. 거래를 config 옵트인 +
  confirm 토큰으로 막아 둔 것보다 더 강한 근거가 있기 전에는 노출하지 않는다.

**마스킹** — `account detail` 은 계좌번호와 **예금주명**(API 의 `accountName` 은 실명이다)
을 기본으로 가린다. `--full` 이 명시적 해제다. CLI 출력은 이슈·채팅에 붙여넣어지므로
"공개 출력에 실계좌 데이터 금지" 원칙이 화면에도 적용된다.

**부분 실패** — 이 화면은 엔드포인트 다섯 개를 합친 것이라, 신원 조회만 필수로 두고
나머지는 실패해도 경고만 남기고 진행한다. 조용히 삼키지는 않는다.

**상태 코드** — `account/detail` 의 `status` 는 의미를 모르는 코드다(관측값 `"00"`).
사람이 읽는 출력에는 넣지 않고 JSON 에만 남긴다.
