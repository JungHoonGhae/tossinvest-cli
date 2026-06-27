# 하이브리드 공식 Open API 라우팅 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 사용자가 토스증권 공식 Open API 키를 설정하면, 공식이 지원하는 op는 공식 경로(OAuth)로 보내고 실패 시 WTS로 폴백하는 선택적 하이브리드를 제공한다.

**Architecture:** 새 `internal/official`(OAuth 토큰 + 공식 21개 엔드포인트 + domain 어댑터)와 `internal/hybrid`(`*client.Client`를 임베드해 적격 op만 오버라이드·폴백, 주문용 `trading.Broker` 구현)를 추가한다. 커맨드는 항상 `*hybrid.Client`를 받고, 자격증명이 없으면 hybrid는 순수 WTS 패스스루다.

**Tech Stack:** Go 1.25+, cobra, 표준 `net/http`, `net/http/httptest`(테스트). 외부 의존성 추가 없음.

설계 출처: `docs/superpowers/specs/2026-06-27-hybrid-openapi-design.md`.

## Global Constraints

- Go 1.25+, cobra CLI. 핵심/AI 경로는 표준 라이브러리 위주(외부 의존성 최소). **예외**: 온보딩 TUI 위저드용 `github.com/charmbracelet/huh`(+ 전이 의존 bubbletea/lipgloss 등) 허용. 단 TUI는 **사람용 옵트인**이며 비TTY/AI 경로는 huh 없이도 완전 동작해야 한다(플래그/JSON). official/hybrid/client 등 비-TUI 패키지는 huh를 import하지 않는다(TUI 의존은 `internal/tui`·`cmd`에 격리).
- 거래는 기본 비활성·config 게이트 유지. 공식 경로 주문도 기존 `trading.Service` 게이트·`order preview`·`--execute/--confirm`를 그대로 통과.
- **공개 출력·테스트·문서에 실제 계좌/키 데이터 금지** — 더미/합성 데이터만. 계약 테스트는 `httptest` + 더미.
- 시크릿(API Key/Secret/토큰)은 디스크 저장 시 **0600**. 출력·로그·에러에 평문 노출 금지(마스킹).
- **stdout 페이로드 불변** — 백엔드/폴백 표시는 stderr. 기존 fixture/계약 테스트가 깨지면 안 됨.
- 라이브 주문 자동 실행 금지. 계약 테스트 + `order preview`까지만 자동, 실주문은 사용자.
- git 출력물(커밋/PR)에 Claude attribution 문구 금지.
- 공식 베이스 URL `https://openapi.tossinvest.com`, 토큰 `POST /oauth2/token`(form-urlencoded), 이후 `Authorization: Bearer`.
- **AI 우선 조작성** — 모든 신규 커맨드는 플래그만으로 **완전 비대화형** 실행 가능해야 하고(`--output json` 지원), AI/스크립트 경로에서 프롬프트로 멈추지 않는다. 대화형 입력은 TTY이고 플래그가 빠졌을 때만 폴백. 非TTY+플래그 누락이면 **프롬프트 대기 없이** 무엇을 줘야 하는지 에러로 알린다(이후 TUI 트랙도 동일 원칙: TUI는 사람용 옵트인, AI 경로는 항상 플래그/JSON).

---

## File Structure

**신규**
- `internal/official/credentials.go` — 자격증명 로딩(env > 0600 파일), 저장/삭제, 마스킹.
- `internal/official/errors.go` — 타입드 에러 + HTTP 상태 분류.
- `internal/official/token.go` — OAuth 토큰 발급/캐시(0600)/만료갱신.
- `internal/official/client.go` — base URL, authed GET/POST 헬퍼, ApiResponse envelope 언랩, 401 1회 갱신 재시도.
- `internal/official/reads.go` — 조회 엔드포인트 메서드 + domain 어댑터(순수 함수).
- `internal/official/orders.go` — 주문 생성/취소/정정 + `OrderCreateRequest` 빌더.
- `internal/official/*_test.go` — httptest + 더미.
- `internal/hybrid/client.go` — `*client.Client` 임베드 + 적격 op 오버라이드 + 라우팅/폴백/소스태깅.
- `internal/hybrid/broker.go` — `trading.Broker` 구현(보수적 주문 폴백).
- `internal/hybrid/*_test.go` — fake 백엔드 라우팅 테스트.
- `cmd/tossctl/openapi.go` — `openapi login/status/test/logout` 커맨드(얇은 표현 계층).
- `cmd/tossctl/openapi_test.go`.

**수정**
- `internal/config/service.go` — `OpenAPI` 블록 + raw 마이그레이션 + 스키마 버전 +1.
- `internal/config/paths.go` — `CredentialsFile`, `TokenFile`.
- `internal/client/openapi_meta.go`(신규) + `probe`/카탈로그 — `OpenAPIClientInfo`, `OpenAPIAllowedIPs`(WTS).
- `cmd/tossctl/root.go` — `newAppContext` 와이어링(항상 hybrid 래핑).
- `cmd/tossctl/doctor.go` — 하이브리드 진단 한 줄.
- `cmd/tossctl/root.go` 전역 플래그 `--backend`.
- `docs/reverse-engineering/wts-endpoints.json` — 두 엔드포인트 candidate→implemented.
- `content/docs/reference/support-scope.mdx`(+`.en`), `README.md`, `CHANGELOG.md`.

---

## Task 1: config `openapi` 블록 + paths

**Files:**
- Modify: `internal/config/service.go`
- Modify: `internal/config/paths.go`
- Test: `internal/config/service_test.go`

**Interfaces:**
- Produces: `config.OpenAPI{Enabled bool; Prefer string; Fallback bool}`; `config.File.OpenAPI`; `config.Trading` 불변; `paths.CredentialsFile`, `paths.TokenFile`.

- [ ] **Step 1: 실패 테스트 작성** — 기본값(미설정 시 Enabled=true, Prefer="auto", Fallback=true)과 raw 마이그레이션(누락 시 기본값 채움) 검증.

```go
func TestLoadDefaultsOpenAPI(t *testing.T) {
	dir := t.TempDir()
	svc := NewService(filepath.Join(dir, "config.json"))
	cfg, err := svc.Load(context.Background())
	if err != nil { t.Fatal(err) }
	if !cfg.OpenAPI.Enabled || cfg.OpenAPI.Prefer != "auto" || !cfg.OpenAPI.Fallback {
		t.Fatalf("unexpected openapi defaults: %+v", cfg.OpenAPI)
	}
}
```

- [ ] **Step 2: 테스트 실패 확인** — Run: `go test ./internal/config/ -run TestLoadDefaultsOpenAPI -v` · Expected: FAIL (`cfg.OpenAPI` 미정의).

- [ ] **Step 3: 구현** — `Config`/`File`에 `OpenAPI` 추가, `rawConfig`에 `*rawOpenAPI` + 기본값 머지, 스키마 버전 +1.

```go
type OpenAPI struct {
	Enabled  bool   `json:"enabled"`
	Prefer   string `json:"prefer"`   // auto | wts | official
	Fallback bool   `json:"fallback"`
}
// File/Config struct에 필드 추가:  OpenAPI OpenAPI `json:"openapi"`
// rawConfig 머지(누락 시): Enabled=true, Prefer="auto", Fallback=true. Prefer 미허용값이면 "auto"로.
```

`paths.go`:
```go
// Paths struct에 추가
CredentialsFile string
TokenFile       string
// DefaultPaths() 반환에 추가
CredentialsFile: filepath.Join(configDir, "openapi-credentials.json"),
TokenFile:       filepath.Join(cacheRoot, AppName, "openapi-token.json"),
```

- [ ] **Step 4: 통과 확인** — Run: `go test ./internal/config/ -v` · Expected: PASS (기존 테스트 포함).

- [ ] **Step 5: 커밋** — `git add internal/config && git commit -m "feat(config): openapi 라우팅 블록 + 자격증명/토큰 경로"`

---

## Task 2: official 자격증명 로딩 (env > 0600 파일)

**Files:**
- Create: `internal/official/credentials.go`
- Test: `internal/official/credentials_test.go`

**Interfaces:**
- Produces:
  - `type Credentials struct { APIKey, SecretKey, Label, SavedAt string }`
  - `func LoadCredentials(getenv func(string) string, file string) (*Credentials, error)` — env(`TOSSCTL_OPENAPI_KEY`/`TOSSCTL_OPENAPI_SECRET`) 우선, 없으면 파일. 둘 다 없으면 `(nil, nil)`.
  - `func SaveCredentials(file string, c Credentials) error` — 0600 저장.
  - `func DeleteCredentials(file string) error`.
  - `func (c Credentials) MaskedKey() string` — `tsck_live_…aVLA`(앞 10 + … + 뒤 4).

- [ ] **Step 1: 실패 테스트** —

```go
func TestLoadCredentialsEnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "c.json")
	if err := SaveCredentials(file, Credentials{APIKey: "filek", SecretKey: "files"}); err != nil { t.Fatal(err) }
	env := map[string]string{"TOSSCTL_OPENAPI_KEY": "envk", "TOSSCTL_OPENAPI_SECRET": "envs"}
	c, err := LoadCredentials(func(k string) string { return env[k] }, file)
	if err != nil { t.Fatal(err) }
	if c.APIKey != "envk" { t.Fatalf("env should win, got %q", c.APIKey) }
}
func TestSaveCredentialsIs0600(t *testing.T) {
	dir := t.TempDir(); file := filepath.Join(dir, "c.json")
	if err := SaveCredentials(file, Credentials{APIKey: "k", SecretKey: "s"}); err != nil { t.Fatal(err) }
	fi, _ := os.Stat(file)
	if fi.Mode().Perm() != 0o600 { t.Fatalf("want 0600, got %v", fi.Mode().Perm()) }
}
func TestLoadCredentialsNoneReturnsNil(t *testing.T) {
	c, err := LoadCredentials(func(string) string { return "" }, filepath.Join(t.TempDir(), "absent.json"))
	if err != nil || c != nil { t.Fatalf("want nil,nil got %v,%v", c, err) }
}
func TestMaskedKey(t *testing.T) {
	c := Credentials{APIKey: "tsck_live_9I24L3TIMVgiFfakZJaVLA"}
	if got := c.MaskedKey(); got != "tsck_live_…aVLA" { t.Fatalf("got %q", got) }
}
```

- [ ] **Step 2: 실패 확인** — Run: `go test ./internal/official/ -run TestLoadCredentials -v` · Expected: FAIL(패키지/함수 없음).

- [ ] **Step 3: 구현** — env 우선(둘 다 있어야 채택), 아니면 파일 JSON 파싱(없으면 nil,nil). `SaveCredentials`는 `os.WriteFile(file, data, 0o600)`(상위 디렉터리 `os.MkdirAll(dir, 0o700)`). `MaskedKey`: 길이 14 미만이면 전부 `…`.

- [ ] **Step 4: 통과 확인** — Run: `go test ./internal/official/ -v` · Expected: PASS.

- [ ] **Step 5: 커밋** — `git commit -am "feat(official): 자격증명 로딩(env>0600 파일)+마스킹"`

---

## Task 3: OAuth 토큰 클라이언트 + 타입드 에러

**Files:**
- Create: `internal/official/errors.go`, `internal/official/token.go`
- Test: `internal/official/token_test.go`, `internal/official/errors_test.go`

**Interfaces:**
- Produces:
  - errors.go: `ErrTransport, ErrAuth, ErrIPNotAllowed, ErrRateLimited, ErrServer` (sentinel `error`); `func classifyStatus(code int, body []byte) error` — 401/403→(IP 단서 있으면 ErrIPNotAllowed, 아니면 ErrAuth), 429→ErrRateLimited, ≥500→ErrServer, 그 외 4xx→`*APIError{Code,Body}`(패스스루, 폴백 안 함); `func ShouldFallback(err error) bool`(transport/auth/ip/rate/server=true, APIError=false).
  - token.go: `type tokenManager struct{...}`; `func newTokenManager(creds Credentials, base, cacheFile string, hc *http.Client) *tokenManager`; `func (m *tokenManager) token(ctx) (string, error)`(캐시 유효하면 재사용, 아니면 발급); `func (m *tokenManager) refresh(ctx) (string, error)`(강제 재발급).

- [ ] **Step 1: 실패 테스트** — httptest 토큰 서버로 발급·캐시·만료갱신, 분류기 검증.

```go
func TestTokenExchangeAndCache(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth2/token" { t.Fatalf("path %s", r.URL.Path) }
		_ = r.ParseForm()
		if r.Form.Get("grant_type") != "client_credentials" || r.Form.Get("client_id") != "k" { t.Fatal("bad form") }
		hits++
		_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
	}))
	defer srv.Close()
	m := newTokenManager(Credentials{APIKey: "k", SecretKey: "s"}, srv.URL, filepath.Join(t.TempDir(), "t.json"), srv.Client())
	tok, err := m.token(context.Background())
	if err != nil || tok != "AT" { t.Fatalf("got %q,%v", tok, err) }
	_, _ = m.token(context.Background()) // 캐시 재사용
	if hits != 1 { t.Fatalf("expected 1 exchange, got %d", hits) }
}
func TestClassifyStatus(t *testing.T) {
	if !errors.Is(classifyStatus(403, []byte(`{"message":"ip not allowed"}`)), ErrIPNotAllowed) { t.Fatal("403 ip") }
	if !errors.Is(classifyStatus(401, nil), ErrAuth) { t.Fatal("401") }
	if !errors.Is(classifyStatus(429, nil), ErrRateLimited) { t.Fatal("429") }
	if !errors.Is(classifyStatus(503, nil), ErrServer) { t.Fatal("5xx") }
	if ShouldFallback(classifyStatus(404, nil)) { t.Fatal("404 must not fallback") }
}
```

- [ ] **Step 2: 실패 확인** — Run: `go test ./internal/official/ -run 'TestToken|TestClassify' -v` · Expected: FAIL.

- [ ] **Step 3: 구현** — 토큰 발급: `POST {base}/oauth2/token`, `Content-Type: application/x-www-form-urlencoded`, body `grant_type=client_credentials&client_id=…&client_secret=…`. 200 아니면 `classifyStatus`. 응답 파싱(`access_token`,`expires_in`) → 메모리 + `cacheFile`(0600)에 `{access_token, expires_at}` 저장. `token()`은 `expires_at-60s > now`면 메모리/파일 재사용. IP 단서 판정: 본문에 `ip`(대소문자 무시) 포함 시 ErrIPNotAllowed.

- [ ] **Step 4: 통과 확인** — Run: `go test ./internal/official/ -v` · Expected: PASS.

- [ ] **Step 5: 커밋** — `git commit -am "feat(official): OAuth 토큰 발급/캐시/갱신 + 에러 분류"`

---

## Task 4: official 클라이언트 코어 (authed 요청 + envelope 언랩 + 401 재시도)

**Files:**
- Create: `internal/official/client.go`
- Test: `internal/official/client_test.go`

**Interfaces:**
- Produces:
  - `type Client struct{...}`; `func New(creds Credentials, cacheFile string, opts ...Option) *Client`(opts로 base URL·http.Client 주입 — 테스트용); `func (c *Client) BaseURL() string`.
  - 내부: `func (c *Client) get(ctx, path string, q url.Values, out any) error`, `func (c *Client) post(ctx, path string, body any, out any) error`. 둘 다 `Authorization: Bearer`, 비2xx→`classifyStatus`, 401이면 `refresh` 후 1회 재시도. 본문은 `{"result": <out>}` envelope를 언랩해 `out`에 디코드.

- [ ] **Step 1: 실패 테스트** — Bearer 부착, envelope 언랩, 401→갱신→재시도.

```go
func TestGetUnwrapsEnvelopeAndRetriesOn401(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`))
		case "/api/v1/ping":
			calls++
			if calls == 1 { w.WriteHeader(401); return } // 첫 호출 401 → 갱신 후 재시도
			if r.Header.Get("Authorization") != "Bearer AT" { t.Fatalf("auth %q", r.Header.Get("Authorization")) }
			_, _ = w.Write([]byte(`{"result":{"ok":true}}`))
		}
	}))
	defer srv.Close()
	c := New(Credentials{APIKey: "k", SecretKey: "s"}, filepath.Join(t.TempDir(), "t.json"), WithBaseURL(srv.URL), WithHTTPClient(srv.Client()))
	var out struct{ OK bool `json:"ok"` }
	if err := c.get(context.Background(), "/api/v1/ping", nil, &out); err != nil { t.Fatal(err) }
	if !out.OK { t.Fatal("envelope not unwrapped") }
	if calls != 2 { t.Fatalf("expected retry, calls=%d", calls) }
}
```

- [ ] **Step 2: 실패 확인** — Run: `go test ./internal/official/ -run TestGet -v` · Expected: FAIL.

- [ ] **Step 3: 구현** — `Option`(`WithBaseURL`, `WithHTTPClient`). `get/post`: 토큰 부착 → 요청 → 401이면 `refresh()` 후 재시도(1회) → 비2xx면 `classifyStatus(code, body)` → 2xx면 `json.Unmarshal` 후 `result` 추출(`struct{ Result json.RawMessage }` → `out`). 기본 http.Client 타임아웃 15s.

- [ ] **Step 4: 통과 확인** — Run: `go test ./internal/official/ -v` · Expected: PASS.

- [ ] **Step 5: 커밋** — `git commit -am "feat(official): authed 요청 코어 + envelope 언랩 + 401 재시도"`

---

## Task 5: 조회 exemplar — Accounts → `domain.Account`

**Files:**
- Create: `internal/official/reads.go`
- Test: `internal/official/reads_test.go`

**Interfaces:**
- Consumes: `Client.get`.
- Produces: `func (c *Client) Accounts(ctx) ([]domain.Account, error)`; `func adaptAccounts(raw []apiAccount) []domain.Account`.

공식 `Account{accountNo, accountSeq, accountType}` → `domain.Account`(필드는 `internal/domain/models.go`의 `Account` 정의를 따른다 — `accountSeq`를 식별 키로 매핑). 어댑터는 순수 함수.

- [ ] **Step 1: 실패 테스트** — 더미 envelope JSON으로 어댑터+호출 검증.

```go
func TestAccountsAdapter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth2/token" { _, _ = w.Write([]byte(`{"access_token":"AT","expires_in":3600,"token_type":"Bearer"}`)); return }
		_, _ = w.Write([]byte(`{"result":[{"accountNo":"123-45","accountSeq":7,"accountType":"BROKERAGE"}]}`))
	}))
	defer srv.Close()
	c := New(Credentials{APIKey: "k", SecretKey: "s"}, filepath.Join(t.TempDir(), "t.json"), WithBaseURL(srv.URL), WithHTTPClient(srv.Client()))
	got, err := c.Accounts(context.Background())
	if err != nil { t.Fatal(err) }
	if len(got) != 1 || got[0].Number != "123-45" { t.Fatalf("got %+v", got) }
}
```
*(주: `domain.Account`의 실제 필드명을 `internal/domain/models.go`에서 확인해 `Number` 등 정확히 사용.)*

- [ ] **Step 2: 실패 확인** — Run: `go test ./internal/official/ -run TestAccounts -v` · Expected: FAIL.

- [ ] **Step 3: 구현** — `apiAccount` 구조체 + `Accounts()`(`c.get("/api/v1/accounts", nil, &raw)`) + `adaptAccounts`.

- [ ] **Step 4: 통과 확인** — Run: `go test ./internal/official/ -v` · Expected: PASS.

- [ ] **Step 5: 커밋** — `git commit -am "feat(official): accounts 조회 + 어댑터 (exemplar)"`

---

## Task 6: 나머지 조회 엔드포인트 (exemplar 패턴 반복)

각 행마다 **Task 5의 5-step TDD 사이클을 동일하게 반복**한다(실패 테스트→실패 확인→`apiXxx` 구조체+메서드+어댑터 구현→통과→커밋). 각 행의 데이터는 고유하다 — 공식 응답 컴포넌트는 `docs/migration/openapi.latest.json`의 명시된 스키마를, 대상 타입은 `internal/domain/models.go`를 권위 출처로 매핑한다. 더미 픽스처는 envelope `{"result": …}`로 감싼다.

| 메서드 | 공식 엔드포인트 (params) | 응답 컴포넌트 | 대상 domain |
|--------|--------------------------|----------------|-------------|
| `BuyingPower(ctx, currency)` | `GET /buying-power` (`currency`) | `BuyingPowerResponse` | `domain.AccountSummary`/예수금 필드 |
| `ExchangeRate(ctx, base, quote)` | `GET /exchange-rate` (`baseCurrency,quoteCurrency,dateTime`) | `ExchangeRateResponse` | `domain.ExchangeRate` |
| `Holdings(ctx, symbol)` | `GET /holdings` (`symbol` opt) | `HoldingsItem[]`,`HoldingsOverview` | `domain.Position` |
| `Prices(ctx, symbols)` | `GET /prices` (`symbols`) | `PriceResponse`/`Price` | `domain.Quote` |
| `Stocks(ctx, symbols)` | `GET /stocks` (`symbols`) | `StockInfo` | `domain.Quote`(기본정보 보강) |
| `Warnings(ctx, symbol)` | `GET /stocks/{symbol}/warnings` | `StockWarning[]` | `domain.StockWarnings` |
| `Orderbook(ctx, symbol)` | `GET /orderbook` (`symbol`) | `OrderbookResponse`,`OrderbookEntry` | `domain.OrderBook`,`OrderBookLevel` |
| `Trades(ctx, symbol, count)` | `GET /trades` (`symbol,count`) | `Trade[]` | `domain.TradeList`,`domain.Trade` |
| `Candles(ctx, symbol, interval, count, before, adjusted)` | `GET /candles` | `CandlePageResponse`,`Candle` | `domain.Chart`,`domain.Candle` |
| `PriceLimits(ctx, symbol)` | `GET /price-limits` (`symbol`) | `PriceLimitResponse` | `domain.PriceLimits` |
| `SellableQuantity(ctx, accountSeq, symbol)` | `GET /sellable-quantity` | `SellableQuantityResponse` | `domain.SellableQuantity` |
| `Commissions(ctx, symbol)` | `GET /commissions` (`symbol`) | `Commission` | `domain.Commission` |
| `Orders(ctx, filter)` | `GET /orders` (`status,symbol,from,to,cursor,limit`) | `PaginatedOrderResponse`,`Order` | `domain.Order` |
| `OrderByID(ctx, orderId)` | `GET /orders/{orderId}` | `Order` | `domain.Order` |

- [ ] 위 14개 메서드 각각에 대해 Task 5 사이클 수행. 메서드당 1 커밋(`feat(official): <name> 조회 + 어댑터`). 매핑 불명 필드는 zero값 + 어댑터 주석에 명시.
- [ ] 전체 통과 확인 — Run: `go test ./internal/official/ -v`

---

## Task 7: WTS 키 메타데이터 메서드 (진단용)

**Files:**
- Create: `internal/client/openapi_meta.go`
- Test: `internal/client/openapi_meta_test.go`
- Modify: `docs/reverse-engineering/wts-endpoints.json`(candidate→implemented)

**Interfaces:**
- Produces:
  - `type OpenAPIClientInfo struct { Status string; IssuedAt, ExpiresAt time.Time; Active bool }`
  - `func (c *Client) OpenAPIClientInfo(ctx) (OpenAPIClientInfo, error)` ← `GET /api/v1/openapi/client`
  - `func (c *Client) OpenAPIAllowedIPs(ctx) ([]string, error)` ← `GET /api/v1/openapi/client/allowed-ips`

- [ ] **Step 1: 실패 테스트** — 기존 `internal/client` 계약 테스트 패턴(httptest + 세션) 따라 더미 메타 응답 파싱.
- [ ] **Step 2: 실패 확인** — Run: `go test ./internal/client/ -run TestOpenAPI -v` · Expected: FAIL.
- [ ] **Step 3: 구현** — 기존 WTS authed GET 헬퍼 사용(다른 read 메서드 참고: `internal/client/account.go`). 응답 필드명은 라이브 캡처가 없으므로, 가장 그럴듯한 매핑(status/issuedAt/expiresAt) + 누락 graceful. 카탈로그 두 항목 status `implemented`로 변경.
- [ ] **Step 4: 통과 확인** — Run: `go test ./internal/client/ -v`
- [ ] **Step 5: 커밋** — `git commit -am "feat(client): WTS openapi 키 메타/허용IP 조회"`

---

## Task 8: 공식 주문 (생성 빌더 + 취소/정정)

**Files:**
- Create: `internal/official/orders.go`
- Test: `internal/official/orders_test.go`

**Interfaces:**
- Consumes: `Client.post`, `orderintent.PlaceIntent/CancelIntent/AmendIntent`, `trading.MutationResult`.
- Produces:
  - `func buildOrderCreate(intent orderintent.PlaceIntent) (any, error)` — `OrderCreateRequest` 두 변형 매핑:
    - 지정가/시장가 수량 주문(variant0): `{symbol, side, orderType: LIMIT|MARKET, price, quantity, timeInForce}`.
    - 시장가 금액 주문(variant1, 소수점 매수): `{symbol, side, orderType: MARKET, orderAmount}`.
    - 소수점 매도는 variant0의 `quantity`(소수) + `orderType: MARKET`.
  - `func (c *Client) PlaceOrder(ctx, intent) (trading.MutationResult, error)` ← `POST /api/v1/orders`(`OrderOperationResponse{orderId}` → MutationResult).
  - `func (c *Client) CancelOrder(ctx, orderID string) (trading.MutationResult, error)` ← `POST /api/v1/orders/{orderId}/cancel`.
  - `func (c *Client) ModifyOrder(ctx, intent) (trading.MutationResult, error)` ← `POST /api/v1/orders/{orderId}/modify`(`OrderModifyRequest{orderType, price, quantity}`).

- [ ] **Step 1: 실패 테스트** — `buildOrderCreate`가 의도별로 정확한 JSON을 만드는지 + POST 호출 계약(더미 `{"result":{"orderId":"O1"}}`).

```go
func TestBuildOrderCreateFractionalSell(t *testing.T) {
	body, err := buildOrderCreate(orderintent.PlaceIntent{Symbol: "TSLA", Side: "sell", Fractional: true, Quantity: 0.5})
	if err != nil { t.Fatal(err) }
	b, _ := json.Marshal(body)
	got := string(b)
	for _, want := range []string{`"symbol":"TSLA"`, `"side":"SELL"`, `"orderType":"MARKET"`, `"quantity":"0.5"`} {
		if !strings.Contains(got, want) { t.Fatalf("missing %s in %s", want, got) }
	}
	if strings.Contains(got, "orderAmount") { t.Fatal("sell must not use orderAmount") }
}
func TestPlaceOrderParsesOrderId(t *testing.T) { /* httptest POST → MutationResult.OrderID == "O1" */ }
```
*(`orderintent.PlaceIntent`/`trading.MutationResult` 실제 필드는 `internal/orderintent`·`internal/trading` 확인. 가격/수량 문자열 변환은 기존 `internal/client/trading.go`의 포맷 로직과 일관되게.)*

- [ ] **Step 2: 실패 확인** — Run: `go test ./internal/official/ -run 'TestBuildOrder|TestPlaceOrder' -v` · Expected: FAIL.
- [ ] **Step 3: 구현** — 빌더 + 3개 메서드. side 대문자화, 금액/수량 문자열.
- [ ] **Step 4: 통과 확인** — Run: `go test ./internal/official/ -v`
- [ ] **Step 5: 커밋** — `git commit -am "feat(official): 주문 생성/취소/정정 + OrderCreateRequest 빌더"`

---

## Task 9: hybrid 라우터 (임베드 + 라우팅/폴백/소스태깅)

**Files:**
- Create: `internal/hybrid/client.go`
- Test: `internal/hybrid/client_test.go`

**Interfaces:**
- Consumes: `*client.Client`(WTS), `*official.Client`, `official.ShouldFallback`, `config.OpenAPI`.
- Produces:
  - `type Policy struct { Prefer string; Fallback bool }`
  - `type Client struct { *client.Client; off *official.Client; pol Policy; stderr io.Writer }`(WTS 임베드)
  - `func New(wts *client.Client, off *official.Client, pol Policy, stderr io.Writer) *Client`
  - 적격 op 오버라이드(시그니처는 WTS 메서드와 **동일**해야 임베딩을 가린다): `Accounts`, `Positions`/`Holdings`, `Quote`, `OrderBook`, `Trades`, `Chart`, `PriceLimits`, `Warnings`, `SellableQuantity`, `Commission`, `Orders`, `ExchangeRate` 등. 각 메서드 내부에서 `route(...)` 호출.
  - `func (c *Client) Broker() trading.Broker`(Task 10).
  - 헬퍼: `func route[T any](c *Client, name string, official func() (T, error), wts func() (T, error)) (T, error)` — `off==nil || pol.Prefer=="wts"`면 WTS. 아니면 official 시도; 성공 시 stderr `via official` 후 반환; 실패가 `ShouldFallback && pol.Fallback`면 stderr `official unavailable → wts (<reason>)` 후 WTS; 아니면 official 에러 반환.

- [ ] **Step 1: 실패 테스트** — fake 백엔드(인터페이스/함수 주입)로: (a) prefer=auto면 official 사용, (b) official이 ErrServer면 WTS 폴백, (c) official이 도메인 APIError(404)면 폴백 안 하고 그 에러 반환, (d) prefer=wts면 official 미호출.

```go
func TestRoutePrefersOfficialThenFallsBackOnServerError(t *testing.T) {
	var buf bytes.Buffer
	c := &Client{pol: Policy{Prefer: "auto", Fallback: true}, stderr: &buf}
	c.off = &official.Client{} // 실제로는 함수 주입 형태 사용
	got, err := route(c, "accounts",
		func() (string, error) { return "", official.ErrServer },
		func() (string, error) { return "wts", nil })
	if err != nil || got != "wts" { t.Fatalf("want wts fallback, got %q,%v", got, err) }
	if !strings.Contains(buf.String(), "wts") { t.Fatal("missing fallback notice on stderr") }
}
func TestRouteDoesNotFallbackOnDomainError(t *testing.T) {
	c := &Client{pol: Policy{Prefer: "auto", Fallback: true}, stderr: io.Discard}
	domainErr := &official.APIError{Code: 404}
	_, err := route(c, "quote", func() (string, error) { return "", domainErr }, func() (string, error) { return "wts", nil })
	if !errors.Is(err, domainErr) && err != domainErr { t.Fatalf("want domain error returned, got %v", err) }
}
```
*(주입 형태: `route`가 `c.off`를 직접 안 보고 두 클로저만 받도록 설계해 테스트에서 official 불필요. 오버라이드 메서드가 `c.off==nil` 분기를 담당.)*

- [ ] **Step 2: 실패 확인** — Run: `go test ./internal/hybrid/ -v` · Expected: FAIL.
- [ ] **Step 3: 구현** — `route` 제네릭 + 각 오버라이드 메서드. 소스 노티스는 stderr만.
- [ ] **Step 4: 통과 확인** — Run: `go test ./internal/hybrid/ -v`
- [ ] **Step 5: 커밋** — `git commit -am "feat(hybrid): 임베딩 라우터 + 폴백 구분 + stderr 소스태깅"`

---

## Task 10: hybrid Broker (보수적 주문 폴백)

**Files:**
- Create: `internal/hybrid/broker.go`
- Test: `internal/hybrid/broker_test.go`

**Interfaces:**
- Consumes: `trading.Broker`(WTS 원본이 이미 구현), `*official.Client`.
- Produces: `func (c *Client) Broker() trading.Broker` → 내부 `hybridBroker{wts trading.Broker; off *official.Client; pol Policy; stderr}`.
  - `PlacePendingOrder`: off 없으면 WTS. 있으면 official 시도. **전송 전 실패(ErrTransport 또는 미설정)만** WTS 폴백. **그 외 official 실패는 폴백 금지** — official 에러 그대로(전송 후 모호 실패면 "결과 불명, `tossctl orders` 확인" 래핑).
  - `CancelPendingOrder`/`AmendPendingOrder`: 동일 보수 정책.
  - `GetOrderAvailableActions`: 항상 WTS(공식 대응 없음).

- [ ] **Step 1: 실패 테스트** —

```go
func TestPlaceFallsBackOnlyOnTransportError(t *testing.T) {
	// official Place가 ErrTransport → WTS 폴백 OK
	// official Place가 ErrServer(전송 후) → 폴백 금지, 에러 반환(중복주문 방지)
}
func TestGetAvailableActionsAlwaysWTS(t *testing.T) { /* off 설정돼 있어도 WTS 호출 */ }
```

- [ ] **Step 2: 실패 확인** — Run: `go test ./internal/hybrid/ -run TestPlace -v` · Expected: FAIL.
- [ ] **Step 3: 구현** — `hybridBroker`. 전송 전/후 구분: official 주문 메서드가 요청 전 단계 실패를 `ErrTransport`로만 반환하도록(Task 8에서 `c.post` 호출 *전* 빌더 에러나 미설정은 전송 전; `c.post` 자체 네트워크 에러도 ErrTransport지만 이 경우는 "전송됐을 수 있음"이므로 폴백 금지). → 명확히: **빌더/미설정 실패만 폴백**, 네트워크 포함 그 외 전부 비폴백.
- [ ] **Step 4: 통과 확인** — Run: `go test ./internal/hybrid/ -v`
- [ ] **Step 5: 커밋** — `git commit -am "feat(hybrid): 주문 Broker(보수적 폴백, 중복주문 방지)"`

---

## Task 11: 와이어링 (`newAppContext` + `--backend` 플래그)

**Files:**
- Modify: `cmd/tossctl/root.go`
- Test: `cmd/tossctl/root_test.go`

**Interfaces:**
- Consumes: Task 1·2·4·9·10.
- Produces: `appContext.client *hybrid.Client`(타입 변경); `rootOptions.backend string`; `--backend auto|wts|official`.

- [ ] **Step 1: 실패 테스트** — 자격증명 없을 때 hybrid가 WTS 패스스루로 동작(기존 커맨드 회귀 없음), `--backend wts`가 prefer 오버라이드.
- [ ] **Step 2: 실패 확인** — Run: `go test ./cmd/tossctl/ -run TestRootBackend -v` · Expected: FAIL.
- [ ] **Step 3: 구현** — 스펙 §4.2 의사코드대로: `creds := official.LoadCredentials(os.Getenv, paths.CredentialsFile)`; 조건부 `off`; 항상 `hybrid.New(wtsClient, off, policyFrom(cfg, opts.backend), os.Stderr)`; `appContext.client = h`; `tradingService = trading.NewService(cfg.Trading, h.Broker())`; auth 서비스는 wtsClient 유지. `--backend` 플래그가 `cfg.OpenAPI.Prefer` 오버라이드.
- [ ] **Step 4: 통과 확인** — Run: `go build ./... && go test ./cmd/tossctl/ -v` · Expected: PASS(전체 빌드 포함).
- [ ] **Step 5: 커밋** — `git commit -am "feat(cmd): 하이브리드 와이어링 + --backend 플래그"`

---

## Task 12: `openapi` 커맨드 (login/test/logout)

**Files:**
- Create: `cmd/tossctl/openapi.go`
- Test: `cmd/tossctl/openapi_test.go`

**Interfaces:**
- Consumes: `official.SaveCredentials/DeleteCredentials/LoadCredentials`, `official.New`, `paths`.
- Produces: `tossctl openapi login|test|logout`. 표현 계층은 얇게(플래그/stdin + `internal/output`) — 이후 UX 트랙이 TUI로 감쌀 수 있게 로직과 렌더 분리.

- [ ] **Step 1: 실패 테스트** — `login --key K --secret S`가 0600 파일 생성(프롬프트 없이), `logout`이 삭제, 출력에 시크릿 평문 없음, **非TTY + 플래그 누락이면 프롬프트 없이 에러**(AI 우선 조작성).

```go
func TestOpenAPILoginNonInteractiveErrorsWhenMissingFlags(t *testing.T) {
	// stdin 非TTY, --key/--secret 미지정 → 즉시 에러("provide --key/--secret or run in a terminal"), 입력 대기 없음
}
```

- [ ] **Step 2: 실패 확인** — Run: `go test ./cmd/tossctl/ -run TestOpenAPILogin -v` · Expected: FAIL.
- [ ] **Step 3: 구현** — `login`: `--key/--secret` 있으면 그대로 저장(완전 비대화형). 없을 때 **TTY면** stdin 프롬프트(secret 에코는 `golang.org/x/term` 의존성 금지 → 라인 읽기 + 경고), **非TTY면 에러**(대기 금지). TTY 감지는 `term.IsTerminal` 대신 표준 라이브러리 `os.Stdin.Stat()`의 `ModeCharDevice` 비트로 판정(의존성 없음). `test`: `official.New` + 토큰 + `/accounts`, 결과 분류 메시지(+`--output json`). `logout`: 파일 삭제. 시크릿 절대 출력 안 함.
- [ ] **Step 4: 통과 확인** — Run: `go test ./cmd/tossctl/ -v`
- [ ] **Step 5: 커밋** — `git commit -am "feat(cmd): openapi login/test/logout"`

---

## Task 13: `openapi status` 진단 대시보드

**Files:**
- Modify: `cmd/tossctl/openapi.go`
- Test: `cmd/tossctl/openapi_test.go`

**Interfaces:**
- Consumes: `client.OpenAPIClientInfo/OpenAPIAllowedIPs`(WTS, Task 7), `official.Client`(라이브 프로브 + 토큰 만료), `LoadCredentials`.
- Produces: `tossctl openapi status`(+`--output json`). 스펙 §9.1 표의 항목을 렌더.

- [ ] **Step 1: 실패 테스트** — 더미 키메타 + 가짜 프로브로 (a) 활성/만료임박, (b) 현재 IP가 허용목록에 없을 때 "추가" 안내, (c) WTS 메타 조회 실패해도 프로브 결과는 표시(graceful), (d) json 구조.
- [ ] **Step 2: 실패 확인** — Run: `go test ./cmd/tossctl/ -run TestOpenAPIStatus -v` · Expected: FAIL.
- [ ] **Step 3: 구현** — 자격증명/상태/발급·만료(D-30 경고)/허용IP/현재 공인 IP(프로브 403 단서 우선, 없으면 생략)/토큰 유효/연결 판정/라우팅 요약. 판정 우선순위=라이브 프로브. 표현은 `internal/output`로 분리.
- [ ] **Step 4: 통과 확인** — Run: `go test ./cmd/tossctl/ -v`
- [ ] **Step 5: 커밋** — `git commit -am "feat(cmd): openapi status 진단 대시보드"`

---

## Task 14: `doctor` 하이브리드 통합

**Files:**
- Modify: `cmd/tossctl/doctor.go`
- Test: `cmd/tossctl/doctor_test.go`(또는 기존 doctor 테스트)

**Interfaces:**
- Consumes: Task 13의 진단 로직(재사용 함수로 추출), `LoadCredentials`.
- Produces: doctor 출력에 하이브리드 한 줄(키 있음=요약+문제 시 해결책 / 없음=힌트).

- [ ] **Step 1: 실패 테스트** — 자격증명 있음/없음 분기, 키메타 조회 실패 시 doctor가 실패로 끝나지 않음.
- [ ] **Step 2: 실패 확인** — Run: `go test ./cmd/tossctl/ -run TestDoctor -v` · Expected: FAIL.
- [ ] **Step 3: 구현** — Task 13의 핵심 판정을 공유 함수로 빼고 doctor에서 한 줄 요약 호출. 네트워크 실패는 기존 doctor 타임아웃/관용 정책 따름(전체 비실패).
- [ ] **Step 4: 통과 확인** — Run: `go test ./cmd/tossctl/ -v`
- [ ] **Step 5: 커밋** — `git commit -am "feat(cmd): doctor 하이브리드 진단 통합"`

---

## Task 15: 문서 · 문서 페이지 · 카탈로그 · CHANGELOG

하이브리드 지원을 **docs 사이트(Fumadocs)·README·랜딩**에 충실히 반영한다(사용자 요청).

**Files:**
- Create: `website-fumadocs/content/docs/guide/hybrid-openapi.mdx`(+`.en.mdx`) — 전용 가이드 페이지.
- Modify: `website-fumadocs/content/docs/reference/support-scope.mdx`(+`.en.mdx`), `README.md`, `CHANGELOG.md`, `docs/reverse-engineering/wts-endpoints.json`(Task 7에서 처리됐으면 확인만), 랜딩 `website-fumadocs/app/[lang]/(home)/page.tsx`(해당 시 "선택적 하이브리드" 카피).
- 사이드바/메타(`meta.json` 등) 등록은 기존 docs 구조 관례 따름.

- [ ] **Step 1 — 전용 가이드 페이지** — `guide/hybrid-openapi.mdx`(+en): (1) 하이브리드가 무엇인지(공식이 지원하는 op는 공식 OAuth 경로, 나머지는 WTS, 실패 시 폴백), (2) 공식 Open API 키 발급 방법(토스 설정 > Open API, 허용 IP 등록 필수), (3) **시크릿 안전 취급 — 별도 섹션으로 강조**: 권장 순서 = ① `TOSSCTL_OPENAPI_KEY`/`TOSSCTL_OPENAPI_SECRET` 환경변수(CI·에이전트), ② `tossctl openapi login`이 만드는 `openapi-credentials.json`(0600). **절대 하지 말 것**: 채팅/이슈/PR/커밋/스크린샷/로그에 평문 키·시크릿 붙여넣기, config.json·코드·문서에 하드코딩. 셸 히스토리 회피(`--key/--secret` 직접 입력 대신 env/stdin), 허용 IP를 방어선으로, **유출 의심 시 토스에서 즉시 재발급**. (4) `tossctl openapi login/status/test/logout` 사용법, (5) `--backend`·config `openapi.{enabled,prefer,fallback}`, (6) 폴백·소스 표시(stderr), 주문 보수적 폴백, (7) `status`로 "왜 안 되는지" 진단(IP 미허용·만료 등). 예시는 모두 더미 값(`tsck_live_…`/`tssk_live_…` 형태의 가짜만), 실제 키 절대 금지.
- [ ] **Step 2 — support-scope** — 공식 칼럼이 *실제 라우팅*됨을 반영(주문/조회 행에 하이브리드 주석), 전용 가이드로 링크. `openapi` 커맨드 + `--backend` 문서화.
- [ ] **Step 3 — README** — 비교표/소개에 하이브리드 한 단락 + `tossctl openapi` 빠른 시작, `<!--since:2026-06-27-->` 마커.
- [ ] **Step 4 — 랜딩 카피** — `page.tsx`의 "선택적 하이브리드"/`thesis.points` 문구가 이제 실재 기능과 일치하는지 점검·정정(과장 없이).
  - **카운트 프레이밍(중요)**: 헤드라인 수치를 "고유 18개"가 아니라 **tossctl 전체 역량 = 공식 지원분(tossctl이 100% 커버) + 고유 18 = 합계**로 바꾼다. support-scope 표에서 정확히 카운트(✅공식+❌고유)해 총 N(현재 추정 ~27 기능 영역)을 확정하고, "공식 지원 전부 + 고유 18 = 총 N개" 식으로 표기. "4%"는 전체 WTS(~430) 대비 *공식* 커버리지로만 사용(tossctl이 96%라는 식의 오기 금지). 과장 없이.
- [ ] **Step 5 — CHANGELOG** — `[Unreleased]`에 사용자 관점 항목(공식 키 연결 시 더 안정적, `openapi` 커맨드, status 진단, doctor 통합).
- [ ] **Step 6** — `python3 tools/update_new_markers.py` 실행(있으면).
- [ ] **Step 7 — 검증** — `make build && make test && make lint`(또는 `go build ./... && go test ./... && go vet ./...`). docs 사이트는 `website-fumadocs`에서 빌드 점검(가능 시).
- [ ] **Step 8: 커밋** — `git commit -am "docs: 하이브리드 공식 Open API 가이드 페이지·support-scope·README·랜딩·CHANGELOG"`

---

## 실행 순서 (개정 — 온보딩 프론트로딩)

사용자 우선순위("온보딩 먼저")로 재정렬한다. 의존성상 위저드는 최소 인증 기반 위에 선다:

`Task 1 ✅ → 2 ✅ → 3 → 4 → 5 → 12 → 16 → 17 → 18 → 6 → 7 → 8 → 9 → 10 → 11 → 13 → 14 → 15`

즉 토큰/코어/Accounts/openapi 커맨드(검증·저장 함수) 후 **온보딩 위저드(16–18)**를 먼저 만들고, 그다음 조회 어댑터·주문 라우팅·status/doctor·문서로 간다. Task 12의 저장/검증 로직을 위저드가 재사용한다.

---

## Task 16: TUI 스캐폴딩 (`internal/tui`, huh 도입, TTY 격리)

**Files:** Create `internal/tui/tui.go`, `internal/tui/tui_test.go`; Modify `go.mod`/`go.sum`.

**Interfaces (Produces):**
- `func IsInteractive(in, out *os.File) bool` — 둘 다 char device(`ModeCharDevice`)일 때만 true.
- `var ErrNotInteractive = errors.New("not an interactive terminal")`
- 얇은 래퍼(huh 격리): `func Select(title string, opts []string) (string, error)`, `func Password(title string) (string, error)`, `func Confirm(title string) (bool, error)` — 비TTY면 `ErrNotInteractive`.

- [ ] Step1 실패 테스트: 파이프(비TTY)에서 `IsInteractive`=false, 래퍼는 `ErrNotInteractive`.

```go
func TestIsInteractiveFalseForPipe(t *testing.T) {
	r, _, _ := os.Pipe()
	if IsInteractive(r, os.Stdout) { t.Fatal("pipe stdin must be non-interactive") }
}
func TestPasswordNonInteractiveErrors(t *testing.T) {
	r, _, _ := os.Pipe()
	if _, err := passwordWith(r, os.Stdout, "secret"); !errors.Is(err, ErrNotInteractive) { t.Fatalf("got %v", err) }
}
```
*(테스트 가능하게 내부 `func passwordWith(in,out,title)` 형태로 in/out 주입; 공개 래퍼는 os.Stdin/Stdout 사용. huh 폼 자체는 비TTY에서 안 띄우고 가드만 검증.)*

- [ ] Step2 실패 확인: `go test ./internal/tui/ -v` FAIL.
- [ ] Step3 구현: `go get github.com/charmbracelet/huh`. `IsInteractive`(`fi.Mode()&os.ModeCharDevice != 0` 양쪽). 래퍼는 interactive면 huh 폼 실행, 아니면 `ErrNotInteractive`.
- [ ] Step4 통과: `go test ./internal/tui/ -v` PASS, `go build ./...`.
- [ ] Step5 커밋: `feat(tui): huh 기반 위저드 래퍼 + TTY 격리`

---

## Task 17: 온보딩 결정 로직 (`internal/onboarding`, 순수·테스트 가능)

렌더링과 분리된 순수 로직(위저드 흐름의 두뇌). AI/테스트가 네트워크 없이 검증 가능.

**Files:** Create `internal/onboarding/onboarding.go`, `_test.go`.

**Interfaces (Produces):**
- `type State struct { HasSession, HasOfficialCreds bool }`
- `type Method string` (`MethodWeb="web"`, `MethodOfficial="official"`)
- `func NeedsOnboarding(s State) bool` — 세션·키 둘 다 없으면 true.
- `func AvailableMethods() []Method` — {web, official} (라벨/설명은 cmd에서).
- `func StepsFor(m Method) []string` — official: ["키 입력","시크릿 입력","검증","저장"]; web: ["브라우저 로그인"].

- [ ] Step1 실패 테스트: 상태별 `NeedsOnboarding`, 메서드별 `StepsFor` 테이블.
- [ ] Step2 실패 확인 → Step3 구현(순수 함수) → Step4 통과(`go test ./internal/onboarding/ -v`) → Step5 커밋 `feat(onboarding): 인증 온보딩 결정 로직`.

---

## Task 18: `tossctl init` 온보딩 위저드 + 첫 실행 힌트

**Files:** Create `cmd/tossctl/init.go`, `cmd/tossctl/init_test.go`; Modify `cmd/tossctl/root.go`(첫 실행 힌트).

**Interfaces (Consumes):** Task16 tui, Task17 onboarding, Task12 `official.SaveCredentials`+검증(`openapi test` 로직 재사용), 기존 `auth login`.

흐름(TTY): 인증 방식 선택(`Select`: "웹 세션 로그인 — 넓은 범위·비공식" / "공식 Open API 키 — OAuth2·안정적·정식" / "둘 다") →
- 공식: 키 입력 + `Password`로 시크릿 → 즉시 검증(토큰+/accounts, 결과 분류 — IP 미허용 등 안내) → 0600 저장.
- 웹: 기존 `auth login` 흐름 트리거.
**비TTY/AI**: 위저드 안 띄우고, 플래그 사용법 안내 후 정상 종료(`tossctl auth login`, `tossctl openapi login --key … --secret …`).
**첫 실행 힌트**: `root` PersistentPreRun에서 `NeedsOnboarding && IsInteractive && cmd∉{help,version,init,completion}`이면 1줄 안내("`tossctl init`으로 설정"). 차단하지 않음.

- [ ] Step1 실패 테스트: 비TTY `init`이 위저드 없이 안내 출력+정상 종료; 첫 실행 힌트 조건(순수 부분은 Task17으로 검증, 여기선 cmd 분기).
- [ ] Step2 실패 확인 → Step3 구현(인터랙티브 경로는 수동 검증; 비TTY/힌트 분기는 테스트) → Step4 통과(`go test ./cmd/tossctl/ -v`, `go build`) → Step5 커밋 `feat(cmd): tossctl init 온보딩 위저드 + 첫 실행 힌트`.

---

## Self-Review 메모

- **스펙 커버리지**: §4 패키지(T2–T10), §4.2 와이어링(T11), §4.3 WTS 메타(T7), §5 인증/토큰(T2–T3), §6 라우팅/폴백(T9), §6.3 보수적 주문(T10), §6.4 소스태깅(T9), §7 어댑터(T5–T6), §8 에러(T3), §9 CLI(T12–T13), §9.1 status(T13), §9.2 doctor(T14), §10 설정/경로(T1), §11 테스트(각 Task), §12 문서(T15) — 전 항목 매핑됨.
- **미확정 의존**: 라이브 캡처 없는 부분(WTS 키메타 응답 필드명 T7, 일부 공식 컴포넌트 필드 T6) — 권위 출처(spec/domain) 참조 + 누락 graceful로 처리. 라이브 주문 검증은 비자동.
- **타입 일관성**: hybrid 오버라이드 메서드 시그니처는 WTS `*client.Client` 메서드와 동일해야 임베딩을 가린다(T9). 구현 시 각 WTS 메서드 시그니처를 그대로 복제할 것.
