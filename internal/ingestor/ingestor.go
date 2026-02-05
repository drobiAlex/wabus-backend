package ingestor

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"wabus/internal/config"
	"wabus/internal/domain"
	"wabus/internal/hub"
	"wabus/internal/store"
	"wabus/pkg/warsawapi"
)

type Broadcaster interface {
	Broadcast(deltas []domain.VehicleDelta)
}

type Ingestor struct {
	client      *warsawapi.Client
	store       *store.Store
	broadcaster Broadcaster
	config      *config.Config
	logger      *slog.Logger
	zoomLevel   int

	ready   bool
	readyMu sync.RWMutex
}

func New(client *warsawapi.Client, store *store.Store, broadcaster Broadcaster, cfg *config.Config, logger *slog.Logger) *Ingestor {
	return &Ingestor{
		client:      client,
		store:       store,
		broadcaster: broadcaster,
		config:      cfg,
		logger:      logger,
		zoomLevel:   cfg.TileZoomLevel,
	}
}

func (i *Ingestor) Run(ctx context.Context) {
	ticker := time.NewTicker(i.config.PollInterval)
	defer ticker.Stop()

	pruneTicker := time.NewTicker(i.config.PollInterval * 3)
	defer pruneTicker.Stop()

	i.poll(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			i.poll(ctx)
		case <-pruneTicker.C:
			i.prune()
		}
	}
}

func (i *Ingestor) poll(ctx context.Context) {
	var wg sync.WaitGroup
	var busesMu, tramsMu sync.Mutex
	var buses, trams []*domain.Vehicle
	var busErr, tramErr error

	wg.Add(2)

	go func() {
		defer wg.Done()
		result, err := i.client.Fetch(ctx, domain.VehicleTypeBus)
		busesMu.Lock()
		buses, busErr = result, err
		busesMu.Unlock()
	}()

	go func() {
		defer wg.Done()
		result, err := i.client.Fetch(ctx, domain.VehicleTypeTram)
		tramsMu.Lock()
		trams, tramErr = result, err
		tramsMu.Unlock()
	}()

	wg.Wait()

	if busErr != nil {
		i.logger.Error("failed to fetch buses", "error", busErr)
	}
	if tramErr != nil {
		i.logger.Error("failed to fetch trams", "error", tramErr)
	}

	allVehicles := make([]*domain.Vehicle, 0, len(buses)+len(trams))
	allVehicles = append(allVehicles, buses...)
	allVehicles = append(allVehicles, trams...)

	for _, v := range allVehicles {
		v.TileID = hub.TileID(v.Lat, v.Lon, i.zoomLevel)
	}

	deltas := i.store.Update(allVehicles)

	if i.broadcaster != nil {
		i.broadcaster.Broadcast(deltas)
	}

	if !i.IsReady() && (busErr == nil || tramErr == nil) {
		i.setReady(true)
		i.logger.Info("ingestor ready", "buses", len(buses), "trams", len(trams))
	}

	i.logger.Debug("poll completed",
		"buses", len(buses),
		"trams", len(trams),
		"deltas", len(deltas),
		"total", i.store.Count(),
	)
}

func (i *Ingestor) prune() {
	deltas := i.store.PruneStale()
	if len(deltas) > 0 {
		if i.broadcaster != nil {
			i.broadcaster.Broadcast(deltas)
		}
		i.logger.Info("pruned stale vehicles", "count", len(deltas))
	}
}

func (i *Ingestor) IsReady() bool {
	i.readyMu.RLock()
	defer i.readyMu.RUnlock()
	return i.ready
}

func (i *Ingestor) setReady(ready bool) {
	i.readyMu.Lock()
	defer i.readyMu.Unlock()
	i.ready = ready
}
