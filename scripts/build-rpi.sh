#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

APP_NAME="wabus"
OUTPUT_DIR="$ROOT_DIR/bin"
OUTPUT_BIN="$OUTPUT_DIR/${APP_NAME}-linux-arm64"
HOST_BIN="$OUTPUT_DIR/${APP_NAME}-host"
ENV_FILE="${ENV_FILE:-$ROOT_DIR/.env}"
CACHE_DIR="${GTFS_CACHE_DIR:-.cache/gtfs}"
WAIT_TIMEOUT_SEC="${WAIT_TIMEOUT_SEC:-900}"
LOG_FILE="${LOG_FILE:-$ROOT_DIR/bin/gtfs-precompute.log}"

mkdir -p "$OUTPUT_DIR"

echo "[1/4] Building for Linux ARM64..."
(
  cd "$ROOT_DIR"
  GOOS=linux GOARCH=arm64 go build -o "$OUTPUT_BIN" ./cmd/wabus
)
echo "Built: $OUTPUT_BIN"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "[2/4] Skipping GTFS precompute: env file not found ($ENV_FILE)"
  exit 0
fi

echo "[2/4] Building host binary for local precompute..."
(
  cd "$ROOT_DIR"
  go build -o "$HOST_BIN" ./cmd/wabus
)
echo "Built: $HOST_BIN"

echo "[3/4] Starting GTFS precompute (cache dir: $CACHE_DIR)"
set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a

: "${WARSAW_API_KEY:?WARSAW_API_KEY must be set in $ENV_FILE for precompute}"

mkdir -p "$CACHE_DIR"

(
  cd "$ROOT_DIR"
  : >"$LOG_FILE"
  HTTP_ADDR=127.0.0.1:18080 \
  POLL_INTERVAL=1h \
  GTFS_CACHE_DIR="$CACHE_DIR" \
  "$HOST_BIN" >"$LOG_FILE" 2>&1 &
  APP_PID=$!

  cleanup() {
    if kill -0 "$APP_PID" 2>/dev/null; then
      kill "$APP_PID" 2>/dev/null || true
      wait "$APP_PID" 2>/dev/null || true
    fi
  }
  trap cleanup EXIT

  start_ts=$(date +%s)
  while true; do
    if grep -qE "persisted parsed GTFS cache|loaded parsed GTFS cache" "$LOG_FILE"; then
      echo "GTFS precompute finished (see $LOG_FILE)"
      break
    fi

    if ! kill -0 "$APP_PID" 2>/dev/null; then
      echo "ERROR: precompute process exited early. Check $LOG_FILE" >&2
      exit 1
    fi

    now_ts=$(date +%s)
    if (( now_ts - start_ts > WAIT_TIMEOUT_SEC )); then
      echo "ERROR: timeout waiting for GTFS precompute. Check $LOG_FILE" >&2
      exit 1
    fi

    sleep 2
  done
)

echo "[4/4] Done. Artifacts:"
ls -lh "$OUTPUT_BIN"
ls -lh "$CACHE_DIR" | grep -E 'gtfs\.zip|gtfs_meta\.json|gtfs_parsed_.*\.gob\.gz' || true
