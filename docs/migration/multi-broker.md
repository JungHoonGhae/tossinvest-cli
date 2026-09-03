# Multi-Broker 추상화 방향 (보류 — future option)

> **🟡 status: 보류 (2026-06-03).** 현재 채택 전략이 **아니다.** 지금 방향은
> "tossctl 단일 레포 + web 주력 + 공식 API 보완 fallback" ([open-api.md](./open-api.md)
> "채택 전략" 참조). 별도 멀티-브로커 레포(investctl)는 **"별도 사유가 생기기 전까지"
> 착수하지 않는다.** 이 문서는 그 사유가 생겼을 때 꺼내볼 **future-option anchor** 로만
> 보존한다.
>
> 보류 이유: ① 공식 API 가 좁아 별도 제품화 동인이 약함 ② 공식의 실익(무인 자가갱신
> 토큰 + 안정성 fallback)은 별도 레포가 아니라 tossctl 안 하이브리드로 흡수하는 게
> 맞음. ③ KIS 통합 등 cross-broker 의 분명한 수요/사유가 아직 없음.
>
> **언제 이 문서를 다시 꺼내나 (트리거):** 토스 비공식 endpoint 가 차단돼 web 고유 표면이
> 무너질 때 / KIS·키움 등 다른 증권사 통합의 구체적 수요가 생길 때 / 공식 토큰만으로도
> 충분한 사용자층이 확인될 때.
>
> 아래 설계는 그 시점 기준의 초안이다 (별도 레포 `oss-kr-investctl`, 공식 API only).
> 다른 세션의 `oss-koreainvestment-cli` KIS MVP 스펙과 동일 설계.

목표: **"한국 증권사를 AI 에이전트에 통일된 인터페이스로 연결하는 공식 API CLI."**
KIS(한국투자) 공식 API 부터, 토스 공식(승인 후), 키움·미래에셋 등으로 확장.

> **status:** 설계 단계 (코드 착수 전) · 마지막 업데이트 2026-06-03
>
> **⚠ 계획·인터페이스·우선순위는 사전 공지 없이 바뀔 수 있습니다.** 실제 백엔드
> (KIS 키, 토스 공식 토큰)를 손에 쥐기 전엔 추상화를 확정하지 않는다 — 표면을 보기
> 전 설계는 잘못 잡을 확률이 크다. 여기 적힌 건 "현 시점 stance".
>
> **참고:** 다른 세션에서 `oss-koreainvestment-cli` 에 이미 KIS MVP 스펙/계획을
> 작성해 둠 (rclone/terraform-provider식 core+adapter+capability). 이 문서와
> 그 스펙은 **동일 설계의 두 사본** — 구현 착수 시 하나로 통합 (레포명
> `oss-kr-investctl` 로 정리).

## 왜 multi-broker 인가

토스가 공식 Open API (`developers.tossinvest.com`, issue #31) 를 내면서 단일
백엔드 의존의 위험이 드러났다:

- 토스가 `llms.txt` 까지 제공 = **AI 친화 레이어를 직접 노림**. 우리 강점의 정면.
- 토스 하나에만 묶이면 토스 공식이 우리를 흡수할 수 있다.
- "한국 증권 통합 AI 인터페이스"가 되면 **어느 한 증권사도 우리를 대체 못 한다.**

핵심 통찰: 우리 가치는 "토스에 접근하는 것"이 아니라 **"여러 증권사를 AI·사람이
가장 쉽게 쓰게 만드는 레이어 + 공식이 안 주는 데이터"** 다. 백엔드는 교체 가능한
부품이어야 한다.

## 목표 아키텍처

```
        cmd/investctl (명령 레이어 — 백엔드 무관, 단일 바이너리)
                     │
                     ▼
            internal/broker.Broker  (interface)
            ┌────────────┬──────────────┐
            ▼            ▼              ▼
   KISBroker     TossOfficialBroker  KiwoomBroker
   (한투 공식)   (토스 공식 OAuth2)  (키움 공식)
        │              │
   apiportal.kis   openapi.tossinvest
```

- **공식 API only.** 비공식 web session(tossctl 의 wts-*) 은 여기 들어오지 않는다 —
  그건 tossctl 레포에 영구히 남는 별도 제품.
- **명령 레이어는 broker 를 모른다.** `investctl portfolio positions` 는 어떤
  공식 백엔드든 동일하게 동작.
- 백엔드는 config 또는 `--broker` 플래그로 선택. 미지정 시 default broker.
- 첫 구현체는 **KIS** (지금 바로 가능 — 공식 운영 중). 토스 공식은 승인 후.

## Broker interface (초안 — 미확정)

tossctl 의 `internal/client.Client` 공개 메서드를 *참고*해 도출 (그대로 복사 아님 —
investctl 은 공식 API 표면에 맞춤). 모든 백엔드가 전부 구현할 필요는 없다 —
미지원은 `ErrUnsupported` 를 반환하고 명령 레이어가 친절히 처리.

```go
// internal/broker/broker.go (제안)
type Broker interface {
    // 조회 (대부분 백엔드 공통)
    ListAccounts(ctx) ([]domain.Account, error)
    ListPositions(ctx, market) ([]domain.Position, error)
    GetQuote(ctx, symbol) (domain.Quote, error)
    GetChart(ctx, symbol, interval, count) (domain.Chart, error)
    ListPendingOrders(ctx) ([]domain.Order, error)
    ListCompletedOrders(ctx, market) ([]domain.Order, error)

    // 거래 (권한 게이트는 명령 레이어가 유지)
    PlaceOrder(ctx, intent) (domain.OrderResult, error)
    CancelOrder(ctx, id) error
    AmendOrder(ctx, id, changes) error

    // 메타 — 백엔드 능력 선언
    Capabilities() BrokerCapabilities
}

type BrokerCapabilities struct {
    Name        string   // "toss-session" | "toss-official" | "kis" | ...
    Markets     []string // "kr", "us"
    Trading     bool
    Realtime    bool     // push/SSE 지원
    Extras      []string // "transactions", "watchlist", "warnings" 등 비표준 표면
}
```

**`Capabilities()` 가 핵심.** 백엔드마다 표면이 다르므로 (공식 API 엔 watchlist
없음, KIS 엔 또 다른 것), 명령 레이어가 능력을 질의해 미지원 기능은 명확한
메시지로 막는다. (예: `quote limits` 가 US 종목을 거부하는 현재 패턴과 동일 철학)

## 백엔드별 매핑 (현재까지 파악)

investctl 은 공식 열만 다룬다. 맨 왼쪽 tossctl(비공식) 열은 *대조용* — investctl 에
없는 것이 곧 tossctl 의 고유 범위임을 보여준다.

| 기능 | tossctl(비공식, 別제품) | toss-official | KIS / 키움 |
|---|---|---|---|
| 계좌·보유·주문·시세 | ✅ | ✅ (OAuth2) | ✅ (각자 OAuth/키) |
| transactions ledger | ✅ | ❌ (공식 표면 없음) | 미확인 |
| watchlist | ✅ | ❌ | 미확인 |
| push (실시간) | ✅ (SSE) | ✅ (`stream`, 웹소켓 — 체결·호가·내 주문) | KIS 는 WebSocket 있음 |
| 매수유의·상하한가·체결·환율·장운영 | ✅ | ✅ | 미확인 |
| 선물옵션·해외·채권 | ❌ | ❌ | ✅ (KIS) |
| rate limit | 없음 | 있음 (ACCOUNT 1/s 등) | 있음 |
| 진입 마찰 | `auth login` 1회 | 사전신청→승인→토큰 | 계좌+키 발급 |

→ tossctl 과 investctl 은 표면이 상보적. tossctl=비공식 넓음, investctl=공식 통합+
선물옵션/해외/채권 같은 공식만의 깊이. 자기잠식 없음.

## 단계적 도입

1. **MVP (지금 — KIS 만으로 가능)** — `core`(auth/output/config) + `domain` 정규화 +
   `Broker` interface + `registry` + **KIS read-only adapter**(quote/account/portfolio/
   chart). 단일 바이너리 `investctl --broker kis`. 이미 KIS 공식 API 가 운영 중이라
   토스 승인을 기다릴 필요 없이 *지금* 아키텍처를 끝까지 검증 가능.
2. **거래** — KIS order preview/place/cancel + permission 게이트 (tossctl 의 안전
   모델 이식).
3. **토스 공식 adapter** — 승인 후 `--broker toss`. KIS 로 검증된 interface 에 끼움.
4. **확장** — 키움·미래에셋·NH. 새 증권사 = 폴더 1개 + `registry.Register` 1줄.

## 설계 원칙

- **명령 레이어는 절대 백엔드를 직접 알지 않는다.** broker interface 만 의존.
- **미지원은 에러로, 침묵하지 않는다.** `Capabilities()` + `ErrUnsupported`.
- **거래 안전 게이트(config·permission)는 명령 레이어에 유지** — 백엔드로 내리지
  않는다. 어떤 백엔드든 동일한 안전 모델.
- **AI ergonomics 레이어(JSON/CSV, AGENTS.md, monitor)는 백엔드 무관 공통자산** —
  이게 차별점이므로 broker 와 직교하게 유지.
- **interface 는 두 번째 구현이 생길 때 확정.** MVP 는 KIS 하나로 짜되, 토스 공식이
  두 번째로 붙을 때 어색한 부분을 리팩토링 ([YAGNI](https://martinfowler.com/bliki/Yagni.html)).
  단 investctl 은 처음부터 cross-broker 가 목적이라 interface 골격은 1일차부터 둔다
  (tossctl 처럼 단일 구현으로 출발한 게 아님).
- **core 는 tossctl 에서 copy-first.** 공유 Go 모듈화는 두 레포가 패턴을 검증한 뒤.

## 위험·미해결

1. **토스 공식의 AI 레이어 확장** — `llms.txt` 가 신호. 토스가 공식 SDK + agent
   통합까지 내면 ergonomics 강점 약화. → 단일 증권사가 아닌 cross-broker 통합이
   방어막 (어느 한 증권사도 "전부 통합"을 대체 못 함).
2. **KIS 공식 MCP 가 이미 존재** — KIS 단독 가치는 약함. investctl 의 가치는 *통합*
   에서만 나옴 — 단일 broker 로는 정당화 안 됨을 명심.
3. **각 증권사 ToS** — 공식 API 는 합법이나 약관·계좌 요건 상이. broker 별 문서화.
4. **두 레포 core 중복 유지비** — copy-first 의 대가. 공유 모듈화는 검증 후.
5. **거래 권한 모델 차이** — KIS 모의/실전 분리, 토스 scope 별도 신청 가능성 →
   Capabilities 에 반영.

## 결정 log

각 항목은 *그 시점의* stance. 뒤집힐 때 사전 공지 없이 새 항목만 추가.

- **2026-06-03a** — multi-broker 를 별도 레포 investctl(공식 only)로 잠정 결정.
  MVP = KIS read-only. (아래 06-03b 에서 보류로 정정됨.)
- **2026-06-03b** — **보류 결정.** 현재 채택 전략을 "tossctl 단일 레포 + web 주력 +
  공식 보완 fallback" 으로 확정 (open-api.md 06-03b). 공식의 실익(무인 자가갱신 토큰·
  안정성 fallback)은 별도 레포가 아니라 tossctl 안 하이브리드로 흡수가 맞고, KIS 등
  cross-broker 의 구체적 사유가 아직 없음. 이 문서는 future-option anchor 로 보존 —
  상단 트리거 조건이 충족되면 재개. 그때 다른 세션 `oss-koreainvestment-cli` KIS
  스펙과 통합, 레포명 `oss-kr-investctl`.
