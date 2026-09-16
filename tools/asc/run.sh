#!/usr/bin/env bash
# Isolated CLI process: ASC patches Python imports, so never embed it in our tools.
set -euo pipefail
asc_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
asc_env="${TOSSCTL_ASC_ENV:-${XDG_DATA_HOME:-$HOME/.local/share}/tossctl-tools/asc}"
asc_requirements="$asc_root/requirements.txt"

if [[ "${1:-}" == "setup" ]]; then
  command -v uv >/dev/null || { echo 'Install uv, then rerun tools/asc/run.sh setup.' >&2; exit 1; }
  if [[ ! -x "$asc_env/bin/python" ]]; then
    uv venv --python 3.12 "$asc_env"
  fi
  uv pip sync --python "$asc_env/bin/python" "$asc_requirements"
  cp "$asc_requirements" "$asc_env/tossctl-asc-requirements.txt"
  echo "ASC ready: $asc_env"
  exit 0
fi

if [[ ! -x "$asc_env/bin/droidasc" ]] || ! cmp -s "$asc_requirements" "$asc_env/tossctl-asc-requirements.txt"; then
  echo 'ASC environment missing or lock changed. Run tools/asc/run.sh setup.' >&2
  exit 1
fi
exec "$asc_env/bin/droidasc" "$@"
