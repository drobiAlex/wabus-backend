#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

PI_USER="${PI_USER:-oleksandrdrobinin}"
PI_HOST="${PI_HOST:-pi}"
PI_PORT="${PI_PORT:-22}"
REMOTE_PARENT_DIR="${REMOTE_PARENT_DIR:-/home/${PI_USER}}"

scp -P "$PI_PORT" -r "$ROOT_DIR" "${PI_USER}@${PI_HOST}:${REMOTE_PARENT_DIR}"
