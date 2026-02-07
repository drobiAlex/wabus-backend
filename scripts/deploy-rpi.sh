#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

"$SCRIPT_DIR/build-rpi.sh"
"$SCRIPT_DIR/transfer-rpi.sh"

echo "Deploy completed. SSH into Raspberry Pi and run ./scripts/run-rpi.sh"
