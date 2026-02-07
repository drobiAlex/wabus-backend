#!/usr/bin/env bash
set -euo pipefail

ENV_FILE="${1:-.env}"
APP_BIN="${2:-./wabus}"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "Error: env file not found: $ENV_FILE" >&2
  exit 1
fi

if [[ ! -x "$APP_BIN" ]]; then
  echo "Error: app binary is not executable or not found: $APP_BIN" >&2
  exit 1
fi

# Export all vars loaded from .env
set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a

exec "$APP_BIN"
