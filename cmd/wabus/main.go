package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"wabus/internal/cache"
	"wabus/internal/config"
	"wabus/internal/handler"
	"wabus/internal/hub"
	"wabus/internal/ingestor"
	"wabus/internal/middleware"
	"wabus/internal/store"
	"wabus/pkg/warsawapi"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))
	slog.SetDefault(logger)

	logger.Info("starting wabus server",
		"log_level", cfg.LogLevel.String(),
		"http_addr", cfg.HTTPAddr,
		"gtfs_enabled", cfg.GTFSEnabled,
		"redis_enabled", cfg.RedisEnabled,
	)

	var redisCache *cache.RedisCache
	if cfg.RedisEnabled {
		var err error
		redisCache, err = cache.NewRedisCache(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB, logger)
		if err != nil {
			logger.Error("failed to connect to Redis", "error", err)
			logger.Warn("continuing without Redis cache")
			redisCache = nil
		} else {
			logger.Info("connected to Redis", "addr", cfg.RedisAddr)
		}
	}

	vehicleStore := store.New(cfg.VehicleStaleAfter)
	gtfsStore := store.NewGTFSStore()
	wsHub := hub.NewHub(logger)
	apiClient := warsawapi.New(cfg.WarsawAPIBaseURL, cfg.WarsawAPIKey, cfg.WarsawResourceID)
	ing := ingestor.New(apiClient, vehicleStore, wsHub, cfg, logger)

	var gtfsIng *ingestor.GTFSIngestor
	var cacheWarmer *cache.CacheWarmer
	if cfg.GTFSEnabled {
		gtfsIng = ingestor.NewGTFSIngestor(cfg.GTFSURL, gtfsStore, cfg.GTFSUpdateInterval, logger)

		if redisCache != nil {
			cacheWarmer = cache.NewCacheWarmer(redisCache, gtfsStore, cfg.CacheTTL, logger)
			gtfsIng.SetOnUpdate(func(ctx context.Context) {
				logger.Info("GTFS data updated, warming cache")
				if err := cacheWarmer.WarmAll(ctx); err != nil {
					logger.Error("cache warming failed", "error", err)
				}
			})
		}
	}

	httpHandler := handler.NewHTTPHandler(vehicleStore)
	wsHandler := handler.NewWSHandler(wsHub, vehicleStore, logger)
	healthHandler := handler.NewHealthHandler(ing, vehicleStore)
	gtfsHandler := handler.NewGTFSHandler(gtfsStore, redisCache, logger)
	statsHandler := handler.NewStatsHandler(vehicleStore, gtfsStore)

	// Rate limiter (configurable), with optional IP whitelist.
	rateLimiter := middleware.NewRateLimiter(cfg.RateLimitPerWindow, cfg.RateLimitWindow, cfg.RateLimitWhitelist, logger)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /v1/vehicles", httpHandler.ListVehicles)
	mux.HandleFunc("GET /v1/vehicles/{key}", httpHandler.GetVehicle)
	mux.HandleFunc("/v1/ws", wsHandler.ServeWS)

	mux.HandleFunc("GET /v1/routes", gtfsHandler.ListRoutes)
	mux.HandleFunc("GET /v1/routes/{line}", gtfsHandler.GetRoute)
	mux.HandleFunc("GET /v1/routes/{line}/shape", gtfsHandler.GetRouteShape)
	mux.HandleFunc("GET /v1/routes/{line}/stops", gtfsHandler.GetRouteStops)
	mux.HandleFunc("GET /v1/stops", gtfsHandler.ListStops)
	mux.HandleFunc("GET /v1/stops/{id}", gtfsHandler.GetStop)
	mux.HandleFunc("GET /v1/stops/{id}/schedule", gtfsHandler.GetStopSchedule)
	mux.HandleFunc("GET /v1/stops/{id}/lines", gtfsHandler.GetStopLines)
	mux.HandleFunc("GET /v1/gtfs/stats", gtfsHandler.GetStats)

	mux.HandleFunc("GET /v1/sync", gtfsHandler.GetSync)
	mux.HandleFunc("GET /v1/sync/check", gtfsHandler.CheckSync)

	mux.HandleFunc("GET /healthz", healthHandler.Healthz)
	mux.HandleFunc("GET /readyz", healthHandler.Readyz)
	mux.HandleFunc("GET /stats", statsHandler.GetStats)

	// Apply middleware chain: CORS -> Gzip -> RateLimit -> Handler
	finalHandler := handler.CORSMiddleware(
		handler.GzipMiddleware(
			rateLimiter.Middleware(mux),
		),
	)

	srv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      finalHandler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go wsHub.Run(ctx)

	go ing.Run(ctx)

	if gtfsIng != nil {
		go gtfsIng.Start(ctx)
	}

	if cacheWarmer != nil {
		go cacheWarmer.ScheduleMidnightRefresh(ctx)
	}

	go func() {
		logger.Info("starting HTTP server", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error", "error", err)
			cancel()
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("shutdown signal received")

	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server shutdown error", "error", err)
	}

	if redisCache != nil {
		if err := redisCache.Close(); err != nil {
			logger.Error("Redis close error", "error", err)
		}
	}

	logger.Info("shutdown complete")
}
