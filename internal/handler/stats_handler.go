package handler

import (
	"encoding/json"
	"net/http"
	"runtime"
	"sync/atomic"
	"time"

	"wabus/internal/store"
)

// Stats tracks server-wide metrics
type Stats struct {
	startTime        time.Time
	requestCount     atomic.Int64
	wsConnections    atomic.Int64
	wsMessagesIn     atomic.Int64
	wsMessagesOut    atomic.Int64
	cacheHits        atomic.Int64
	cacheMisses      atomic.Int64
	rateLimitBlocked atomic.Int64
}

// Global stats instance
var ServerStats = &Stats{
	startTime: time.Now(),
}

func (s *Stats) IncRequests()         { s.requestCount.Add(1) }
func (s *Stats) IncWSConnections()    { s.wsConnections.Add(1) }
func (s *Stats) DecWSConnections()    { s.wsConnections.Add(-1) }
func (s *Stats) IncWSMessagesIn()     { s.wsMessagesIn.Add(1) }
func (s *Stats) IncWSMessagesOut()    { s.wsMessagesOut.Add(1) }
func (s *Stats) IncCacheHits()        { s.cacheHits.Add(1) }
func (s *Stats) IncCacheMisses()      { s.cacheMisses.Add(1) }
func (s *Stats) IncRateLimitBlocked() { s.rateLimitBlocked.Add(1) }

type StatsHandler struct {
	vehicleStore *store.Store
	gtfsStore    *store.GTFSStore
}

func NewStatsHandler(vehicleStore *store.Store, gtfsStore *store.GTFSStore) *StatsHandler {
	return &StatsHandler{
		vehicleStore: vehicleStore,
		gtfsStore:    gtfsStore,
	}
}

type StatsResponse struct {
	Server    ServerStatsResponse    `json:"server"`
	Vehicles  VehicleStatsResponse   `json:"vehicles"`
	GTFS      GTFSStatsResponse      `json:"gtfs"`
	WebSocket WebSocketStatsResponse `json:"websocket"`
	Cache     CacheStatsResponse     `json:"cache"`
	Go        GoStatsResponse        `json:"go"`
}

type ServerStatsResponse struct {
	Uptime         string    `json:"uptime"`
	UptimeSeconds  float64   `json:"uptime_seconds"`
	StartTime      time.Time `json:"start_time"`
	RequestCount   int64     `json:"request_count"`
	RateLimited    int64     `json:"rate_limited"`
	Version        string    `json:"version"`
}

type VehicleStatsResponse struct {
	Total int `json:"total"`
	Buses int `json:"buses"`
	Trams int `json:"trams"`
}

type GTFSStatsResponse struct {
	Routes     int       `json:"routes"`
	Stops      int       `json:"stops"`
	Shapes     int       `json:"shapes"`
	IsLoaded   bool      `json:"is_loaded"`
	LastUpdate time.Time `json:"last_update"`
}

type WebSocketStatsResponse struct {
	Connections int64 `json:"connections"`
	MessagesIn  int64 `json:"messages_in"`
	MessagesOut int64 `json:"messages_out"`
}

type CacheStatsResponse struct {
	Hits   int64   `json:"hits"`
	Misses int64   `json:"misses"`
	Ratio  float64 `json:"hit_ratio"`
}

type GoStatsResponse struct {
	Goroutines   int    `json:"goroutines"`
	HeapAlloc    uint64 `json:"heap_alloc_bytes"`
	HeapAllocMB  float64 `json:"heap_alloc_mb"`
	NumGC        uint32 `json:"num_gc"`
	GoVersion    string `json:"go_version"`
}

func (h *StatsHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	ServerStats.IncRequests()

	uptime := time.Since(ServerStats.startTime)

	// Vehicle stats
	buses, trams := h.vehicleStore.CountByType()

	// GTFS stats
	gtfsStats := h.gtfsStore.GetStats()

	// Memory stats
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	// Cache ratio
	hits := ServerStats.cacheHits.Load()
	misses := ServerStats.cacheMisses.Load()
	var ratio float64
	if total := hits + misses; total > 0 {
		ratio = float64(hits) / float64(total)
	}

	response := StatsResponse{
		Server: ServerStatsResponse{
			Uptime:        uptime.Round(time.Second).String(),
			UptimeSeconds: uptime.Seconds(),
			StartTime:     ServerStats.startTime,
			RequestCount:  ServerStats.requestCount.Load(),
			RateLimited:   ServerStats.rateLimitBlocked.Load(),
			Version:       "1.0.0",
		},
		Vehicles: VehicleStatsResponse{
			Total: buses + trams,
			Buses: buses,
			Trams: trams,
		},
		GTFS: GTFSStatsResponse{
			Routes:     gtfsStats.RoutesCount,
			Stops:      gtfsStats.StopsCount,
			Shapes:     gtfsStats.ShapesCount,
			IsLoaded:   gtfsStats.IsLoaded,
			LastUpdate: gtfsStats.LastUpdate,
		},
		WebSocket: WebSocketStatsResponse{
			Connections: ServerStats.wsConnections.Load(),
			MessagesIn:  ServerStats.wsMessagesIn.Load(),
			MessagesOut: ServerStats.wsMessagesOut.Load(),
		},
		Cache: CacheStatsResponse{
			Hits:   hits,
			Misses: misses,
			Ratio:  ratio,
		},
		Go: GoStatsResponse{
			Goroutines:  runtime.NumGoroutine(),
			HeapAlloc:   mem.HeapAlloc,
			HeapAllocMB: float64(mem.HeapAlloc) / 1024 / 1024,
			NumGC:       mem.NumGC,
			GoVersion:   runtime.Version(),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")
	json.NewEncoder(w).Encode(response)
}
