#!/usr/bin/env bash
# Launch DIS using the repository configuration and available Python environment.
set -euo pipefail
DIS_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
if [[ -z "${PYTHON:-}" ]]; then
  if [[ -x "$DIS_ROOT/.venv/bin/python" ]]; then
    PYTHON="$DIS_ROOT/.venv/bin/python"
  else
    PYTHON=python3
  fi
fi
if [[ $# -eq 0 ]]; then
  set -- up
elif [[ "$1" == --config ]]; then
  set -- up "$@"
fi
exec "$PYTHON" -u "$DIS_ROOT/scripts/dis.py" "$@"
