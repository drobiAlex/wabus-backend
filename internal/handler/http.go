package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"wabus/internal/domain"
	"wabus/internal/store"
)

type HTTPHandler struct {
	store *store.Store
}

func NewHTTPHandler(store *store.Store) *HTTPHandler {
	return &HTTPHandler{store: store}
}

type VehiclesResponse struct {
	Vehicles   []*domain.Vehicle `json:"vehicles"`
	Count      int               `json:"count"`
	ServerTime time.Time         `json:"serverTime"`
}

func (h *HTTPHandler) ListVehicles(w http.ResponseWriter, r *http.Request) {
	opts := store.ListOptions{}

	if typeStr := r.URL.Query().Get("type"); typeStr != "" {
		t, err := strconv.Atoi(typeStr)
		if err != nil || (t != 1 && t != 2) {
			respondError(w, http.StatusBadRequest, "invalid type parameter: must be 1 (bus) or 2 (tram)")
			return
		}
		vt := domain.VehicleType(t)
		opts.Type = &vt
	}

	opts.Line = r.URL.Query().Get("line")

	if bboxStr := r.URL.Query().Get("bbox"); bboxStr != "" {
		parts := strings.Split(bboxStr, ",")
		if len(parts) != 4 {
			respondError(w, http.StatusBadRequest, "invalid bbox format: expected minLat,minLon,maxLat,maxLon")
			return
		}
		bbox, err := parseBBox(parts)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid bbox values: "+err.Error())
			return
		}
		opts.BBox = bbox
	}

	vehicles := h.store.List(opts)

	respondJSON(w, http.StatusOK, VehiclesResponse{
		Vehicles:   vehicles,
		Count:      len(vehicles),
		ServerTime: time.Now(),
	})
}

func (h *HTTPHandler) GetVehicle(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if key == "" {
		respondError(w, http.StatusBadRequest, "missing vehicle key")
		return
	}

	vehicle, ok := h.store.Get(key)
	if !ok {
		respondError(w, http.StatusNotFound, "vehicle not found")
		return
	}

	respondJSON(w, http.StatusOK, vehicle)
}

func parseBBox(parts []string) (*domain.BoundingBox, error) {
	minLat, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return nil, err
	}
	minLon, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return nil, err
	}
	maxLat, err := strconv.ParseFloat(parts[2], 64)
	if err != nil {
		return nil, err
	}
	maxLon, err := strconv.ParseFloat(parts[3], 64)
	if err != nil {
		return nil, err
	}
	return &domain.BoundingBox{
		MinLat: minLat, MinLon: minLon,
		MaxLat: maxLat, MaxLon: maxLon,
	}, nil
}

type errorResponse struct {
	Error string `json:"error"`
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, errorResponse{Error: message})
}
