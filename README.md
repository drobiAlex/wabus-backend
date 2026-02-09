# WaBus

Real-time Warsaw bus and tram tracking backend.

## Quick Start

```bash
# Build
go build -o wabus ./cmd/wabus

# Run (requires API key from api.um.warszawa.pl)
WARSAW_API_KEY=your_key ./wabus
```

## Raspberry Pi deploy flow

```bash
cd wabus-backend
./scripts/deploy-rpi.sh
```

On Raspberry Pi:

```bash
cd ~/wabus-backend
GTFS_CACHE_DIR=/tmp/wabus-gtfs-cache ./scripts/run-rpi.sh
```

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `WARSAW_API_KEY` | (required) | API key from api.um.warszawa.pl |
| `HTTP_ADDR` | `:8080` | HTTP server address |
| `POLL_INTERVAL` | `10s` | Upstream polling interval |
| `VEHICLE_STALE_AFTER` | `5m` | Remove vehicles not seen for this duration |
| `TILE_ZOOM_LEVEL` | `14` | Web Mercator zoom level for tile subscriptions |

## API Endpoints

### REST

- `GET /v1/vehicles` - List all vehicles
  - `?type=1` - Filter by type (1=bus, 2=tram)
  - `?line=520` - Filter by line number
  - `?bbox=52.1,20.8,52.4,21.2` - Filter by bounding box (minLat,minLon,maxLat,maxLon)
- `GET /v1/vehicles/{key}` - Get single vehicle
- `GET /healthz` - Liveness check
- `GET /readyz` - Readiness check

### WebSocket

Connect to `ws://localhost:8080/v1/ws`

**Subscribe to tiles:**
```json
{"type":"subscribe","payload":{"tileIds":["14/9234/5235"]}}
```

**Unsubscribe:**
```json
{"type":"unsubscribe","payload":{"tileIds":["14/9234/5235"]}}
```

**Server messages:**
- `snapshot` - Initial vehicles for subscribed tiles
- `delta` - Updates and removes

## Architecture

```
Warsaw API (10s poll)
       │
       ▼
   Ingestor ─── calculates tiles, detects deltas
       │
       ├──────► Store (in-memory, indexed)
       │              │
       │              ▼
       │        HTTP API (/v1/vehicles)
       │
       ▼
      Hub ─────► WebSocket Clients (tile-based fanout)
```


# Run stress test with vegeta:

```bash
export BASE_URL=https://wabus-api.lokki.space &&
export REQ_PER_SEC=300 &&
export DURATION=20s &&
export TARGETS_COUNT=800 &&
export RATE_LIMIT_WHITELIST=46.205.203.191 &&
./scripts/stress-gtfs.sh
```
