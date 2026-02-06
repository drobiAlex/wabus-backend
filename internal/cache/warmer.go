package cache

import (
	"context"
	"log/slog"
	"time"

	"wabus/internal/domain"
	"wabus/internal/store"
)

type CacheWarmer struct {
	cache  *RedisCache
	store  *store.GTFSStore
	ttl    time.Duration
	logger *slog.Logger
}

func NewCacheWarmer(cache *RedisCache, store *store.GTFSStore, ttl time.Duration, logger *slog.Logger) *CacheWarmer {
	return &CacheWarmer{
		cache:  cache,
		store:  store,
		ttl:    ttl,
		logger: logger.With("component", "cache_warmer"),
	}
}

func (w *CacheWarmer) WarmAll(ctx context.Context) error {
	start := time.Now()
	w.logger.Info("starting cache warming")

	if err := w.warmSyncData(ctx); err != nil {
		w.logger.Error("failed to warm sync data", "error", err)
	}

	if err := w.warmSchedules(ctx); err != nil {
		w.logger.Error("failed to warm schedules", "error", err)
	}

	if err := w.warmStopLines(ctx); err != nil {
		w.logger.Error("failed to warm stop lines", "error", err)
	}

	w.logger.Info("cache warming completed", "duration_ms", time.Since(start).Milliseconds())
	return nil
}

func (w *CacheWarmer) warmSyncData(ctx context.Context) error {
	start := time.Now()

	syncData := w.buildSyncData()
	if err := w.cache.SetJSONCompressed(ctx, KeySyncFull, syncData, w.ttl); err != nil {
		return err
	}

	w.logger.Info("warmed sync data",
		"routes", len(syncData.Routes),
		"stops", len(syncData.Stops),
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return nil
}

func (w *CacheWarmer) warmSchedules(ctx context.Context) error {
	start := time.Now()
	today := time.Now()
	tomorrow := today.AddDate(0, 0, 1)

	stops := w.store.GetAllStops()
	warmed := 0

	for _, stop := range stops {
		todaySchedule := w.store.GetStopScheduleForDate(stop.ID, today)
		if len(todaySchedule) > 0 {
			if err := w.cache.SetJSON(ctx, KeyScheduleToday(stop.ID), todaySchedule, w.ttl); err != nil {
				w.logger.Debug("failed to cache today schedule", "stop_id", stop.ID, "error", err)
				continue
			}
		}

		tomorrowSchedule := w.store.GetStopScheduleForDate(stop.ID, tomorrow)
		if len(tomorrowSchedule) > 0 {
			if err := w.cache.SetJSON(ctx, KeyScheduleTomorrow(stop.ID), tomorrowSchedule, w.ttl); err != nil {
				w.logger.Debug("failed to cache tomorrow schedule", "stop_id", stop.ID, "error", err)
				continue
			}
		}

		warmed++
	}

	w.logger.Info("warmed schedules",
		"stops_warmed", warmed,
		"total_stops", len(stops),
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return nil
}

func (w *CacheWarmer) warmStopLines(ctx context.Context) error {
	start := time.Now()
	stops := w.store.GetAllStops()
	warmed := 0

	for _, stop := range stops {
		lines := w.store.GetStopLines(stop.ID)
		if len(lines) > 0 {
			if err := w.cache.SetJSON(ctx, KeyStopLines(stop.ID), lines, w.ttl); err != nil {
				w.logger.Debug("failed to cache stop lines", "stop_id", stop.ID, "error", err)
				continue
			}
			warmed++
		}
	}

	w.logger.Info("warmed stop lines",
		"stops_warmed", warmed,
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return nil
}

type SyncData struct {
	Routes        []*domain.Route        `json:"routes"`
	Stops         []*domain.Stop         `json:"stops"`
	Calendars     []*domain.Calendar     `json:"calendars"`
	CalendarDates []*domain.CalendarDate `json:"calendar_dates"`
	Version       string                 `json:"version"`
	GeneratedAt   time.Time              `json:"generated_at"`
}

func (w *CacheWarmer) buildSyncData() *SyncData {
	stats := w.store.GetStats()

	calendars, calendarDates := w.store.GetCalendarsAndDates()

	return &SyncData{
		Routes:        w.store.GetAllRoutes(),
		Stops:         w.store.GetAllStops(),
		Calendars:     calendars,
		CalendarDates: calendarDates,
		Version:       stats.LastUpdate.Format("2006-01-02"),
		GeneratedAt:   time.Now(),
	}
}

func (w *CacheWarmer) ScheduleMidnightRefresh(ctx context.Context) {
	for {
		now := time.Now()
		midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 5, 0, 0, now.Location())
		waitDuration := midnight.Sub(now)

		w.logger.Info("scheduled next cache refresh", "at", midnight, "in", waitDuration)

		select {
		case <-ctx.Done():
			return
		case <-time.After(waitDuration):
			w.logger.Info("midnight cache refresh starting")
			if err := w.WarmAll(ctx); err != nil {
				w.logger.Error("midnight cache refresh failed", "error", err)
			}
		}
	}
}
