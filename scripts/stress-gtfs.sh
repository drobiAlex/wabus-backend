#!/usr/bin/env bash
set -euo pipefail

# GTFS API stress test (rate-controlled) using Vegeta.
#
# Features:
# - Prepopulates route lines and stop IDs from live GTFS API
# - Generates mixed targets for:
#   - /v1/routes/{line}
#   - /v1/routes/{line}/shape?time=now
#   - /v1/stops/{id}/schedule?date=today
# - Allows req/sec + duration configuration
#
# Usage:
#   ./scripts/stress-gtfs.sh
#   BASE_URL=http://127.0.0.1:8080 REQ_PER_SEC=200 DURATION=60s ./scripts/stress-gtfs.sh
#
# Optional env:
#   BASE_URL                (default: http://127.0.0.1:8080)
#   REQ_PER_SEC             (default: 100)
#   DURATION                (default: 30s)
#   TARGETS_COUNT           (default: 5000)
#   TIMEOUT                 (default: 10s)
#   ROUTE_WEIGHT            (default: 20)
#   ROUTE_SHAPE_WEIGHT      (default: 30)
#   STOP_SCHEDULE_WEIGHT    (default: 50)
#   SCHEDULE_DATE           (default: today)
#   OUTPUT_DIR              (default: ./tmp/stress)
#
# Notes:
# - Requires: curl, python3, awk
# - Requires Vegeta binary: https://github.com/tsenart/vegeta

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
REQ_PER_SEC="${REQ_PER_SEC:-100}"
DURATION="${DURATION:-30s}"
TARGETS_COUNT="${TARGETS_COUNT:-5000}"
TIMEOUT="${TIMEOUT:-10s}"

ROUTE_WEIGHT="${ROUTE_WEIGHT:-20}"
ROUTE_SHAPE_WEIGHT="${ROUTE_SHAPE_WEIGHT:-30}"
STOP_SCHEDULE_WEIGHT="${STOP_SCHEDULE_WEIGHT:-50}"
SCHEDULE_DATE="${SCHEDULE_DATE:-today}"

OUTPUT_DIR="${OUTPUT_DIR:-./tmp/stress}"
mkdir -p "$OUTPUT_DIR"

TARGETS_FILE="$OUTPUT_DIR/targets.txt"
RESULT_BIN="$OUTPUT_DIR/results.bin"
REPORT_TXT="$OUTPUT_DIR/report.txt"
PLOT_HTML="$OUTPUT_DIR/plot.html"
ROUTES_JSON="$OUTPUT_DIR/routes.json"
STOPS_JSON="$OUTPUT_DIR/stops.json"

echo "==> Stress config"
echo "BASE_URL=$BASE_URL"
echo "REQ_PER_SEC=$REQ_PER_SEC"
echo "DURATION=$DURATION"
echo "TARGETS_COUNT=$TARGETS_COUNT"
echo "TIMEOUT=$TIMEOUT"
echo "WEIGHTS route=$ROUTE_WEIGHT shape=$ROUTE_SHAPE_WEIGHT schedule=$STOP_SCHEDULE_WEIGHT"

if ! command -v vegeta >/dev/null 2>&1; then
  echo "ERROR: vegeta is not installed or not in PATH"
  echo "Install: brew install vegeta  (macOS)"
  echo "         or download from: https://github.com/tsenart/vegeta/releases"
  exit 1
fi

# Pick Python interpreter robustly (pyenv shims can be broken on some setups).
PYTHON_BIN="${PYTHON_BIN:-}"
if [[ -z "$PYTHON_BIN" ]]; then
  if command -v python3 >/dev/null 2>&1; then
    PYTHON_BIN="$(command -v python3)"
  fi
fi
if [[ -z "$PYTHON_BIN" || ! -x "$PYTHON_BIN" ]]; then
  if [[ -x "/usr/bin/python3" ]]; then
    PYTHON_BIN="/usr/bin/python3"
  fi
fi
if [[ -z "$PYTHON_BIN" || ! -x "$PYTHON_BIN" ]]; then
  echo "ERROR: python3 is required (set PYTHON_BIN=/path/to/python3)"
  exit 1
fi

echo "==> Checking API availability"
if ! curl -fsS "$BASE_URL/healthz" >/dev/null 2>&1; then
  echo "ERROR: API is not reachable at $BASE_URL"
  exit 1
fi

echo "==> Fetching GTFS routes/stops"
curl -fsS "$BASE_URL/v1/routes" > "$ROUTES_JSON"
curl -fsS "$BASE_URL/v1/stops" > "$STOPS_JSON"

echo "==> Building Vegeta targets"
"$PYTHON_BIN" - "$ROUTES_JSON" "$STOPS_JSON" "$TARGETS_FILE" "$BASE_URL" "$TARGETS_COUNT" "$ROUTE_WEIGHT" "$ROUTE_SHAPE_WEIGHT" "$STOP_SCHEDULE_WEIGHT" "$SCHEDULE_DATE" <<'PY'
import json
import random
import sys

routes_path, stops_path, out_path, base_url, total_s, w_route_s, w_shape_s, w_sched_s, sched_date = sys.argv[1:]

total = int(total_s)
w_route = int(w_route_s)
w_shape = int(w_shape_s)
w_sched = int(w_sched_s)

if total <= 0:
    raise SystemExit("TARGETS_COUNT must be > 0")

if w_route < 0 or w_shape < 0 or w_sched < 0:
    raise SystemExit("weights must be >= 0")

w_sum = w_route + w_shape + w_sched
if w_sum <= 0:
    raise SystemExit("at least one weight must be > 0")

with open(routes_path, 'r', encoding='utf-8') as f:
    routes_json = json.load(f)
with open(stops_path, 'r', encoding='utf-8') as f:
    stops_json = json.load(f)

routes = routes_json.get('routes', [])
stops = stops_json.get('stops', [])

lines = [str(r.get('short_name', '')).strip() for r in routes if r.get('short_name')]
stop_ids = [str(s.get('id', '')).strip() for s in stops if s.get('id')]

if not lines:
    raise SystemExit("No routes/lines loaded from /v1/routes")
if not stop_ids:
    raise SystemExit("No stops loaded from /v1/stops")

# Normalize counts by weights.
n_route = total * w_route // w_sum
n_shape = total * w_shape // w_sum
n_sched = total - n_route - n_shape

rnd = random.Random(42)

targets = []
for _ in range(n_route):
    line = rnd.choice(lines)
    targets.append(f"GET {base_url}/v1/routes/{line}")

for _ in range(n_shape):
    line = rnd.choice(lines)
    targets.append(f"GET {base_url}/v1/routes/{line}/shape?time=now")

for _ in range(n_sched):
    sid = rnd.choice(stop_ids)
    targets.append(f"GET {base_url}/v1/stops/{sid}/schedule?date={sched_date}")

rnd.shuffle(targets)

with open(out_path, 'w', encoding='utf-8') as f:
    f.write("\n".join(targets))
    f.write("\n")

print(f"lines={len(lines)} stops={len(stop_ids)} targets={len(targets)}")
print(f"mix: route={n_route} shape={n_shape} schedule={n_sched}")
PY

echo "==> Running Vegeta attack"
vegeta attack \
  -targets="$TARGETS_FILE" \
  -rate="$REQ_PER_SEC" \
  -duration="$DURATION" \
  -timeout="$TIMEOUT" \
  > "$RESULT_BIN"

echo "==> Generating report"
vegeta report < "$RESULT_BIN" | tee "$REPORT_TXT"
vegeta plot < "$RESULT_BIN" > "$PLOT_HTML"

echo "\nArtifacts:"
echo "- Targets:  $TARGETS_FILE"
echo "- Results:  $RESULT_BIN"
echo "- Report:   $REPORT_TXT"
echo "- Plot:     $PLOT_HTML"