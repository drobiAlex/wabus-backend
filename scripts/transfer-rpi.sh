#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

PI_USER="${PI_USER:-oleksandrdrobinin}"
PI_HOST="${PI_HOST:-raspberrypi}"
PI_PORT="${PI_PORT:-22}"
REMOTE_DIR="${REMOTE_DIR:-/home/${PI_USER}/wabus-backend}"

LOCAL_CACHE_DIR="${LOCAL_CACHE_DIR:-${GTFS_CACHE_DIR:-/tmp/wabus-gtfs-cache}}"
REMOTE_CACHE_DIR="${REMOTE_CACHE_DIR:-/tmp/wabus-gtfs-cache}"

LOCAL_BIN="$ROOT_DIR/bin/wabus-linux-arm64"
if [[ ! -f "$LOCAL_BIN" ]]; then
  echo "ERROR: binary not found: $LOCAL_BIN" >&2
  echo "Run scripts/build-rpi.sh first." >&2
  exit 1
fi

echo "Ensuring remote directories..."
ssh -p "$PI_PORT" "${PI_USER}@${PI_HOST}" "mkdir -p '$REMOTE_DIR/bin' '$REMOTE_DIR/scripts' '$REMOTE_CACHE_DIR'"

echo "Copying binary and scripts..."
rsync -avz --progress -e "ssh -p $PI_PORT" \
  "$LOCAL_BIN" "${PI_USER}@${PI_HOST}:$REMOTE_DIR/bin/wabus-linux-arm64"

rsync -avz --progress -e "ssh -p $PI_PORT" \
  --include='*.sh' \
  --exclude='*' \
  "$ROOT_DIR/scripts/" "${PI_USER}@${PI_HOST}:$REMOTE_DIR/scripts/"

echo "Copying optional .env.example..."
rsync -avz -e "ssh -p $PI_PORT" \
  "$ROOT_DIR/.env.example" "${PI_USER}@${PI_HOST}:$REMOTE_DIR/.env.example"

if [[ -d "$LOCAL_CACHE_DIR" ]]; then
  echo "Copying GTFS cache files from $LOCAL_CACHE_DIR..."
  rsync -avz --progress -e "ssh -p $PI_PORT" \
    --include='gtfs.zip' \
    --include='gtfs_meta.json' \
    --include='gtfs_parsed_*.gob.gz' \
    --exclude='*' \
    "${LOCAL_CACHE_DIR}/" "${PI_USER}@${PI_HOST}:${REMOTE_CACHE_DIR}/"
else
  echo "Skipping GTFS cache copy: local cache dir not found ($LOCAL_CACHE_DIR)"
fi

echo "Setting executable bits on Raspberry Pi..."
ssh -p "$PI_PORT" "${PI_USER}@${PI_HOST}" "chmod +x '$REMOTE_DIR/bin/wabus-linux-arm64' '$REMOTE_DIR/scripts/'*.sh"

echo "Done. On Raspberry Pi run:"
echo "  cd $REMOTE_DIR"
echo "  GTFS_CACHE_DIR=$REMOTE_CACHE_DIR ./scripts/run-rpi.sh"
echo "  ./scripts/install-systemd.sh"
