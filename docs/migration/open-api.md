# Toss Securities Open API 마이그레이션 계획

토스증권이 공식 Open API 사전 신청을 시작했습니다 (광고 심의 시작일 2026-05-14).
이 문서는 `tossctl` 이 공식 API 출시 흐름에 맞춰 어떻게 진화할지 정리한 living
document 입니다. issue [#31](https://github.com/JungHoonGhae/tossinvest-cli/issues/31)
이 트래킹 anchor.

> **status:** Phase 0.5 (공식 spec + 문서 공개, 토큰 발급 절차 미검증) · 마지막 업데이트 2026-06-02
>
> **⚠ 본 문서의 계획·timeline·phase 정의·우선순위는 사전 공지 없이 바뀔 수 있습니다.**
> 토스 공식 표면이 실제로 드러나는 시점, 자원/시간 사정, 새로운 정보에 따라 유연하게
> 재조정합니다. 여기 적힌 내용은 commitment 가 아니라 "현 시점 stance" 입니다.

## 토스 Open API 의 윤곽 (공식 문서 기준)

2026-06-02 공식 개발자 문서 + OpenAPI spec 이 공개됐습니다 (제보: @skyisle, issue #31).
아래는 추정이 아니라 **공식 spec 에서 확인된 사실**입니다.

- **개발자 문서:** https://developers.tossinvest.com/docs (+ AI agent 용 `/llms.txt`)
- **OpenAPI spec (source of truth):** `https://openapi.tossinvest.com/openapi-docs/latest/openapi.json` (v1.0.3, 20 endpoints)
- **Base URL:** `https://openapi.tossinvest.com`
- **인증:** OAuth 2.0 **Client Credentials Grant** — `POST /oauth2/token` 에 `client_id` + `client_secret` (form-urlencoded) → `access_token` (Bearer, `expires_in` 86400, refresh token 없음, client 당 유효 토큰 1개). 계좌·자산·주문 API 는 `Authorization: Bearer` 외에 `X-Tossinvest-Account` 헤더 필수.
- **프로토콜:** REST only (마케팅 페이지의 WebSocket 언급은 아직 spec 에 없음)
- **endpoint 표면 (tossctl 명령과의 매핑):**

  | tossctl | 공식 endpoint |
  |---|---|
  | `account list` | `GET /api/v1/accounts` |
  | `portfolio positions` | `GET /api/v1/holdings` |
  | `quote get` | `GET /api/v1/prices` · `/orderbook` · `/trades` |
  | `quote chart` | `GET /api/v1/candles` (1분봉·일봉) |
  | `order place` | `POST /api/v1/orders` |
  | `order amend` | `POST /api/v1/orders/{orderId}/modify` |
  | `order cancel` | `POST /api/v1/orders/{orderId}/cancel` |
  | `orders list` / `order show` | `GET /api/v1/orders` · `/orders/{orderId}` |
  | (신규) | `buying-power` · `sellable-quantity` · `commissions` · `exchange-rate` · `market-calendar/{KR,US}` · `stocks` · `price-limits` |

- **미검증:** client_id/secret 발급 콘솔 절차, 거래 권한 scope 모델, candle 이 1분봉·일봉만이라 기존 tossctl 의 3/5/15/30/60분봉과 차이 있음 (rate limit 은 [실측 완료 ↓](#rate-limit-실측))

## 포지셔닝 — 공식이 나와도 tossctl 은 갈아타지 않는다

**핵심 교정 (2026-06-03):** 초기 계획은 "공식 나오면 tossctl 이 공식으로 이주,
session 은 deprecate" 였다. 이는 *"공식 = 상위호환"* 이라는 잘못된 가정에 기반했다.

공식 API 는 상위호환이 아니라 **다른 trade-off** 다:

| | tossctl (비공식 web session) | 토스 공식 API |
|---|---|---|
| 데이터 표면 | **넓음** — transactions ledger·watchlist·push(SSE) 포함 | 좁음 — 시세·계좌·주문만 (ledger/watchlist/push **없음**) |
| rate limit | 사실상 없음 | 빡빡 — 계좌·보유는 연속 1회 후 대부분 429 ([실측 ↓](#rate-limit-실측)) |
| 진입 | `auth login` 1회 | 사전신청→승인→토큰 24h 재발급 |
| 합법성·안정성 | 약함 (TOS 리스크, 예고 없는 변경) | 강함 (계약된 versioned spec) |

→ tossctl 이 공식으로 *통째* 이주하면 **고유 강점(넓은 표면 + 무제한)을 스스로 버린다.**
그래서 **이주·deprecation 하지 않는다.** web session 이 영구 주력.

## rate limit 실측

> 측정 방법: 2026-06-30 · 서로 다른 3개 시점 · 조회별로 15~20번 쉬지 않고 연속 호출 · 화면 출력은 버려 계좌 정보 미노출, "우회 발생" 로그로만 막힘(429) 여부 판정.

공식 spec 은 한도를 공개하지 않는다. 그래서 같은 조회를 빠르게 연달아 부르며 **"몇 번째 호출부터
막히는지"** 를 서로 다른 3개 시점에서 직접 셌다. (`--backend openapi` 로 공식 경로 강제)

| 무엇을 조회 | 몇 번째부터 막히나 | 빠르게 반복하면 | 한 줄 요약 |
|---|---|---|---|
| 시세 (`quote`) | 막히지 않음 | 거의 다 성공 (40번 중 39번) | 초당 10번씩 불러도 멀쩡 |
| 내 계좌 (`account`) | **2번째 호출부터** | 10번 중 8번 막힘 (45번 중 8번만 성공) | 한 번 부르고 몇 초 쉬어야 또 됨 |
| 보유종목 (`holdings`) | **사실상 첫 호출부터** | 10번 중 8번 막힘 (45번 중 7번만 성공) | 셋 중 가장 빡빡 |

쉽게 말하면 — 공식 API 로 계좌·보유를 보려면 **한 번 부르고 몇 초 기다렸다 또 부르는** 정도만 허용된다.
대시보드처럼 빠르게 여러 번, 또는 자동매매·모니터·에이전트 루프처럼 쉴 새 없이 부르면 대부분 막힌다(429).
3개 시점 모두 똑같이 재현됐다.

**tossctl 의 답:** 공식이 막히면 tossctl 이 **곧바로 웹 세션(WTS)으로 우회**해 같은 데이터를 가져온다.
기본 설정(`prefer: auto, fallback: true`)이 자동으로 처리하고, 모든 조회 기능이 같은 길을 타므로 빠짐없이
적용된다. 위 측정에서 막힌 호출이 전부 우회로 메워져 **사용자 체감 성공률 100%·중단 0** — 즉 공식의
한도가 곧 tossctl 의 한도가 되지 않는다.

> **이 숫자는 토스가 정한 한도라 언제든 바뀔 수 있다** (공식적으로 공개된 값이 아님). 계정·시점에 따라
> 달라질 수 있고, 위 값은 2026-06-30 측정치다. 하지만 중요한 건 정확한 숫자가 아니라 **"공식이 막혀도
> tossctl 은 다른 길로 우회한다"는 구조** — 토스가 한도를 바꿔도 이 장점은 그대로다.

## 채택 전략 — 단일 레포 하이브리드 (2026-06-03 확정)

별도 레포로 쪼개지 않는다 ("별도 사유 생기기 전까지"). tossctl **이 레포 안에서**,
web 을 주력으로 두고 공식 API 를 **선택적 보완 fallback** 으로만 흡수한다.

- **web session = 주력.** 넓은 표면(transactions·watchlist·push)은 web 만 가능 →
  계속 web RE 로 표면 확장 (지금까지 trades·limits·warnings·hours·fx 추가한 흐름 유지).
- **공식 토큰 = 보완.** 딱 두 가지 가치에서만 끌어온다:

  1. **안정성 fallback** — web endpoint 가 깨질 때(#29/#30급) 공식과 겹치는 subset
     (account·holdings·quote·orders·candle 등)은 공식으로 대체 호출. 사용자 체감
     중단을 줄인다.
  2. **무인 지속성** — 핵심. 갱신에 사람이 끼는지가 갈림:

     | | 명목 수명 | 갱신 방식 | 무인 운영 |
     |---|---|---|---|
     | web session | 서버측 ~7일 (쿠키는 1년) | `auth extend` = **폰 푸시 1탭 / ~7일** | 주 1회 폰 탭 필요 |
     | 공식 토큰 | 24h (refresh 없음) | **client_secret 무인 재발급** | **폰 탭 0회 (영구)** |

     web 도 `auth extend`(폰 푸시 1탭)로 ~7일씩 연장되므로 완전 만료(QR+폰 풀 재로그인)는
     드물다. 하지만 무인 환경(서버/cron/agent)은 그 주 1회 폰 탭조차 불가능 → 결국 끊긴다.
     공식 토큰은 secret 으로 사람 없이 영구 갱신 → **무인 시나리오의 auth 는 공식이 정답.**
     반대로 대화형·넓은표면은 web 이 정답.

- **NOT 통짜 백엔드 교체.** 공식 토큰은 공식 endpoint 만 인증한다 — web 전용 표면
  (transactions·watchlist·push)은 여전히 web 쿠키 필수. 하이브리드는 *endpoint 별*.

## 분리 기준 — 어느 백엔드를 언제 쓰나

핵심은 **두 축**으로 갈린다: ① 기능이 어디에 있나(표면) ② 어떻게 실행되나(모드).

### ① 표면별 (기능이 한쪽에만 있으면 선택지 없음)

| 기능 | web | 공식 | 라우팅 |
|---|---|---|---|
| transactions ledger · watchlist · push(SSE) | ✅ | ❌ | **web 전용** (선택 불가) |
| 분봉 5종(3/5/15/30/60m) · 체결강도 등 풍부한 시세 | ✅ | 일부(1m·일봉만) | **web 우선** |
| account · holdings · quote · orders · candle(겹침) | ✅ | ✅ | **모드로 결정 ↓** |
| 선물옵션 · 해외 깊이 · 채권 | ❌ | ❌(토스) | 둘 다 없음 (KIS future) |

### ② 모드별 (겹치는 기능에 한해)

| 실행 모드 | 기본 백엔드 | 이유 |
|---|---|---|
| **대화형** (사람이 터미널에서) | **web** | 넓은 표면·무제한·풍부한 데이터·이미 동작. 폰 탭 부담 없음 |
| **무인 자동화** (cron·monitor·agent) | **공식 토큰** | 폰 탭 0회 자가갱신. web 은 주 1회 탭이 불가능해 끊김 |
| **web 장애 시** (#29/#30급) | **공식 fallback** | 겹치는 subset 만 공식으로 우회, 체감 중단 최소화 |
| **합법성 민감** (배포·컴플라이언스) | **공식** | 계약된 versioned spec, ToS 명확 |

**요약 규칙:** web-전용 기능은 무조건 web. 겹치는 기능은 *대화형이면 web, 무인이면
공식*. 공식은 "더 좁지만 더 안정적·무인 친화" 라는 좁은 자리에서만 이긴다.

## 실제 할 일

| 시점 | tossctl 동작 |
|---|---|
| **지금** | web 주력 유지 + 표면 계속 확장. issue #31 모니터링 (spec version 추적). 토스 토큰 **승인 대기** |
| **토큰 승인 후** | ① 공식 OAuth2 auth 추가 (config 에 client_id/secret) ② 무인 경로(monitor/cron)에서 공식 토큰 우선 ③ web↔공식 겹치는 endpoint 에 fallback |
| **별도 사유 생기면** | 그때 multi-broker 분리 재검토 ([multi-broker.md](./multi-broker.md) — 현재 보류) |

## 위험 요소

1. **비공식 endpoint 차단 가능성** — 공식 출시 후 토스가 reverse-engineered 접근을
   정책/기술적으로 막을 수 있음. 이것이 tossctl 의 유일한 실존적 위험 (넓은
   고유 표면이 통째로 사라짐). 차단되면 그때 공식 adapter 옵션을 검토 — 단 공식은 표면이
   좁아 완전 대체는 안 되므로 일부 기능 손실 감수. 그 전엔 선제 이주 불필요.
2. **하이브리드 복잡도** — auth 경로가 web 쿠키 + 공식 토큰 둘로 갈림. endpoint 별
   라우팅(공식 가능 subset vs web 전용)이 명확해야 혼란이 없음. → 공식 토큰은 무인
   경로 + fallback 에만 쓰고 대화형 기본은 web 으로 단순 유지.
3. **공식 토큰 24h no-refresh** — 무인 갱신 로직(만료 전 client_credentials 재발급)
   을 직접 구현해야 함. 캐시 + 만료 마진 처리 필요.

## 결정 log

각 항목은 *그 시점의* stance. 이후 정보로 뒤집힐 수 있고, 뒤집힐 때 사전 공지하지
않습니다 — 새 항목을 추가할 뿐입니다.

- **2026-05-19** — issue #31 등록 (제보: @DaeHyeoNi). 사전 신청 페이지 확인. 사전 신청 진행. tossctl 의 일반화 (multi-broker) 방향을 *현재로서는* 선호. Phase 1 진입 전까지 코드 추상화는 보류 — 공식 표면을 보기 전 추상화는 잘못 잡을 확률이 크다는 판단
- **2026-06-02** — 공식 개발자 문서 + OpenAPI spec 공개 확인 (제보: @skyisle). `developers.tossinvest.com/docs` + `openapi.tossinvest.com/openapi-docs/.../openapi.json` (v1.0.3, 20 endpoints, OAuth2 Client Credentials). 모니터링 신호를 corp 페이지 chunk hash → spec version + endpoint 목록 hash 로 교체 (훨씬 강한 신호). endpoint 표면이 tossctl 명령과 거의 1:1 매핑 확인.
- **2026-06-03a** — **이주·deprecation 계획 철회.** 공식 API 가 상위호환이 아니라 다른 trade-off (표면 좁고 rate limit 있음) 임이 spec 으로 확인됨. tossctl 이 공식으로 갈아타면 고유 강점(transactions·watchlist·push·무제한)을 스스로 버림. → web session 영구 주력.
- **2026-06-03b** — **단일 레포 하이브리드로 확정 (별도 레포 보류).** 같은 날 잠깐 "공식은 별도 investctl 레포로" 를 적었으나 (06-03a 후속), 재검토 끝에 **이 레포 안에서 web 주력 + 공식 보완 fallback** 으로 정정. 이유: ① 공식이 좁아 별도 제품화 동인 약함 ② 공식 토큰의 진짜 가치는 "통합"이 아니라 **무인 자가갱신**(24h no-refresh 지만 client_secret 로 사람 없이 재발급 → cron/agent 가 web 의 QR+폰 재인증 없이 영구 운영) + 안정성 fallback. 이 둘은 별도 레포가 아니라 tossctl 안 hybrid 로 흡수가 맞음. multi-broker/investctl 분리는 **"별도 사유 생기기 전까지" 보류** ([multi-broker.md](./multi-broker.md) = future-option). 착수는 토스 토큰 승인 후.

## 외부 contributor / 사용자에게 부탁

1. 토스 Open API 토큰을 받으셨다면 issue #31 에 댓글로 알려주세요 — phase 1 진입
   판단의 가장 빠른 신호. 토큰/계정번호 같은 민감 정보는 절대 공개 댓글에 붙이지 마세요
2. 공식 API 의 endpoint/스펙 문서를 발견하시면 issue #31 에 링크 공유 환영
3. 이 문서의 timeline/계획에 의견 있으면 issue #31 댓글 또는 별도 PR 환영
