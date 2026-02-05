package handler

import (
	"net/http"
	"time"

	"wabus/internal/domain"
	"wabus/internal/store"
)

type GTFSHandler struct {
	store *store.GTFSStore
}

func NewGTFSHandler(store *store.GTFSStore) *GTFSHandler {
	return &GTFSHandler{store: store}
}

type RoutesResponse struct {
	Routes     []*domain.Route `json:"routes"`
	Count      int             `json:"count"`
	ServerTime time.Time       `json:"server_time"`
}

func (h *GTFSHandler) ListRoutes(w http.ResponseWriter, r *http.Request) {
	routes := h.store.GetAllRoutes()

	respondJSON(w, http.StatusOK, RoutesResponse{
		Routes:     routes,
		Count:      len(routes),
		ServerTime: time.Now(),
	})
}

func (h *GTFSHandler) GetRoute(w http.ResponseWriter, r *http.Request) {
	line := r.PathValue("line")
	if line == "" {
		respondError(w, http.StatusBadRequest, "missing line parameter")
		return
	}

	route, ok := h.store.GetRouteByLine(line)
	if !ok {
		respondError(w, http.StatusNotFound, "route not found")
		return
	}

	respondJSON(w, http.StatusOK, route)
}

type ShapesResponse struct {
	Shapes     []*domain.Shape `json:"shapes"`
	Count      int             `json:"count"`
	ServerTime time.Time       `json:"server_time"`
}

func (h *GTFSHandler) GetRouteShape(w http.ResponseWriter, r *http.Request) {
	line := r.PathValue("line")
	if line == "" {
		respondError(w, http.StatusBadRequest, "missing line parameter")
		return
	}

	route, ok := h.store.GetRouteByLine(line)
	if !ok {
		respondError(w, http.StatusNotFound, "route not found")
		return
	}

	shapes := h.store.GetRouteShapes(route.ID)

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
	stops := h.store.GetAllStops()

	respondJSON(w, http.StatusOK, StopsResponse{
		Stops:      stops,
		Count:      len(stops),
		ServerTime: time.Now(),
	})
}

func (h *GTFSHandler) GetStop(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "missing stop id")
		return
	}

	stop, ok := h.store.GetStopByID(id)
	if !ok {
		respondError(w, http.StatusNotFound, "stop not found")
		return
	}

	respondJSON(w, http.StatusOK, stop)
}

func (h *GTFSHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.store.GetStats()
	respondJSON(w, http.StatusOK, stats)
}
