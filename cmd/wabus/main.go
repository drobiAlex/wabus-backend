package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"wabus/internal/config"
	"wabus/internal/handler"
	"wabus/internal/hub"
	"wabus/internal/ingestor"
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
	)

	vehicleStore := store.New(cfg.VehicleStaleAfter)
	gtfsStore := store.NewGTFSStore()
	wsHub := hub.NewHub(logger)
	apiClient := warsawapi.New(cfg.WarsawAPIBaseURL, cfg.WarsawAPIKey, cfg.WarsawResourceID)
	ing := ingestor.New(apiClient, vehicleStore, wsHub, cfg, logger)

	var gtfsIng *ingestor.GTFSIngestor
	if cfg.GTFSEnabled {
		gtfsIng = ingestor.NewGTFSIngestor(cfg.GTFSURL, gtfsStore, cfg.GTFSUpdateInterval, logger)
	}

	httpHandler := handler.NewHTTPHandler(vehicleStore)
	wsHandler := handler.NewWSHandler(wsHub, vehicleStore, logger)
	healthHandler := handler.NewHealthHandler(ing, vehicleStore)
	gtfsHandler := handler.NewGTFSHandler(gtfsStore, logger)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /v1/vehicles", httpHandler.ListVehicles)
	mux.HandleFunc("GET /v1/vehicles/{key}", httpHandler.GetVehicle)
	mux.HandleFunc("/v1/ws", wsHandler.ServeWS)

	mux.HandleFunc("GET /v1/routes", gtfsHandler.ListRoutes)
	mux.HandleFunc("GET /v1/routes/{line}", gtfsHandler.GetRoute)
	mux.HandleFunc("GET /v1/routes/{line}/shape", gtfsHandler.GetRouteShape)
	mux.HandleFunc("GET /v1/stops", gtfsHandler.ListStops)
	mux.HandleFunc("GET /v1/stops/{id}", gtfsHandler.GetStop)
	mux.HandleFunc("GET /v1/stops/{id}/schedule", gtfsHandler.GetStopSchedule)
	mux.HandleFunc("GET /v1/stops/{id}/lines", gtfsHandler.GetStopLines)
	mux.HandleFunc("GET /v1/gtfs/stats", gtfsHandler.GetStats)

	mux.HandleFunc("GET /healthz", healthHandler.Healthz)
	mux.HandleFunc("GET /readyz", healthHandler.Readyz)

	srv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      mux,
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

	logger.Info("shutdown complete")
}
