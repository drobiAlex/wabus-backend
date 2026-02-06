package handler

import (
	"log/slog"
	"net/http"
	"time"

	"wabus/internal/domain"
	"wabus/internal/store"
)

type GTFSHandler struct {
	store  *store.GTFSStore
	logger *slog.Logger
}

func NewGTFSHandler(store *store.GTFSStore, logger *slog.Logger) *GTFSHandler {
	return &GTFSHandler{
		store:  store,
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

	shapes := h.store.GetRouteShapes(route.ID)

	totalPoints := 0
	for _, s := range shapes {
		totalPoints += len(s.Points)
	}

	h.logger.Debug("GetRouteShape response",
		"line", line,
		"shapes_count", len(shapes),
		"total_points", totalPoints,
		"duration_ms", time.Since(start).Milliseconds(),
	)

	respondJSON(w, http.StatusOK, ShapesResponse{
		Shapes:     shapes,
		Count:      len(shapes),
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

	if dateParam != "" {
		var filterDate time.Time
		var err error

		if dateParam == "today" {
			filterDate = time.Now()
		} else {
			filterDate, err = time.Parse("2006-01-02", dateParam)
			if err != nil {
				h.logger.Warn("GetStopSchedule bad date format", "date", dateParam, "error", err)
				respondError(w, http.StatusBadRequest, "invalid date format, use YYYY-MM-DD or 'today'")
				return
			}
		}

		schedule = h.store.GetStopScheduleForDate(id, filterDate)
		h.logger.Debug("GetStopSchedule filtered by date",
			"stop_id", id,
			"date", filterDate.Format("2006-01-02"),
			"weekday", filterDate.Weekday().String(),
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

	lines := h.store.GetStopLines(id)

	lineNames := make([]string, len(lines))
	for i, l := range lines {
		lineNames[i] = l.Line
	}

	h.logger.Debug("GetStopLines response",
		"stop_id", id,
		"stop_name", stop.Name,
		"lines_count", len(lines),
		"lines", lineNames,
		"duration_ms", time.Since(start).Milliseconds(),
	)

	respondJSON(w, http.StatusOK, StopLinesResponse{
		Lines:      lines,
		Count:      len(lines),
		ServerTime: time.Now(),
	})
}
