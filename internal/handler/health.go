package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"wabus/internal/ingestor"
	"wabus/internal/store"
)

type HealthHandler struct {
	ingestor *ingestor.Ingestor
	store    *store.Store
}

func NewHealthHandler(ing *ingestor.Ingestor, s *store.Store) *HealthHandler {
	return &HealthHandler{
		ingestor: ing,
		store:    s,
	}
}

func (h *HealthHandler) Healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

type ReadyResponse struct {
	Ready        bool      `json:"ready"`
	VehicleCount int       `json:"vehicleCount"`
	ServerTime   time.Time `json:"serverTime"`
}

func (h *HealthHandler) Readyz(w http.ResponseWriter, r *http.Request) {
	ready := h.ingestor.IsReady()
	status := http.StatusOK
	if !ready {
		status = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ReadyResponse{
		Ready:        ready,
		VehicleCount: h.store.Count(),
		ServerTime:   time.Now(),
	})
}
