# Configuration

`tossctl`은 기본적으로 조회 중심으로 동작합니다. 거래 기능은 설치 직후 바로 열리지 않고, 사용자가 로컬 `config.json`에서 기능별로 직접 허용해야만 사용할 수 있습니다.

## 기본 경로

- config dir: `$(os.UserConfigDir)/tossctl`
- config file: `<config dir>/config.json`

먼저 현재 설정을 확인하거나 기본 파일을 만들 수 있습니다.

```bash
tossctl config show
tossctl config init
```

## 기본 설정

파일이 없더라도 CLI는 동작하지만, 이 경우 거래 기능은 모두 비활성으로 간주됩니다.

`tossctl config init`으로 생성되는 기본 파일은 아래와 같습니다.

```json
{
  "$schema": "https://raw.githubusercontent.com/JungHoonGhae/tossinvest-cli/main/schemas/config.schema.json",
  "schema_version": 5,
  "trading": {
    "place": false,
    "sell": false,
    "fractional": false,
    "cancel": false,
    "amend": false,
    "conditional": false,
    "allow_live_order_actions": false,
    "dangerous_automation": {
      "accept_fx_consent": false
    }
  },
  "update_check": {
    "enabled": true
  },
  "openapi": {
    "enabled": true,
    "prefer": "auto",
    "fallback": true
  },
  "experimental": {
    "paper_trading": false
  }
}
```

## 필드 설명

토글은 두 종류로 나뉩니다.

**경로 게이트 (broker API 분기별)**
- `trading.place` — `tossctl order place` 허용 여부
- `trading.cancel` — `tossctl order cancel` 허용 여부
- `trading.amend` — `tossctl order amend` 허용 여부
- `trading.conditional` — `tossctl order conditional place|cancel|modify` 및 대응 ops/MCP 쓰기 허용 여부

**스코프 선언 (유저 자가 제한)**
- `trading.sell` — `tossctl order place --side sell` 허용 여부. `trading.place`도 함께 켜야 합니다. 끄면 매수만 가능
- `trading.fractional` — `tossctl order place --fractional --amount <KRW>` 허용 여부. US 시장가 주문으로만 지원. `trading.place`도 함께 켜야 합니다

> 시장(US/KR)은 게이트가 아닙니다. `v0.5.2`에서 비대칭이던 `trading.kr` 토글을 제거했습니다 — KR 주문이 US 주문보다 위험하지 않으므로 시장은 대칭 취급하며, `trading.place` + `allow_live_order_actions` 가 양쪽을 동일하게 게이트합니다. KR 6자리 종목코드는 `--market kr` 없이도 자동 인식됩니다.

**마스터 / 자동화**
- `trading.allow_live_order_actions` — 실계좌에 도달하는 일반·조건 주문 액션(`place`, `cancel`, `amend`, `conditional`) 자체를 허용. **마스터 킬스위치**로 위 경로 게이트가 켜져 있어도 이 값이 false면 broker에 닿지 않음
- `trading.dangerous_automation.accept_fx_consent` — post-prepare FX confirmation branch를 자동 수락하고 같은 주문을 계속 진행하도록 허용. 현재는 `prepare` 성공 후 `needExchange > 0`인 미국주식 KRW 매수 경로에만 연결됨

**실험 기능**

- `experimental.paper_trading` — 롤아웃 중인 미국 옵션 모의투자 명령과 ops/MCP
  오퍼레이션, 전용 회귀 probe를 노출합니다. 기본값은 `false`입니다. 다음 명령으로만
  켜거나 끕니다.

  ```bash
  tossctl config experimental paper-trading --enable
  tossctl config experimental paper-trading --disable
  ```

  모의 원장 쓰기는 실거래 config와 별개로 기본 preview 뒤 `--execute`를 요구합니다.
  이 설정은 실계좌 주문 권한을 부여하지 않습니다. 상류 계약이 최소 3개 연속 build와
  7일·7회 probe에서 안정되고 공식 UI 일반 공개·상태 일관성·미해결 5xx 없음까지 확인될
  때만 stable로 승격합니다.

즉, 각 액션은 config에서 먼저 열려 있어야 하고, 그 다음에도 런타임 게이트(`--execute` → `--confirm <token>`)를 통과해야 합니다.

## 실행 순서

거래 mutation이 실제로 실행되려면 아래 순서를 모두 만족해야 합니다.

1. `config.json`에서 해당 액션 허용
   - live mutation은 `trading.allow_live_order_actions=true`도 필요
2. `--execute`
3. `--confirm <token>` (`order preview`에서 받은 주문별 토큰)

> v0.5.x에서 중복이던 `--dangerously-skip-permissions` 게이트는 은퇴했습니다(거짓 이름 + `--execute`와 의미 중복). 기존 플래그는 한 릴리즈 동안 deprecated no-op으로 받아들여 호환을 유지합니다.

## 로컬 파일 권한

tossctl이 저장하는 모든 상태 파일과 디렉토리는 소유자 전용 권한으로 관리됩니다.

| 대상 | 모드 | 내용 |
|---|---|---|
| `~/Library/Application Support/tossctl/` (macOS) / `~/.config/tossctl/` (Linux) | `0o700` | 디렉토리 자체 (다른 로컬 사용자의 목록 조회 차단) |
| `session.json` | `0o600` | 전체 쿠키·localStorage 포함 세션 |
| `config.json` | `0o600` | 거래 허용 플래그 |
| `trading-lineage.json` | `0o600` | order ref 추적 |
| `~/Library/Caches/tossctl/auth/playwright-storage-state.json` (로그인 중간 산출물) | `0o600` | 로그인 성공 직후 자동 삭제 (v0.4.1+) |
| `--link` / `--headless` 로그인 URL | stderr에만 출력 | 일회성 URL이므로 공개 로그에 남기지 않음 |
| `--qr-output <path>` PNG (link/headless 로그인) | `0o600` | `fchmod`로 기존 파일도 강제 |

**진단:** `tossctl doctor --report` 의 `file_modes` 항목에서 각 파일/디렉토리의 실제 모드와 기대 모드를 JSON으로 확인할 수 있습니다. `ok: false`로 표시되는 항목은 보통 v0.4.0 이전에 생성된 디렉토리(0o755)라, `chmod 0700 ~/Library/Application\ Support/tossctl` 한 번이면 정리됩니다. 기능에는 영향 없음.

## Legacy Compatibility

기존 `schema_version: 1` 파일과 `trading.allow_dangerous_execute`는 계속 읽을 수 있습니다.

`v0.4.3`에서 제거된 필드(`trading.grant`, `trading.dangerous_automation.complete_trade_auth`, `trading.dangerous_automation.accept_product_ack`)와 `v0.5.2`에서 제거된 `trading.kr`(비대칭 시장 게이트)는 설정에 남아있어도 무시됩니다. legacy 필드(또는 구버전 스키마)가 남아 있으면 일반 명령 실행 시 stderr 경고 1줄(24h backoff)로 안내되고, `config status`/`doctor`의 `legacy_config` 체크에서도 감지해 알려줍니다.

다만 `config show`와 `doctor`는 새 이름 기준으로 해석해서 보여주고, legacy key를 변환해서 읽고 있으면 그 사실을 따로 알려줍니다.

`order preview`는 거래 기능이 꺼져 있어도 계속 사용할 수 있습니다.

## Schema

설정 파일은 아래 JSON Schema를 기준으로 합니다.

- [`schemas/config.schema.json`](../schemas/config.schema.json)

에디터나 LLM이 이 schema를 기준으로 `config.json`을 생성하거나 수정할 수 있습니다.
