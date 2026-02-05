package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr        string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration

	WarsawAPIBaseURL string
	WarsawAPIKey     string
	WarsawResourceID string
	PollInterval     time.Duration

	VehicleStaleAfter time.Duration
	TileZoomLevel     int

	GTFSEnabled        bool
	GTFSURL            string
	GTFSUpdateInterval time.Duration
}

func Load() (*Config, error) {
	apiKey := os.Getenv("WARSAW_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("WARSAW_API_KEY environment variable is required")
	}

	return &Config{
		HTTPAddr:        getEnv("HTTP_ADDR", ":8080"),
		ReadTimeout:     getDurationEnv("READ_TIMEOUT", 10*time.Second),
		WriteTimeout:    getDurationEnv("WRITE_TIMEOUT", 10*time.Second),
		ShutdownTimeout: getDurationEnv("SHUTDOWN_TIMEOUT", 30*time.Second),

		WarsawAPIBaseURL: getEnv("WARSAW_API_URL", "https://api.um.warszawa.pl/api/action/busestrams_get"),
		WarsawAPIKey:     apiKey,
		WarsawResourceID: getEnv("WARSAW_RESOURCE_ID", "f2e5503e-927d-4ad3-9500-4ab9e55deb59"),
		PollInterval:     getDurationEnv("POLL_INTERVAL", 10*time.Second),

		VehicleStaleAfter: getDurationEnv("VEHICLE_STALE_AFTER", 5*time.Minute),
		TileZoomLevel:     getIntEnv("TILE_ZOOM_LEVEL", 14),

		GTFSEnabled:        getBoolEnv("GTFS_ENABLED", true),
		GTFSURL:            getEnv("GTFS_URL", "https://mkuran.pl/gtfs/warsaw.zip"),
		GTFSUpdateInterval: getDurationEnv("GTFS_UPDATE_INTERVAL", 24*time.Hour),
	}, nil
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getDurationEnv(key string, defaultVal time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return defaultVal
}

func getIntEnv(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}

func getBoolEnv(key string, defaultVal bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return defaultVal
}
