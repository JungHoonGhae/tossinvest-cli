# Operations

운영 측면의 가이드 — API 회귀 감시 cron 설정, 알림 채널 등.

## API 회귀 감시 (`tossctl monitor api`)

토스 웹 API는 예고 없이 변경됩니다. 과거 두 차례 user-facing 회귀가 있었습니다:

- [#15 / #17](https://github.com/JungHoonGhae/tossinvest-cli/issues/15) — User-Agent 핑거프린팅 차단 (v0.3.6 fix)
- [#29](https://github.com/JungHoonGhae/tossinvest-cli/issues/29) — `/sections/all` body 계약 변경 (v0.4.8 fix)

`monitor api` 명령은 6개 read-only endpoint 를 schema-invariant probe 로 호출해 이런 변경을 사용자보다 먼저 감지합니다.

### 동작 흐름

```
[당신 머신: tossctl monitor api]
       ↓ (당신 세션 쿠키)
[토스 서버] ← 본인 계좌 조회
       ↓ (응답)
[당신 머신: 응답 schema 체크]
       ↓ (실패 시만)
[당신이 설정한 Discord webhook] → 당신 채널
```

`monitor api` 는 본인 머신에서만 실행되며, 본인 세션으로 본인 계좌만 조회합니다. webhook URL 은 코드에 기본값이 없어 사용자가 직접 설정합니다.

### Probe 목록

| 이름 | Endpoint | 보호하는 명령 |
| --- | --- | --- |
| `account-list` | `GET /api/v1/account/list` | `account list` |
| `account-summary-overview` | `GET /api/v3/my-assets/summaries/markets/all/overview` | `account summary` |
| `portfolio-positions` | `POST /api/v2/dashboard/asset/sections/all` (`SORTED_OVERVIEW`) | `portfolio positions` |
| `watchlist` | `GET /api/v1/new-watchlists` | `watchlist list` |
| `watchlist-groups` | `GET /api/v1/new-watchlists/groups/simple` | `watchlist groups` |
| `quote-stock-infos` | `GET /api/v2/stock-infos/A005930` | `quote get` |
| `pending-orders` | `GET /api/v1/trading/orders/histories/all/pending` | `orders list` |

각 probe 는 status 200 + 핵심 JSON 경로 존재 + 타입을 검사합니다. Toss 가 새 필드를 추가하는 변경은 통과시키고, 핵심 필드가 사라지거나 빈 응답을 받으면 실패합니다.

### Cron + 알림 합성

`monitor api` 는 exit 0/1 만 반환합니다. 알림 채널은 cron 라인의 `||` 우항에서 사용자가 자유롭게 합성합니다. `crontab -e`:

```cron
# 매시간 정각, 실패 시 Discord 알림
0 * * * * /usr/local/bin/tossctl monitor api --quiet || \
  curl -sS -X POST -H 'Content-Type: application/json' \
    -d '{"content":"⚠️ tossctl regression"}' \
    'https://discord.com/api/webhooks/...'
```

Discord 외 Slack · ntfy · macOS notification · 이메일 등 다른 채널 합성 예시는 [`AGENTS.md`](../AGENTS.md). launchd · systemd timer 등 다른 스케줄러도 동일하게 동작합니다 (exit code 기반).

### 출력 예시

정상 (모든 probe 통과):

```
  ✓ account-list — status=200 (43ms)
  ✓ account-summary-overview — status=200 (53ms)
  ✓ portfolio-positions — status=200 (52ms)
  ✓ watchlist — status=200 (16ms)
  ✓ quote-stock-infos — status=200 (44ms)
  ✓ pending-orders — status=200 (19ms)

6 passed, 0 failed
```

실패 (예: #29 같은 body-contract 회귀):

```
  ✓ account-list — status=200 (43ms)
  ✓ account-summary-overview — status=200 (53ms)
  ✓ watchlist — status=200 (15ms)
  ✓ quote-stock-infos — status=200 (44ms)
  ✓ pending-orders — status=200 (19ms)
  ✗ portfolio-positions — status=200: result.sections is empty — likely body-contract regression (#29-class)

5 passed, 1 failed
```

webhook 페이로드:

```
🚨 tossctl API regression detected (0.4.9)
2026-05-13 10:00 UTC — 1/6 probes failed

❌ portfolio-positions — POST wts-cert-api.tossinvest.com/api/v2/dashboard/asset/sections/all
    status=200, result.sections is empty — likely body-contract regression (#29-class)
```

### 새 probe 추가

새 read-only endpoint 의존이 생기면 `internal/monitor/probes.go` 의 `Probes()` 반환 슬라이스에 항목 추가:

```go
{
    Name:   "new-endpoint",
    Method: "POST",
    URL:    cert + "/api/v2/...",
    Body:   `{"types":["..."]}`,
    Check: func(status int, body []byte) error {
        if err := expectStatus(status, body, 200); err != nil {
            return err
        }
        return expectPath(body, "result.someKey", "array")
    },
},
```

새 Check 함수는 schema 진단 메시지만 반환하면 됩니다 — `expectStatus` / `expectPath` 가 기본 패턴.
