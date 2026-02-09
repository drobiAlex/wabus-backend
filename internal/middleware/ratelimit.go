package middleware

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// RateLimiter implements a simple token bucket rate limiter per IP
type RateLimiter struct {
	mu        sync.RWMutex
	clients   map[string]*client
	rate      int           // requests per window
	window    time.Duration // time window
	cleanup   time.Duration // cleanup interval
	whitelist map[string]struct{}
	logger    *slog.Logger
}

type client struct {
	tokens    int
	lastReset time.Time
}

// NewRateLimiter creates a rate limiter allowing 'rate' requests per 'window'.
// IPs in whitelist bypass the limiter.
func NewRateLimiter(rate int, window time.Duration, whitelist []string, logger *slog.Logger) *RateLimiter {
	wl := make(map[string]struct{}, len(whitelist))
	for _, ip := range whitelist {
		ip = strings.TrimSpace(ip)
		if ip != "" {
			wl[ip] = struct{}{}
		}
	}

	rl := &RateLimiter{
		clients:   make(map[string]*client),
		rate:      rate,
		window:    window,
		cleanup:   window * 2,
		whitelist: wl,
		logger:    logger.With("component", "rate_limiter"),
	}

	// Start cleanup goroutine
	go rl.cleanupLoop()

	return rl
}

func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.cleanup)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, c := range rl.clients {
			if now.Sub(c.lastReset) > rl.cleanup {
				delete(rl.clients, ip)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) IsWhitelisted(ip string) bool {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	_, ok := rl.whitelist[ip]
	return ok
}

// Allow checks if a request from the given IP should be allowed
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	c, exists := rl.clients[ip]

	if !exists {
		rl.clients[ip] = &client{
			tokens:    rl.rate - 1,
			lastReset: now,
		}
		return true
	}

	// Reset tokens if window has passed
	if now.Sub(c.lastReset) > rl.window {
		c.tokens = rl.rate - 1
		c.lastReset = now
		return true
	}

	// Check if tokens available
	if c.tokens > 0 {
		c.tokens--
		return true
	}

	return false
}

// Middleware returns an HTTP middleware that applies rate limiting
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := getClientIP(r)
		if rl.IsWhitelisted(ip) {
			next.ServeHTTP(w, r)
			return
		}

		if !rl.Allow(ip) {
			rl.logger.Warn("rate limit exceeded", "ip", ip, "path", r.URL.Path)
			w.Header().Set("Retry-After", "60")
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (from reverse proxy). Example: "client, proxy1, proxy2"
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		first := strings.TrimSpace(strings.Split(xff, ",")[0])
		if host, _, err := net.SplitHostPort(first); err == nil {
			return host
		}
		return first
	}

	// Check X-Real-IP header
	if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// Stats returns current rate limiter statistics
func (rl *RateLimiter) Stats() map[string]interface{} {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	return map[string]interface{}{
		"tracked_ips":      len(rl.clients),
		"rate_per_window":  rl.rate,
		"window_seconds":   rl.window.Seconds(),
		"whitelist_entries": len(rl.whitelist),
	}
}
