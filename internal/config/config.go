package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	LogLevel        slog.Level
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

	RedisEnabled     bool
	RedisAddr        string
	RedisPassword    string
	RedisDB          int
	CacheTTL         time.Duration
	CacheWarmOnStart bool

	RateLimitPerWindow int
	RateLimitWindow    time.Duration
	RateLimitWhitelist []string
}

func Load() (*Config, error) {
	apiKey := os.Getenv("WARSAW_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("WARSAW_API_KEY environment variable is required")
	}

	return &Config{
		LogLevel:        getLogLevelEnv("LOG_LEVEL", slog.LevelInfo),
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

		RedisEnabled:     getBoolEnv("REDIS_ENABLED", false),
		RedisAddr:        getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:    getEnv("REDIS_PASSWORD", ""),
		RedisDB:          getIntEnv("REDIS_DB", 0),
		CacheTTL:         getDurationEnv("CACHE_TTL", 24*time.Hour),
		CacheWarmOnStart: getBoolEnv("CACHE_WARM_ON_START", true),

		RateLimitPerWindow: getIntEnv("RATE_LIMIT_PER_WINDOW", 120),
		RateLimitWindow:    getDurationEnv("RATE_LIMIT_WINDOW", time.Minute),
		RateLimitWhitelist: getCSVEnv("RATE_LIMIT_WHITELIST"),
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

func getLogLevelEnv(key string, defaultVal slog.Level) slog.Level {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}

	switch strings.ToLower(v) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return defaultVal
	}
}

func getCSVEnv(key string) []string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return nil
	}

	parts := strings.Split(v, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			result = append(result, t)
		}
	}
	return result
}
