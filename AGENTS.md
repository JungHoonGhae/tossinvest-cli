# Agents

`tossctl` 자동화를 셋업하려는 AI 에이전트 (OpenClaw / Claude Code / Codex / Cursor / 기타) 가 참고할 짧은 recipe.

## 전제

```bash
tossctl version          # 0.4.9+
tossctl auth status      # Session: active / Live Check: valid 여야 함
```

`auth status` 가 active 가 아니면 사용자가 직접 `tossctl auth login` 으로 QR + 폰 2차 인증을 마쳐야 합니다 (에이전트가 대신 못 함).

## Command taxonomy & safety

Every leaf command carries machine-readable annotations:

- `source`: `official` (official Open API only), `wts` (WTS internal endpoint only),
  or `both` (official preferred, WTS fallback). `wts` endpoints are unofficial and
  may change without notice.
- `mutating: true`: the command can change account state (live trading). It is
  attached to `order place`, `order cancel`, `order amend`, `order conditional
  place|cancel|modify`, and `ops call` (because that generic dispatcher can
  invoke write operations).

Rules for agents:

- **Never auto-invoke `mutating: true` commands.** A human must approve every
  live order. Trading is also gated in `config.json` (disabled by default).
- **Preview before placing.** Use `order preview` to validate a canonical order
  intent, then let a human run `order place`.
- Open API IP 교체도 기본은 preview입니다. `openapi ip replace-current` 결과의
  `confirm_token`을 검토한 뒤에만 `--execute --confirm <token>`으로 실행하세요.
  이 작업은 주문을 만들지 않으며, 새 IP 추가 실패 시 기존 목록 복구를 시도합니다.
- Prefer `--json` for machine-readable output.
- Treat `source: wts` results as best-effort; add a `monitor api` probe when you
  build automation on top of them.
- **Don't auto-run `tossctl update`.** It changes the tossctl binary itself
  (not account state), but an agent silently switching versions mid-task can
  surprise the human running it — let a human trigger it, or run
  `tossctl update --check` (read-only) if you need to know whether an update
  exists.

## API 회귀 감지 → 알림

`monitor api` 는 exit 0/1 만 반환합니다. 알림 채널은 cron 라인의 `||` 우항에서 자유롭게 합성합니다.

```cron
# Discord
0 * * * * /usr/local/bin/tossctl monitor api --quiet || \
  curl -sS -X POST -H 'Content-Type: application/json' \
    -d '{"content":"⚠️ tossctl regression"}' \
    "$YOUR_DISCORD_WEBHOOK"

# Slack
0 * * * * /usr/local/bin/tossctl monitor api --quiet || \
  curl -sS -X POST -H 'Content-Type: application/json' \
    -d '{"text":"⚠️ tossctl regression"}' \
    "$YOUR_SLACK_WEBHOOK"

# ntfy.sh (push to phone, no signup)
0 * * * * /usr/local/bin/tossctl monitor api --quiet || \
  curl -sS -d "tossctl regression" "https://ntfy.sh/$YOUR_NTFY_TOPIC"

# macOS native notification (when the cron runs on your laptop)
0 * * * * /usr/local/bin/tossctl monitor api --quiet || \
  osascript -e 'display notification "tossctl regression" with title "tossctl"'

# Email (requires mailx/msmtp configured)
0 * * * * /usr/local/bin/tossctl monitor api --quiet || \
  echo "tossctl regression $(date)" | mail -s "tossctl regression" you@example.com
```

### 에이전트가 해야 할 일

1. `crontab -e` 를 열어 위 라인 중 하나를 사용자 환경에 맞게 추가.
2. 사용자 머신의 실제 `tossctl` 경로 확인 (`which tossctl`) 후 라인 경로 교체.
3. webhook URL · ntfy topic · 이메일 주소 등 user-specific 값은 사용자에게 묻거나 환경변수로 분리.
4. 설치 직후 한 번 `tossctl monitor api` 수동 실행해서 통과 확인.
5. (선택) 실패 시뮬레이션: 본인 머신에서 `monitor api && false` 같은 변형을 한 번 시험 발사해 알림 채널 도달 확인.

## Probe 목록

현재 `monitor api` 는 54개 read-only endpoint 를 감시합니다. 단일 진실 소스는
`internal/monitor.Probes()` 런타임 결과입니다. 대부분은 `internal/ops`
레지스트리의 오퍼레이션 옆 `ProbeSpec` 에서 파생되고, 카탈로그 오퍼레이션이
없는 CLI 전용 6개만 `internal/monitor/probes.go` 에 직접 선언됩니다.

- `market-index` — `GET /api/v1/dashboard/wts/overview/indicator/index`
- `index-prices` — `GET /api/v1/index-prices/KGG01P`
- `stock-ranking` — `GET /api/v1/rankings/realtime/stock`
- `investor-rankings` — `GET /api/v1/dashboard/wts/overview/rankings/by-investors`
- `theme-rankings` — `GET /api/v1/tics/rankings`
- `sectors-tics` — `GET /api/v1/tics/all`
- `sector-detail-overview` — `GET /api/v2/dashboard/wts/overview/tics/1/overview`
- `sector-detail-stocks` — `POST /api/v2/dashboard/wts/overview/tics/1/stocks`
- `sector-detail-etfs` — `POST /api/v2/dashboard/wts/overview/tics/1/etfs`
- `sector-detail-news` — `GET /api/v2/dashboard/wts/overview/tics/1/news`
- `ai-signals` — `GET /api/v2/reasoning-contents/interest`
- `screener-presets` — `GET /api/v2/screener/presets/common`
- `trading-flows` — `GET /api/v1/stock-infos/trade/trend/trading-trend`
- `earning-call` — `GET /api/v1/earning-call/upcoming`
- `news-briefing` — `GET /api/v2/reasoning/personalized`
- `market-news-briefing` — `GET /api/v1/dashboard/wts/overview/ai-signals/latest?nationCode=KOR`
- `community-rankings` — `GET /api/v1/community/top-rankings/INFLUENCER`
- `lending-expected` — `GET /api/v1/lending/revenue/account/expected`
- `lending-top-revenue` — `GET /api/v1/lending/revenue/account/top-revenue`
- `accumulation-plans` — `GET /api/v2/autotrade/plan/find`
- `profit-overview` — `POST /api/v1/profit/overview`
- `market-issues` — `GET /api/v1/lens/issues`
- `auto-trades` — `GET /api/v3/trading/auto-trading/histories`
- `market-calendar` — `POST /api/v4/calendar/monthly/<YYYY-MM>`
- `market-key-events` — `GET /api/v1/calendar/ai-summary/key-events`
- `quote-charts` — `POST /api/v1/dashboard/common/stocks/mini-chart`
- `quote-reasons` — `POST /api/v1/dashboard/wts/overview/ai-signals`
- `market-halt` — `GET /api/v4/dashboard/wts/overview/indicator`
- `quote-crypto` — `GET /api/v1/crypto-prices`
- `quote-stock-signals` — `GET /api/v1/dashboard/wts/overview/signals`
- `account-receivable` — `GET /api/v1/margin/cert/notice/receivable`
- `screener-filter-range` — `POST /api/v1/screener/filters/range`
- `option-expiries` — `GET /api/v1/option-maturity-date/get-all`
- `order-funding` — `GET /api/v2/trading/order/buy-control/required-deposit-amount`
- `ria-report` — `GET /api/v1/ria-calculator/report`
- `account-interest-years` — `GET /api/v1/interest/accounts/annual/history/years`
- `account-commission-info` — `GET /api/v2/trading/commission-info`
- `account-summary-overview` — `GET /api/v3/my-assets/summaries/markets/all/overview`
- `account-all-overview` — `POST /api/v1/dashboard/all-accounts`
- `open-banking-status` — `GET /api/v1/autotrade/open-banking/info/find`
- `notification-settings` — `GET /api/v1/user-alimies`
- `price-alerts` — `GET /api/v1/user-price-alimy/A005930`
- `hidden-holdings` — `GET /api/v2/hidden-stocks`
- `portfolio-positions` — `POST /api/v2/dashboard/asset/sections/all`
- `pending-orders` — `GET /api/v1/trading/orders/histories/all/pending`
- `watchlist` — `GET /api/v1/new-watchlists`
- `watchlist-groups` — `GET /api/v1/new-watchlists/groups/simple`
- `earning-call-home` — `GET /api/v1/earning-call/home`
- `account-list` — `GET /api/v1/account/list`
- `quote-stock-infos` — `GET /api/v2/stock-infos/A005930`
- `quote-trades` — `GET /api/v2/stock-prices/A005930/ticks`
- `quote-orderbook` — `GET /api/v3/stock-prices/A005930/quotes`
- `quote-price-limits` — `GET /api/v2/stock-prices/A005930/upper-lower`
- `market-trading-hours` — `GET /api/v2/system/trading-hours/integrated`

새 카탈로그 endpoint 의존이 생기면 해당 `internal/ops` 오퍼레이션에 `ProbeSpec`을
붙입니다. 한 기능이 여러 endpoint를 합치면 나머지는 `ExtraProbes`에 모두 선언합니다.
CLI 전용 의존성만 `internal/monitor/probes.go` 에 추가합니다. 가이드:
`docs/operations.md`.
