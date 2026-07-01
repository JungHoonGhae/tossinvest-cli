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
- `mutating: true`: the command changes account state (live trading). Only
  `order place`, `order cancel`, `order amend` carry this.

Rules for agents:

- **Never auto-invoke `mutating: true` commands.** A human must approve every
  live order. Trading is also gated in `config.json` (disabled by default).
- **Preview before placing.** Use `order preview` to validate a canonical order
  intent, then let a human run `order place`.
- Prefer `--json` for machine-readable output.
- Treat `source: wts` results as best-effort; add a `monitor api` probe when you
  build automation on top of them.

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

현재 감시 중인 endpoint (`internal/monitor/probes.go` 의 `Probes()` 슬라이스):

- `account-list` — `GET /api/v1/account/list`
- `account-summary-overview` — `GET /api/v3/my-assets/summaries/markets/all/overview`
- `portfolio-positions` — `POST /api/v2/dashboard/asset/sections/all` (`SORTED_OVERVIEW`)
- `watchlist` — `POST /api/v2/dashboard/asset/sections/all` (`WATCHLIST`)
- `quote-stock-infos` — `GET /api/v2/stock-infos/A005930`
- `pending-orders` — `GET /api/v1/trading/orders/histories/all/pending`

새 endpoint 의존이 생기면 `internal/monitor/probes.go` 에 항목 추가. 가이드: `docs/operations.md`.
