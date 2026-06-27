# 하이브리드 공식 Open API 라우팅 — 설계

- 날짜: 2026-06-27
- 상태: 설계 승인 대기(브레인스토밍 산출물)
- 브랜치: `feat/hybrid-openapi`

## 1. 목표 / 비목표

**목표** — 사용자가 토스증권 공식 Open API 키(API Key + Secret)를 발급해 tossctl에
설정하면, **공식 API가 지원하는 기능은 공식 경로로(더 안정적으로) 보내고**, 공식이
다루지 못하는 나머지 WTS 기능은 기존 웹세션 경로로 그대로 처리하는 **선택적
하이브리드**를 제공한다.

**비목표**
- 웹세션(WTS) 경로 대체가 아님. 공식은 96% 기능을 못 다루므로 WTS는 필수로 남는다.
- OS 키체인 저장(후순위), 공식 경로 헬스 probe(후순위), JSON 구조화 소스 노출(`--show-source`, 후순위).
- 라이브 주문 자동 실행은 범위 밖(사용자가 직접). 계약 테스트 + dry-run preview 까지만 자동.

## 2. 배경 — 공식 Open API (spec v1.1.5)

- 베이스: `https://openapi.tossinvest.com`
- 인증: OAuth 2.0 **Client Credentials**.
  `POST /oauth2/token` (`application/x-www-form-urlencoded`,
  `grant_type=client_credentials` + `client_id` + `client_secret`, 무인증) →
  `access_token`(JWT) + `expires_in`(초) + `token_type`(Bearer).
  이후 모든 요청에 `Authorization: Bearer {access_token}`. + 허용 IP 제한(토스 측 설정).
- 엔드포인트 21개(= 토스 WTS 기능의 약 4%). 전부 tossctl 조회·거래 핵심과 매핑됨(부록 A).

## 3. 결정 사항 (브레인스토밍 합의)

| 축 | 결정 |
|----|------|
| 라우팅 모델 | **자동 선호 + WTS 폴백.** 키 설정 시 적격 op는 공식 자동 사용, 공식이 *쓸 수 없을* 때 WTS 폴백. |
| 자격증명 보관 | **전용 파일(0600) + env 오버라이드.** config.json엔 시크릿 미저장. |
| 구현 범위 | **전부 한 번에** — 조회 + 주문 21개 op 모두. (주문은 기존 안전 게이트 통과) |
| 아키텍처 | **A: `internal/official` 패키지 + `internal/hybrid` 임베딩 라우터.** |

## 4. 아키텍처

### 4.1 새 패키지

- **`internal/official/`** — 공식 Open API 클라이언트. WTS를 전혀 모른다(단독 테스트 가능).
  - OAuth 토큰 발급/캐시/갱신(`token.go`).
  - 21개 엔드포인트 타입드 메서드(`accounts.go`, `quote.go`, `orders.go`, …).
  - **응답 → 기존 `internal/domain` 모델 어댑터**(순수 함수).
  - 타입드 에러(§8).
- **`internal/hybrid/`** — 라우터.
  - `*client.Client`(WTS)를 **임베드**하고, 공식 적격 op만 **오버라이드**해서
    `공식 시도 → (쓸 수 없으면) WTS 폴백`. 비적격 op(~96%)는 임베딩으로 자동 통과.
  - 주문용 `trading.Broker` 구현체(공식↔WTS 라우팅 + 보수적 폴백).
  - 라우팅 정책(`prefer`, `fallback`) 보유, 소스 태깅 방출.

### 4.2 와이어링 변경 (`cmd/tossctl/root.go` `newAppContext`)

```
creds := official.LoadCredentials(env, paths.CredentialsFile)  // 없으면 nil
wtsClient := tossclient.New(...)                                // 기존 그대로

// 항상 hybrid로 감싼다. 자격증명이 없거나 prefer="wts"면 off=nil →
// hybrid는 모든 호출을 WTS로 패스스루(동작 = 기존과 동일).
var off *official.Client
if creds != nil && cfg.OpenAPI.Enabled && cfg.OpenAPI.Prefer != "wts" {
    off = official.New(creds, paths.TokenFile)
}
h := hybrid.New(wtsClient, off, routingPolicy(cfg, flags))      // 항상 *hybrid.Client

appContext.client = h                                           // 항상 *hybrid.Client (WTS 임베드)
authService = auth.NewService(..., Validator: wtsClient, ExtensionRunner: wtsClient)  // WTS 원본 유지
tradingService = trading.NewService(cfg.Trading, h.Broker())    // Broker도 hybrid (off=nil이면 WTS 패스스루)
```

- `appContext.client` 타입을 `*tossclient.Client` → **항상 `*hybrid.Client`**(WTS 임베드)로
  통일한다. 자격증명 유무와 무관하게 타입이 하나라 분기·인터페이스 보일러플레이트가 없다.
  임베딩 덕에 기존 커맨드의 메서드 호출은 그대로 컴파일되고, 적격 op만 라우팅된다.
- `off == nil`이면 hybrid는 순수 패스스루 → 기존 동작과 바이트 단위로 동일.
- `*tossclient.Client`는 이미 `trading.Broker`를 만족(현재 코드가 그렇게 주입 중)하므로,
  hybrid의 Broker는 `off==nil`일 때 이를 그대로 위임한다.
- `auth` 서비스의 Validator/ExtensionRunner는 **WTS 원본 클라이언트 유지**(세션 검증은 WTS 전용).

## 5. 인증 · 자격증명 · 토큰 수명

### 5.1 자격증명 로딩 & 우선순위

1. 환경변수 `TOSSCTL_OPENAPI_KEY` / `TOSSCTL_OPENAPI_SECRET` (있으면 우선)
2. `openapi-credentials.json` (configDir, **0600**)
3. 둘 다 없음 → 하이브리드 **자동 비활성**(순수 WTS, 기존 동작 동일)

파일 포맷:
```json
{ "api_key": "tsck_live_…", "secret_key": "tssk_live_…", "label": "default", "saved_at": "2026-06-27T14:29:00Z" }
```
- 시크릿은 출력·로그·에러·커밋 어디에도 평문으로 남기지 않는다.
- `status`/`test`는 **마스킹**해서만 표시(`tsck_live_…QXJA`).

### 5.2 토큰 수명 (`internal/official` 내부)

- 최초 적격 호출 시 토큰 발급 → `openapi-token.json`(cacheDir, **0600**)에
  `access_token` + 만료시각 캐시.
- 만료 **60초 전**까지 재사용, 이후 자동 재발급.
- API 호출이 401 → **한 번** 강제 갱신 후 재시도. 그래도 401/403 → `ErrAuth`/`ErrIPNotAllowed`.
- 토큰 발급 실패(잘못된 키, IP 차단)는 명확한 타입드 에러로 구분(§8).

## 6. 라우팅 & 폴백

### 6.1 적격성 & 결정 (op 1건마다)

- 정적 **적격 매핑 테이블**(부록 A): 조회 ~15 + 주문 3.
- 결정: `자격증명 있음 AND prefer≠"wts" AND op 적격` → 공식 시도. 그 외 → WTS.
  - `prefer="official"`도 비적격 op는 항상 WTS(공식에 엔드포인트가 없으므로).
- 비적격 op(watchlist·market·push·community·export·dividends 등)는 임베딩으로 무조건 WTS.

### 6.2 폴백 구분 (핵심) — `prefer=auto AND fallback=true`

- **폴백함 (공식을 "쓸 수 없음")**: 토큰/인증 실패(갱신 후에도 401/403),
  IP 미허용(403), 429(rate limit), 5xx, 네트워크/타임아웃 → WTS로 재시도.
- **폴백 안 함 (공식이 "정상 답변")**: 404 종목없음, 400 잘못된 파라미터 등
  도메인 에러는 *진짜 답*이므로 그대로 반환(폴백하면 결과를 가린다).
- 구분 기준: `official` 패키지의 타입드 에러 클래스(§8). transport/config 계열만 폴백.

### 6.3 주문 폴백 — 보수적 (중복 주문 방지)

- 전송 **이전** 단계 실패(미설정·DNS·연결 거부 등 요청이 서버에 닿기 전)만 WTS 폴백 허용.
- 요청을 보낸 **이후의 모호한 실패**(타임아웃, 응답 파싱 실패 등)는 **절대 교차 폴백하지 않는다.**
  공식에서 주문이 접수됐을 수 있어 WTS로 재시도하면 중복 주문 위험.
  → "공식 경로 주문 결과 불명, `tossctl orders`로 직접 확인 요망" 에러 반환.

### 6.4 소스 태깅 (출력 계약 비파괴)

- **stdout 페이로드는 변경 없음** — 기존 fixture/계약 테스트 보존.
- 백엔드/폴백 여부는 **stderr에 dim 한 줄**: `via official` / `official unavailable → wts (5xx)`.
  머신 파이프라인은 stdout만 읽으므로 영향 없음.
- 구조화 노출(`--show-source`, JSON `meta.source`)은 후순위.

## 7. 응답 어댑터 (공식 JSON → `internal/domain`)

- 엔드포인트별 **순수 함수** 어댑터: 공식 JSON → 커맨드·output이 이미 쓰는 표준 domain 타입.
- 필드 불일치:
  - 공식에 없는 필드(WTS엔 있던) → zero/empty로 두고 어댑터 주석에 명시.
  - 공식에만 있는 필드 → 무시(또는 domain에 유용하면 매핑).
- 순수 함수이므로 더미 JSON 픽스처로 단위테스트.

## 8. 에러 처리

`internal/official`이 타입드 에러를 정의(기존 `internal/client/errors.go` 스타일):

| 에러 | 의미 | 폴백? |
|------|------|:----:|
| `ErrTransport` | 네트워크/타임아웃/DNS | ✅ |
| `ErrAuth` | 키/시크릿 무효, 갱신 후 401 | ✅ |
| `ErrIPNotAllowed` | 허용 IP 아님(403) | ✅ |
| `ErrRateLimited` | 429 | ✅ |
| `ErrServer` | 5xx | ✅ |
| 도메인 에러(패스스루) | 400/404 등 정상 거절 | ❌ (반환) |

- `openapi test`는 실행가능한 메시지로 변환("공인 IP 222.x 미허용 → 토스 설정에서 추가").

## 9. CLI 표면

신규 커맨드 그룹 `tossctl openapi <sub>` (웹세션 `auth`와 별개 자격증명이라 분리):

- `openapi login` — `--key/--secret` 또는 stdin 프롬프트 → 0600 파일 저장. 실제 키 미출력.
- `openapi status` — 자격증명 유무, 토큰 유효성, 현재 공인 IP↔허용목록 힌트, 공식 경로로 갈 op 목록(마스킹).
- `openapi test` — 토큰 발급 + 저비용 호출(`/accounts`)로 키·IP·연결 실검증.
- `openapi logout` — 자격증명/토큰 파일 삭제.

전역 플래그 `--backend auto|wts|official` — 1회성 라우팅 오버라이드(config `prefer`보다 우선).

## 10. 설정 & 경로

`config.json`에 `openapi` 블록(**시크릿 아님**, 라우팅 취향만):
```json
"openapi": { "enabled": true, "prefer": "auto", "fallback": true }
```
- `enabled`: 마스터 토글(false면 키가 있어도 순수 WTS).
- `prefer`: `auto`(지원 op는 공식) | `wts`(킬스위치) | `official`.
- `fallback`: 공식이 쓸 수 없을 때 WTS 폴백 여부.
- `config` 스키마 버전 +1, 마이그레이션은 기존 `rawConfig` 패턴 따름.

`internal/config/paths.go`에 추가:
- `CredentialsFile = configDir/openapi-credentials.json`
- `TokenFile = cacheDir/openapi-token.json`

## 11. 테스트 전략 (프로젝트 원칙: httptest + **더미 데이터**, 라이브 호출 없음)

- `internal/official`: httptest로 더미 공식 JSON → 어댑터 + 토큰발급 + 401-갱신 + 에러분류 테스트.
- `internal/hybrid`: fake WTS/공식 백엔드로 테이블 기반 라우팅 테스트 —
  적격성, 폴백 각 실패클래스, 도메인에러 시 비폴백, 주문 보수적 폴백(전송 후 모호 실패 시 비폴백).
- `internal/trading`: PlaceIntent→공식 주문 바디 계약 테스트(기존 `trading_test.go` 방식), preview/dry-run 불변.
- `cmd/tossctl`: `openapi` 커맨드(login이 0600 저장, status 마스킹, logout 삭제).
- 라이브 검증은 비자동 — `openapi test` + `order preview` 수동, 실주문은 사용자 몫.

## 12. 문서 / 카탈로그 갱신 (프로젝트 흐름)

- `content/docs/reference/support-scope.mdx`(+ `.en`) — 공식 칼럼이 이제 *실제 라우팅*됨 반영.
- README 비교표(+ `since` 마커), `CHANGELOG.md` `[Unreleased]`.
- 랜딩 "선택적 하이브리드" 카피 — 이제 실재 기능으로.
- `docs/reverse-engineering/wts-endpoints.json` / openapi 스냅샷은 기존 자동 추적.
- probe 추가는 후순위 노트.

## 13. 범위 밖 / 향후

- OS 키체인 저장, 공식 경로 헬스 probe, `--show-source`/JSON `meta.source` 구조화 노출.
- market-calendar(KR/US)용 신규 tossctl 커맨드(공식엔 있으나 현재 tossctl 미보유) — 별도 검토.

---

## 부록 A — 적격 매핑 테이블 (tossctl op → 공식 엔드포인트)

| tossctl | 공식 엔드포인트 |
|---------|-----------------|
| `account list` / `summary` | `GET /api/v1/accounts`, `GET /api/v1/buying-power`, `GET /api/v1/exchange-rate` |
| `portfolio positions` | `GET /api/v1/holdings` |
| `quote get` | `GET /api/v1/prices`, `GET /api/v1/stocks` |
| `quote orderbook` | `GET /api/v1/orderbook` |
| `quote trades` | `GET /api/v1/trades` |
| `quote chart` | `GET /api/v1/candles` |
| `quote limits` | `GET /api/v1/price-limits` |
| `quote warnings` | `GET /api/v1/stocks/{symbol}/warnings` |
| `quote sellable` | `GET /api/v1/sellable-quantity` |
| `quote commission` | `GET /api/v1/commissions` |
| `orders` (목록/상세) | `GET /api/v1/orders`, `GET /api/v1/orders/{orderId}` |
| `order place` | `POST /api/v1/orders` |
| `order cancel` | `POST /api/v1/orders/{orderId}/cancel` |
| `order amend` | `POST /api/v1/orders/{orderId}/modify` |
| (신규 검토) market-calendar | `GET /api/v1/market-calendar/KR`, `/US` |

**비적격(항상 WTS)**: `quote flows`·`market *`·`watchlist *`·`community *`·
`portfolio dividends`·`transactions *`·`export *`·`push listen` 등.
