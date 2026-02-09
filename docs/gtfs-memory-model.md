# GTFS Memory Model & Index Logic

This document describes the optimized GTFS in-memory structures and index-building flow.

## 1) High-level GTFS data pipeline

```mermaid
flowchart TD
    A[GTFS ZIP] --> B[Parser]

    B --> C[routes map route_id to Route]
    B --> D[shapes map shape_id to Shape]
    B --> E[stops map stop_id to Stop]
    B --> F[routeShapes map route_id to shape_id list]
    B --> G[trips list of TripMeta]
    B --> H[stopSchedules map stop_id to compact rows]
    B --> I[stopLines map stop_id to StopLine list]
    B --> J[routeStops map route_id to Stop list]
    B --> K[routeTripTimes map route_id to TripTimeEntry list]
    B --> L[calendars map service_id to Calendar]
    B --> M[calendarDates map service_id to CalendarDate list]

    N[parse only tripIndex map trip_id to trip_idx] -. used while parsing .-> B
    B -. dropped after parse .-> X[nil]
```

---

## 2) Compact stop_times representation

```mermaid
flowchart LR
    A[StopTimeCompact] --> A1[TripIndex uint32]
    A --> A2[ArrivalSeconds uint32]
    A --> A3[DepartureSeconds uint32]
    A --> A4[StopSequence uint16]

    A1 --> B[trips list]
    B --> C[TripMeta fields ID RouteID ServiceID ShapeID Headsign]
    C --> D[routes by RouteID]
    D --> E[Route ShortName to line]
```

### Why this is smaller

Instead of storing repeated strings per stop-time row (`trip_id`, `route_id`, `service_id`, `line`, `headsign`, `arrival_time`, `departure_time`), each row stores mostly numeric fields and a compact trip index.

---

## 3) API read/decode flow

```mermaid
sequenceDiagram
    participant API as GET /v1/stops/{id}/schedule
    participant S as GTFSStore
    participant SS as stopSchedules by stopID
    participant T as trips list
    participant R as routes map

    API->>S: GetStopSchedule or GetStopScheduleForDate
    S->>SS: load compact rows for stop
    loop each compact row
        S->>T: resolve TripMeta via TripIndex
        S->>R: resolve Route by RouteID
        S->>S: build StopTime response model
    end
    S-->>API: StopTime list same API shape
```

---

## 4) Derived indices build flow

```mermaid
flowchart TD
    A[stopSchedules compact] --> B[buildStopLines]
    A --> C[buildRouteStops]
    A --> D[buildTripTimeRanges]

    B --> B1[stopLines map stop_id to StopLine list]
    C --> C1[routeStops map route_id to Stop list]
    D --> D1[routeTripTimes map route_id to TripTimeEntry list]
```

---

## 5) Vehicle index update logic (separate fix)

```mermaid
flowchart LR
    U[vehicle update] --> E{exists}
    E -- no --> A[insert vehicle and add indices]
    E -- yes --> R[remove old indices]
    R --> W[store new vehicle]
    W --> A

    A --> T[byTile]
    A --> L[byLine]
    A --> Y[byType]
```

This prevents stale `byLine` / `byType` index growth when vehicle attributes change.
