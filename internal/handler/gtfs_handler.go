package handler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"wabus/internal/cache"
	"wabus/internal/domain"
	"wabus/internal/store"
)

type GTFSHandler struct {
	store  *store.GTFSStore
	cache  *cache.RedisCache
	logger *slog.Logger
}

func NewGTFSHandler(store *store.GTFSStore, redisCache *cache.RedisCache, logger *slog.Logger) *GTFSHandler {
	return &GTFSHandler{
		store:  store,
		cache:  redisCache,
		logger: logger.With("handler", "gtfs"),
	}
}

type RoutesResponse struct {
	Routes     []*domain.Route `json:"routes"`
	Count      int             `json:"count"`
	ServerTime time.Time       `json:"server_time"`
}

func (h *GTFSHandler) ListRoutes(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	h.logger.Debug("ListRoutes request",
		"method", r.Method,
		"path", r.URL.Path,
		"remote_addr", r.RemoteAddr,
	)

	routes := h.store.GetAllRoutes()

	h.logger.Debug("ListRoutes response",
		"count", len(routes),
		"duration_ms", time.Since(start).Milliseconds(),
	)

	respondJSON(w, http.StatusOK, RoutesResponse{
		Routes:     routes,
		Count:      len(routes),
		ServerTime: time.Now(),
	})
}

func (h *GTFSHandler) GetRoute(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	line := r.PathValue("line")

	h.logger.Debug("GetRoute request",
		"method", r.Method,
		"path", r.URL.Path,
		"line", line,
		"remote_addr", r.RemoteAddr,
	)

	if line == "" {
		h.logger.Warn("GetRoute bad request", "error", "missing line parameter")
		respondError(w, http.StatusBadRequest, "missing line parameter")
		return
	}

	route, ok := h.store.GetRouteByLine(line)
	if !ok {
		h.logger.Debug("GetRoute not found", "line", line)
		respondError(w, http.StatusNotFound, "route not found")
		return
	}

	h.logger.Debug("GetRoute response",
		"line", line,
		"route_id", route.ID,
		"duration_ms", time.Since(start).Milliseconds(),
	)

	respondJSON(w, http.StatusOK, route)
}

type ShapesResponse struct {
	Shapes     []*domain.Shape `json:"shapes"`
	Count      int             `json:"count"`
	ServerTime time.Time       `json:"server_time"`
}

func (h *GTFSHandler) GetRouteShape(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	line := r.PathValue("line")

	h.logger.Debug("GetRouteShape request",
		"method", r.Method,
		"path", r.URL.Path,
		"line", line,
		"remote_addr", r.RemoteAddr,
	)

	if line == "" {
		h.logger.Warn("GetRouteShape bad request", "error", "missing line parameter")
		respondError(w, http.StatusBadRequest, "missing line parameter")
		return
	}

	route, ok := h.store.GetRouteByLine(line)
	if !ok {
		h.logger.Debug("GetRouteShape route not found", "line", line)
		respondError(w, http.StatusNotFound, "route not found")
		return
	}

	timeParam := r.URL.Query().Get("time")

	var shapes []*domain.Shape
	if timeParam != "" {
		timeMinutes := parseTimeToMinutes(timeParam)
		shapes = h.store.GetActiveRouteShapes(route.ID, time.Now(), timeMinutes)
		h.logger.Debug("GetRouteShape filtered by time",
			"line", line,
			"time_param", timeParam,
			"time_minutes", timeMinutes,
		)
	} else {
		shapes = h.store.GetRouteShapes(route.ID)
	}

	totalPoints := 0
	for _, s := range shapes {
		totalPoints += len(s.Points)
	}

	h.logger.Debug("GetRouteShape response",
		"line", line,
		"shapes_count", len(shapes),
		"total_points", totalPoints,
		"time_filtered", timeParam != "",
		"duration_ms", time.Since(start).Milliseconds(),
	)

	respondJSON(w, http.StatusOK, ShapesResponse{
		Shapes:     shapes,
		Count:      len(shapes),
		ServerTime: time.Now(),
	})
}

type RouteStopsResponse struct {
	Stops      []*domain.Stop `json:"stops"`
	Count      int            `json:"count"`
	ServerTime time.Time      `json:"server_time"`
}

func (h *GTFSHandler) GetRouteStops(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	line := r.PathValue("line")

	h.logger.Debug("GetRouteStops request",
		"method", r.Method,
		"path", r.URL.Path,
		"line", line,
		"remote_addr", r.RemoteAddr,
	)

	if line == "" {
		h.logger.Warn("GetRouteStops bad request", "error", "missing line parameter")
		respondError(w, http.StatusBadRequest, "missing line parameter")
		return
	}

	route, ok := h.store.GetRouteByLine(line)
	if !ok {
		h.logger.Debug("GetRouteStops route not found", "line", line)
		respondError(w, http.StatusNotFound, "route not found")
		return
	}

	stops := h.store.GetRouteStops(route.ID)

	h.logger.Debug("GetRouteStops response",
		"line", line,
		"stops_count", len(stops),
		"duration_ms", time.Since(start).Milliseconds(),
	)

	respondJSON(w, http.StatusOK, RouteStopsResponse{
		Stops:      stops,
		Count:      len(stops),
		ServerTime: time.Now(),
	})
}

type StopsResponse struct {
	Stops      []*domain.Stop `json:"stops"`
	Count      int            `json:"count"`
	ServerTime time.Time      `json:"server_time"`
}

func (h *GTFSHandler) ListStops(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	h.logger.Debug("ListStops request",
		"method", r.Method,
		"path", r.URL.Path,
		"remote_addr", r.RemoteAddr,
	)

	stops := h.store.GetAllStops()

	h.logger.Debug("ListStops response",
		"count", len(stops),
		"duration_ms", time.Since(start).Milliseconds(),
	)

	respondJSON(w, http.StatusOK, StopsResponse{
		Stops:      stops,
		Count:      len(stops),
		ServerTime: time.Now(),
	})
}

func (h *GTFSHandler) GetStop(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	id := r.PathValue("id")

	h.logger.Debug("GetStop request",
		"method", r.Method,
		"path", r.URL.Path,
		"stop_id", id,
		"remote_addr", r.RemoteAddr,
	)

	if id == "" {
		h.logger.Warn("GetStop bad request", "error", "missing stop id")
		respondError(w, http.StatusBadRequest, "missing stop id")
		return
	}

	stop, ok := h.store.GetStopByID(id)
	if !ok {
		h.logger.Debug("GetStop not found", "stop_id", id)
		respondError(w, http.StatusNotFound, "stop not found")
		return
	}

	h.logger.Debug("GetStop response",
		"stop_id", id,
		"stop_name", stop.Name,
		"duration_ms", time.Since(start).Milliseconds(),
	)

	respondJSON(w, http.StatusOK, stop)
}

func (h *GTFSHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	h.logger.Debug("GetStats request",
		"method", r.Method,
		"path", r.URL.Path,
		"remote_addr", r.RemoteAddr,
	)

	stats := h.store.GetStats()

	h.logger.Debug("GetStats response",
		"routes_count", stats.RoutesCount,
		"shapes_count", stats.ShapesCount,
		"stops_count", stats.StopsCount,
		"is_loaded", stats.IsLoaded,
		"duration_ms", time.Since(start).Milliseconds(),
	)

	respondJSON(w, http.StatusOK, stats)
}

type StopScheduleResponse struct {
	StopTimes  []*domain.StopTime  `json:"stop_times"`
	Count      int                 `json:"count"`
	ServerTime time.Time           `json:"server_time"`
}

func (h *GTFSHandler) GetStopSchedule(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	id := r.PathValue("id")
	dateParam := r.URL.Query().Get("date")

	h.logger.Debug("GetStopSchedule request",
		"method", r.Method,
		"path", r.URL.Path,
		"stop_id", id,
		"date", dateParam,
		"remote_addr", r.RemoteAddr,
	)

	if id == "" {
		h.logger.Warn("GetStopSchedule bad request", "error", "missing stop id")
		respondError(w, http.StatusBadRequest, "missing stop id")
		return
	}

	stop, ok := h.store.GetStopByID(id)
	if !ok {
		h.logger.Debug("GetStopSchedule stop not found", "stop_id", id)
		respondError(w, http.StatusNotFound, "stop not found")
		return
	}

	var schedule []*domain.StopTime
	cacheHit := false
	ctx := r.Context()

	if dateParam != "" {
		var filterDate time.Time
		var err error

		if dateParam == "today" {
			filterDate = time.Now()
			if h.tryGetFromCache(ctx, cache.KeyScheduleToday(id), &schedule) {
				cacheHit = true
				h.logger.Debug("GetStopSchedule cache hit", "stop_id", id, "key", "today")
			}
		} else if dateParam == "tomorrow" {
			filterDate = time.Now().AddDate(0, 0, 1)
			if h.tryGetFromCache(ctx, cache.KeyScheduleTomorrow(id), &schedule) {
				cacheHit = true
				h.logger.Debug("GetStopSchedule cache hit", "stop_id", id, "key", "tomorrow")
			}
		} else {
			filterDate, err = time.Parse("2006-01-02", dateParam)
			if err != nil {
				h.logger.Warn("GetStopSchedule bad date format", "date", dateParam, "error", err)
				respondError(w, http.StatusBadRequest, "invalid date format, use YYYY-MM-DD, 'today', or 'tomorrow'")
				return
			}
		}

		if !cacheHit {
			schedule = h.store.GetStopScheduleForDate(id, filterDate)
		}
		h.logger.Debug("GetStopSchedule filtered by date",
			"stop_id", id,
			"date", filterDate.Format("2006-01-02"),
			"weekday", filterDate.Weekday().String(),
			"cache_hit", cacheHit,
		)
	} else {
		schedule = h.store.GetStopSchedule(id)
	}

	h.logger.Debug("GetStopSchedule response",
		"stop_id", id,
		"stop_name", stop.Name,
		"schedule_count", len(schedule),
		"filtered_by_date", dateParam != "",
		"duration_ms", time.Since(start).Milliseconds(),
	)

	respondJSON(w, http.StatusOK, StopScheduleResponse{
		StopTimes:  schedule,
		Count:      len(schedule),
		ServerTime: time.Now(),
	})
}

type StopLinesResponse struct {
	Lines      []*domain.StopLine  `json:"lines"`
	Count      int                 `json:"count"`
	ServerTime time.Time           `json:"server_time"`
}

func (h *GTFSHandler) GetStopLines(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	id := r.PathValue("id")

	h.logger.Debug("GetStopLines request",
		"method", r.Method,
		"path", r.URL.Path,
		"stop_id", id,
		"remote_addr", r.RemoteAddr,
	)

	if id == "" {
		h.logger.Warn("GetStopLines bad request", "error", "missing stop id")
		respondError(w, http.StatusBadRequest, "missing stop id")
		return
	}

	stop, ok := h.store.GetStopByID(id)
	if !ok {
		h.logger.Debug("GetStopLines stop not found", "stop_id", id)
		respondError(w, http.StatusNotFound, "stop not found")
		return
	}

	var lines []*domain.StopLine
	cacheHit := false
	ctx := r.Context()

	if h.tryGetFromCache(ctx, cache.KeyStopLines(id), &lines) {
		cacheHit = true
		h.logger.Debug("GetStopLines cache hit", "stop_id", id)
	} else {
		lines = h.store.GetStopLines(id)
	}

	lineNames := make([]string, len(lines))
	for i, l := range lines {
		lineNames[i] = l.Line
	}

	h.logger.Debug("GetStopLines response",
		"stop_id", id,
		"stop_name", stop.Name,
		"lines_count", len(lines),
		"lines", lineNames,
		"cache_hit", cacheHit,
		"duration_ms", time.Since(start).Milliseconds(),
	)

	respondJSON(w, http.StatusOK, StopLinesResponse{
		Lines:      lines,
		Count:      len(lines),
		ServerTime: time.Now(),
	})
}

type SyncResponse struct {
	Routes        []*domain.Route        `json:"routes"`
	Stops         []*domain.Stop         `json:"stops"`
	Calendars     []*domain.Calendar     `json:"calendars"`
	CalendarDates []*domain.CalendarDate `json:"calendar_dates"`
	Version       string                 `json:"version"`
	GeneratedAt   time.Time              `json:"generated_at"`
}

func (h *GTFSHandler) GetSync(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	h.logger.Debug("GetSync request",
		"method", r.Method,
		"path", r.URL.Path,
		"remote_addr", r.RemoteAddr,
	)

	stats := h.store.GetStats()

	// Return 503 if GTFS data is not loaded yet
	if !stats.IsLoaded {
		h.logger.Warn("GetSync called but GTFS data not loaded yet")
		w.Header().Set("Retry-After", "30")
		respondError(w, http.StatusServiceUnavailable, "GTFS data is loading, please retry")
		return
	}
	etag := fmt.Sprintf(`"%x"`, stats.LastUpdate.Unix())

	if r.Header.Get("If-None-Match") == etag {
		h.logger.Debug("GetSync not modified (ETag match)")
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "public, max-age=3600")

	ctx := r.Context()

	if h.cache != nil {
		var syncData SyncResponse
		found, err := h.cache.GetJSONCompressed(ctx, cache.KeySyncFull, &syncData)
		if err == nil && found {
			h.logger.Debug("GetSync cache hit", "duration_ms", time.Since(start).Milliseconds())
			respondJSON(w, http.StatusOK, syncData)
			return
		}
	}

	calendars, calendarDates := h.store.GetCalendarsAndDates()

	syncData := SyncResponse{
		Routes:        h.store.GetAllRoutes(),
		Stops:         h.store.GetAllStops(),
		Calendars:     calendars,
		CalendarDates: calendarDates,
		Version:       stats.LastUpdate.Format("2006-01-02"),
		GeneratedAt:   time.Now(),
	}

	h.logger.Debug("GetSync response",
		"routes", len(syncData.Routes),
		"stops", len(syncData.Stops),
		"calendars", len(syncData.Calendars),
		"calendar_dates", len(syncData.CalendarDates),
		"duration_ms", time.Since(start).Milliseconds(),
	)

	respondJSON(w, http.StatusOK, syncData)
}

type SyncCheckResponse struct {
	Version    string    `json:"version"`
	HasUpdates bool      `json:"has_updates"`
	LastUpdate time.Time `json:"last_update"`
}

func (h *GTFSHandler) CheckSync(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	sinceParam := r.URL.Query().Get("since")

	h.logger.Debug("CheckSync request",
		"method", r.Method,
		"path", r.URL.Path,
		"since", sinceParam,
		"remote_addr", r.RemoteAddr,
	)

	stats := h.store.GetStats()

	// Return 503 if GTFS data is not loaded yet
	if !stats.IsLoaded {
		h.logger.Warn("CheckSync called but GTFS data not loaded yet")
		w.Header().Set("Retry-After", "30")
		respondError(w, http.StatusServiceUnavailable, "GTFS data is loading, please retry")
		return
	}

	version := stats.LastUpdate.Format("2006-01-02")

	hasUpdates := true
	if sinceParam != "" {
		sinceDate, err := time.Parse("2006-01-02", sinceParam)
		if err == nil {
			hasUpdates = stats.LastUpdate.After(sinceDate)
		}
	}

	h.logger.Debug("CheckSync response",
		"version", version,
		"has_updates", hasUpdates,
		"duration_ms", time.Since(start).Milliseconds(),
	)

	respondJSON(w, http.StatusOK, SyncCheckResponse{
		Version:    version,
		HasUpdates: hasUpdates,
		LastUpdate: stats.LastUpdate,
	})
}

func (h *GTFSHandler) tryGetFromCache(ctx context.Context, key string, dest interface{}) bool {
	if h.cache == nil {
		return false
	}
	found, err := h.cache.GetJSON(ctx, key, dest)
	return err == nil && found
}

// parseTimeToMinutes parses "HH:MM" or "now" to minutes since midnight.
func parseTimeToMinutes(s string) int {
	if s == "now" {
		now := time.Now()
		return now.Hour()*60 + now.Minute()
	}
	parts := strings.Split(s, ":")
	if len(parts) < 2 {
		return 0
	}
	hours, _ := strconv.Atoi(parts[0])
	minutes, _ := strconv.Atoi(parts[1])
	return hours*60 + minutes
}
