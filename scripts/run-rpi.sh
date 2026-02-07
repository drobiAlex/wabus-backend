#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

ENV_FILE="${1:-$ROOT_DIR/.env}"
APP_BIN="${2:-$ROOT_DIR/bin/wabus-linux-arm64}"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "Error: env file not found: $ENV_FILE" >&2
  exit 1
fi

if [[ ! -x "$APP_BIN" ]]; then
  echo "Error: app binary is not executable or not found: $APP_BIN" >&2
  exit 1
fi

set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a

exec "$APP_BIN"
