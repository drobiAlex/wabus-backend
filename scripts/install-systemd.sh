#!/usr/bin/env bash
set -euo pipefail

SERVICE_NAME="${SERVICE_NAME:-wabus}"
APP_USER="${APP_USER:-$USER}"
APP_DIR="${APP_DIR:-$HOME/wabus-backend}"
ENV_FILE="${ENV_FILE:-$APP_DIR/.env}"
BIN_PATH="${BIN_PATH:-$APP_DIR/bin/wabus-linux-arm64}"
RUN_SCRIPT="${RUN_SCRIPT:-$APP_DIR/scripts/run-rpi.sh}"
GTFS_CACHE_DIR="${GTFS_CACHE_DIR:-/tmp/wabus-gtfs-cache}"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "Error: env file not found: $ENV_FILE" >&2
  exit 1
fi

if [[ ! -x "$BIN_PATH" ]]; then
  echo "Error: binary not executable: $BIN_PATH" >&2
  exit 1
fi

if [[ ! -x "$RUN_SCRIPT" ]]; then
  echo "Error: run script not executable: $RUN_SCRIPT" >&2
  exit 1
fi

SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"

echo "Installing systemd service: $SERVICE_FILE"

sudo tee "$SERVICE_FILE" >/dev/null <<EOF
[Unit]
Description=WaBus backend
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=$APP_USER
WorkingDirectory=$APP_DIR
Environment=GTFS_CACHE_DIR=$GTFS_CACHE_DIR
ExecStart=$RUN_SCRIPT $ENV_FILE $BIN_PATH
Restart=always
RestartSec=3
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable "$SERVICE_NAME"
sudo systemctl restart "$SERVICE_NAME"

sudo systemctl --no-pager --full status "$SERVICE_NAME" || true

echo "Done. Useful commands:"
echo "  sudo systemctl restart $SERVICE_NAME"
echo "  sudo systemctl stop $SERVICE_NAME"
echo "  sudo journalctl -u $SERVICE_NAME -f"
