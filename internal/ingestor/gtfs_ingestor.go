package ingestor

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"wabus/internal/store"
	"wabus/pkg/gtfs"
)

type GTFSIngestor struct {
	downloader     *gtfs.Downloader
	parser         *gtfs.Parser
	store          *store.GTFSStore
	updateInterval time.Duration
	logger         *slog.Logger
	onUpdate       func(context.Context)

	ready   bool
	readyMu sync.RWMutex
}

func NewGTFSIngestor(url string, store *store.GTFSStore, updateInterval time.Duration, logger *slog.Logger) *GTFSIngestor {
	ingestorLogger := logger.With("component", "gtfs_ingestor")
	return &GTFSIngestor{
		downloader:     gtfs.NewDownloader(url, logger),
		parser:         gtfs.NewParser(logger),
		store:          store,
		updateInterval: updateInterval,
		logger:         ingestorLogger,
	}
}

func (i *GTFSIngestor) Start(ctx context.Context) {
	i.update(ctx)

	ticker := time.NewTicker(i.updateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			i.update(ctx)
		}
	}
}

func (i *GTFSIngestor) update(ctx context.Context) {
	i.logger.Info("starting GTFS update")
	start := time.Now()

	reader, _, err := i.downloader.Download(ctx)
	if err != nil {
		i.logger.Error("failed to download GTFS", "error", err)
		return
	}

	downloadDuration := time.Since(start)
	i.logger.Info("GTFS downloaded", "duration", downloadDuration)

	parseStart := time.Now()
	result, err := i.parser.Parse(reader)
	if err != nil {
		i.logger.Error("failed to parse GTFS", "error", err)
		return
	}

	parseDuration := time.Since(parseStart)

	i.store.UpdateAll(result.Routes, result.Shapes, result.Stops, result.RouteShapes, result.StopSchedules, result.StopLines, result.RouteStops, result.RouteTripTimes, result.Calendars, result.CalendarDates)

	if !i.IsReady() {
		i.setReady(true)
	}

	if i.onUpdate != nil {
		i.onUpdate(ctx)
	}

	i.logger.Info("GTFS update completed",
		"download_duration", downloadDuration,
		"parse_duration", parseDuration,
		"total_duration", time.Since(start),
		"routes", len(result.Routes),
		"shapes", len(result.Shapes),
		"stops", len(result.Stops),
		"stops_with_schedules", len(result.StopSchedules),
		"calendars", len(result.Calendars),
	)
}

func (i *GTFSIngestor) IsReady() bool {
	i.readyMu.RLock()
	defer i.readyMu.RUnlock()
	return i.ready
}

func (i *GTFSIngestor) setReady(ready bool) {
	i.readyMu.Lock()
	defer i.readyMu.Unlock()
	i.ready = ready
}

func (i *GTFSIngestor) SetOnUpdate(fn func(context.Context)) {
	i.onUpdate = fn
}
