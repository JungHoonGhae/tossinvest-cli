# auth-helper

This directory contains the Python-based browser login helper used by `tossctl auth login`.

Responsibilities:

- open a real browser session
- let the user complete Toss Securities web login
- extract the minimum session state needed by the Go client
- return sanitized session payloads to `tossctl`

Hard boundary:

- no Toss Securities domain logic
- no read-only client bindings
- no trading logic

The helper exists to isolate browser automation from the main CLI.

## CLI

```bash
cd auth-helper
python3 -m pip install -e .
python3 -m tossctl_auth_helper login --storage-state /tmp/tossctl-storage-state.json
```

> Google Chrome이 시스템에 설치되어 있어야 합니다. `playwright install chromium`은 더 이상 필요하지 않습니다.

The helper emits JSON on stdout. On success it returns `status=ok` and the path of the saved Playwright storage-state file.

## Python 선택 (tossctl auth login)

`tossctl auth login`이 helper를 실행할 때 쓰는 Python은 아래 순서로 결정됩니다.

1. `TOSSCTL_AUTH_HELPER_PYTHON` (명시적 override — 있으면 그대로 사용)
2. uv tool이 관리하는 Python (있는 첫 번째가 선택됨)
   - `$UV_TOOL_DIR/tossctl-auth-helper/bin/python`
   - `$UV_TOOL_DIR/playwright/bin/python`
   - `$XDG_DATA_HOME/uv/tools/...`
   - `~/.local/share/uv/tools/...`
   - Windows: `%APPDATA%/uv/tools/...` 또는 `Scripts/python.exe`
3. PATH 상의 `python3`

즉 `uv tool install ./auth-helper` 또는 `uv tool install playwright`로 준비해 두면 전역 python 환경을 오염시키지 않고 helper를 실행할 수 있습니다. 다른 Python을 쓰고 싶을 때는 `TOSSCTL_AUTH_HELPER_PYTHON`으로 덮어쓰면 됩니다.

### Remote / CLI-only login (headless)

```bash
python3 -m tossctl_auth_helper login \
  --storage-state /tmp/tossctl-storage-state.json \
  --headless
```

stderr에 QR URL과 확인 문자가 출력됨. URL을 텔레그램 등으로 폰에 보내 탭 → Toss 앱이 열림 → 확인 문자 선택 → 완료. PNG 저장이 필요하면 `--qr-output <path>` (0600 권한).

Go CLI에서는 링크 중심 별칭인 `tossctl auth login --link`를 권장한다. 기존 자동화는 `tossctl auth login --headless [--qr-output <path>]`를 그대로 사용할 수 있다.
