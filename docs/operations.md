# Operations

운영 측면의 가이드 — API 회귀 감시 cron 설정, 알림 채널 등.

## WTS build + 일반 Toss Android 버전 감시

저장소의 주간 `WTS API Monitor` workflow는 서로 다른 두 접근 경로를 함께 보되 섞지 않습니다.

- **증권 WTS**: build ID, chunk 수, endpoint 추가·삭제를
  `docs/reverse-engineering/wts-endpoints.json`과 비교합니다. build만 바뀌어도 Discord 알림에
  이전/현재 ID를 표시합니다. 수집 결과가 기존 endpoint의 75% 아래로 급감하면 부분 fetch로
  판정해 카탈로그를 덮어쓰지 않습니다.
- **일반 Toss Android 앱**: `viva.republica.toss`의 공개 배포 후보 버전을
  `docs/reverse-engineering/android-app.json`의 마지막 정적 감사본과 비교합니다. Google Play가
  안정적인 기계용 version 필드를 제공하지 않아 APKPure 메타데이터를 **비신뢰 후보 신호**로만
  사용합니다. 후보가 감사본보다 새로우면 `audit_status=stale`로 알리지만, 다운로드·package 및
  서명 연속성 검증·정적 분석 전에는 감사 완료 버전으로 승격하지 않습니다.

```bash
ANDROID_DIFF_OUT=/tmp/android_diff.json python3 tools/android_app_monitor.py
jq . /tmp/android_diff.json
```

`metadata_source`는 실제 네트워크 조회면 `live`, `--metadata-file` fixture를 쓴 재현
테스트면 `offline`입니다. 콘솔과 workflow 알림에도 같은 값을 표시하므로 fixture의 가상
버전이 실제 배포 후보처럼 보이지 않습니다.

이 감시는 모바일 API를 호출하거나 사용자 토큰을 수집하지 않습니다. 앱 버전이 바뀌었다는
사실만 다음 정적 감사의 트리거로 사용합니다.

## API 회귀 감시 (`tossctl monitor api`)

토스 웹 API는 예고 없이 변경됩니다. 과거 두 차례 user-facing 회귀가 있었습니다:

- [#15 / #17](https://github.com/JungHoonGhae/tossinvest-cli/issues/15) — User-Agent 핑거프린팅 차단 (v0.3.6 fix)
- [#29](https://github.com/JungHoonGhae/tossinvest-cli/issues/29) — `/sections/all` body 계약 변경 (v0.4.8 fix)

`monitor api` 명령은 57개 read-only endpoint 를 schema-invariant probe 로 호출해 이런 변경을 사용자보다 먼저 감지합니다.

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

런타임 목록인 `internal/monitor.Probes()` 가 단일 진실 소스입니다. 51개는
`internal/ops` 레지스트리의 오퍼레이션 옆 `ProbeSpec` 에서 파생되고, 카탈로그
오퍼레이션이 없는 CLI 전용 6개만 `internal/monitor/probes.go` 에 직접 선언됩니다.

| 보호 영역 | Probe (개수) |
| --- | --- |
| 계좌·포트폴리오 | `account-list`, `account-summary-overview`, `account-all-overview`, `account-receivable`, `account-interest-years`, `account-commission-info`, `portfolio-positions`, `hidden-holdings` (8) |
| 주문·자금 | `pending-orders`, `order-funding`, `auto-trades` (3) |
| 시세·종목 | `quote-stock-infos`, `quote-trades`, `quote-orderbook`, `quote-price-limits`, `quote-charts`, `quote-reasons`, `quote-crypto`, `quote-stock-signals`, `trading-flows`, `option-expiries` (10) |
| 시장·리서치 | `market-index`, `index-prices`, `stock-ranking`, `investor-rankings`, `theme-rankings`, `sectors-tics`, `sector-detail-simple`, `sector-detail-overview`, `sector-detail-stocks`, `sector-detail-etfs`, `sector-detail-news`, `ai-signals`, `ai-signal-detail`, `screener-presets`, `screener-filter-range`, `earning-call`, `earning-call-home`, `earning-call-detail`, `news-briefing`, `market-news-briefing`, `market-issues`, `market-calendar`, `market-key-events`, `market-halt`, `market-trading-hours` (25) |
| 개인화·계좌 부가기능 | `community-rankings`, `lending-expected`, `lending-top-revenue`, `accumulation-plans`, `profit-overview`, `ria-report`, `open-banking-status`, `notification-settings`, `price-alerts`, `watchlist`, `watchlist-groups` (11) |

이름·method·endpoint 전체 매핑은 [`AGENTS.md`](../AGENTS.md) 의 “Probe 목록”에 있고,
다음 명령으로 실제 런타임 구성을 검증할 수 있습니다.

```bash
go run ./tools/wtsinventory -mode probes -root "$(pwd)" | jq 'length'
# 57
```

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
  ✓ market-index — status=200 (43ms)
  ✓ index-prices — status=200 (53ms)
  … remaining probes …

56 passed, 0 failed
```

실패 (예: #29 같은 body-contract 회귀):

```
  ✗ portfolio-positions — status=200: result.sections is empty — likely body-contract regression (#29-class)
… 55 passing probes omitted …

55 passed, 1 failed
```

webhook 페이로드:

```
🚨 tossctl API regression detected (0.4.9)
2026-05-13 10:00 UTC — 1/57 probes failed

❌ portfolio-positions — POST wts-cert-api.tossinvest.com/api/v2/dashboard/asset/sections/all
    status=200, result.sections is empty — likely body-contract regression (#29-class)
```

### 새 probe 추가

새 read-only endpoint 의존이 카탈로그 오퍼레이션에 연결되면 해당 `internal/ops`
항목에 `ProbeSpec` 을 같이 선언합니다. 여러 HTTP 요청을 합치는 오퍼레이션이면 첫
의존성은 `Probe`, 나머지는 `ExtraProbes`에 선언해 모든 요청 계약을 감시합니다.
오퍼레이션·probe 소유권이 한 곳에 남아 MCP, CLI, monitor 계약이 같이 바뀔 수 있습니다.

```go
Probe: &ProbeSpec{
    Name:   "new-endpoint", Method: "POST",
    URL:    probeCert + "/api/v2/...",
    Body:   `{"types":["..."]}`,
    Check: func(status int, body []byte) error {
        if err := ExpectStatus(status, 200); err != nil {
            return err
        }
        return ExpectPath(body, "result.someKey", "array")
    },
},
```

카탈로그 오퍼레이션이 없는 CLI 직접 표면은 예외적으로
`internal/monitor/probes.go` 에 넣습니다. 새 Check 함수는 schema 진단 메시지만 반환하면
됩니다. `ExpectStatus` / `ExpectPath` 가 기본 패턴입니다.
