package gtfs

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type Downloader struct {
	url       string
	cacheDir  string
	client    *http.Client
	logger    *slog.Logger
}

type cacheMetadata struct {
	ETag         string    `json:"etag"`
	LastModified string    `json:"last_modified"`
	DownloadedAt time.Time `json:"downloaded_at"`
	SizeBytes    int64     `json:"size_bytes"`
}

func NewDownloader(url string, logger *slog.Logger) *Downloader {
	cacheDir := os.Getenv("GTFS_CACHE_DIR")
	if cacheDir == "" {
		cacheDir = filepath.Join(os.TempDir(), "wabus-gtfs-cache")
	}

	return &Downloader{
		url:      url,
		cacheDir: cacheDir,
		client: &http.Client{
			Timeout: 2 * time.Minute,
		},
		logger: logger.With("component", "gtfs_downloader"),
	}
}

func (d *Downloader) Download(ctx context.Context) (*zip.Reader, []byte, error) {
	start := time.Now()

	// Ensure cache directory exists
	if err := os.MkdirAll(d.cacheDir, 0755); err != nil {
		d.logger.Warn("failed to create cache directory", "error", err, "dir", d.cacheDir)
	}

	zipPath := filepath.Join(d.cacheDir, "gtfs.zip")
	metaPath := filepath.Join(d.cacheDir, "gtfs_meta.json")

	// Load existing metadata
	meta := d.loadMetadata(metaPath)

	d.logger.Info("starting GTFS download",
		"url", d.url,
		"cache_dir", d.cacheDir,
		"cached_etag", meta.ETag,
		"cached_last_modified", meta.LastModified,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.url, nil)
	if err != nil {
		d.logger.Error("failed to create request", "error", err)
		return nil, nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("User-Agent", "WaBus-Backend/1.0")

	// Add conditional headers if we have cached data
	if meta.ETag != "" {
		req.Header.Set("If-None-Match", meta.ETag)
	}
	if meta.LastModified != "" {
		req.Header.Set("If-Modified-Since", meta.LastModified)
	}

	d.logger.Debug("sending HTTP request",
		"method", req.Method,
		"url", req.URL.String(),
		"if_none_match", meta.ETag,
		"if_modified_since", meta.LastModified,
	)

	resp, err := d.client.Do(req)
	if err != nil {
		// Try to use cached file on network error
		d.logger.Warn("download failed, attempting to use cached file", "error", err)
		return d.loadFromCache(zipPath)
	}
	defer resp.Body.Close()

	d.logger.Debug("received HTTP response",
		"status_code", resp.StatusCode,
		"content_length", resp.ContentLength,
		"content_type", resp.Header.Get("Content-Type"),
		"etag", resp.Header.Get("ETag"),
		"last_modified", resp.Header.Get("Last-Modified"),
	)

	// 304 Not Modified - use cached file
	if resp.StatusCode == http.StatusNotModified {
		d.logger.Info("GTFS not modified (304), using cached file",
			"cached_size_mb", fmt.Sprintf("%.2f", float64(meta.SizeBytes)/(1024*1024)),
			"cached_at", meta.DownloadedAt.Format(time.RFC3339),
			"duration_ms", time.Since(start).Milliseconds(),
		)
		return d.loadFromCache(zipPath)
	}

	if resp.StatusCode != http.StatusOK {
		d.logger.Error("unexpected HTTP status",
			"status_code", resp.StatusCode,
			"status", resp.Status,
		)
		// Try cached file as fallback
		if reader, data, err := d.loadFromCache(zipPath); err == nil {
			d.logger.Warn("using cached file due to HTTP error")
			return reader, data, nil
		}
		return nil, nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	// Download new file
	readStart := time.Now()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		d.logger.Error("failed to read response body",
			"error", err,
			"duration_ms", time.Since(readStart).Milliseconds(),
		)
		return nil, nil, fmt.Errorf("read body: %w", err)
	}

	d.logger.Debug("read response body",
		"size_bytes", len(data),
		"size_mb", float64(len(data))/(1024*1024),
		"read_duration_ms", time.Since(readStart).Milliseconds(),
	)

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		d.logger.Error("failed to open ZIP archive", "error", err)
		return nil, nil, fmt.Errorf("open zip: %w", err)
	}

	// Save to cache
	d.saveToCache(zipPath, metaPath, data, resp)

	d.logger.Info("GTFS download completed",
		"size_mb", fmt.Sprintf("%.2f", float64(len(data))/(1024*1024)),
		"files_in_archive", len(reader.File),
		"total_duration_ms", time.Since(start).Milliseconds(),
		"cached", true,
	)

	return reader, data, nil
}

func (d *Downloader) loadMetadata(path string) cacheMetadata {
	var meta cacheMetadata
	data, err := os.ReadFile(path)
	if err != nil {
		return meta
	}
	json.Unmarshal(data, &meta)
	return meta
}

func (d *Downloader) loadFromCache(zipPath string) (*zip.Reader, []byte, error) {
	data, err := os.ReadFile(zipPath)
	if err != nil {
		d.logger.Error("failed to read cached ZIP", "error", err, "path", zipPath)
		return nil, nil, fmt.Errorf("read cached zip: %w", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		d.logger.Error("failed to open cached ZIP", "error", err)
		return nil, nil, fmt.Errorf("open cached zip: %w", err)
	}

	d.logger.Info("loaded GTFS from cache",
		"size_mb", fmt.Sprintf("%.2f", float64(len(data))/(1024*1024)),
		"files_in_archive", len(reader.File),
	)

	return reader, data, nil
}

func (d *Downloader) saveToCache(zipPath, metaPath string, data []byte, resp *http.Response) {
	// Save ZIP file
	if err := os.WriteFile(zipPath, data, 0644); err != nil {
		d.logger.Warn("failed to cache ZIP file", "error", err, "path", zipPath)
		return
	}

	// Save metadata
	meta := cacheMetadata{
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
		DownloadedAt: time.Now(),
		SizeBytes:    int64(len(data)),
	}

	metaData, _ := json.Marshal(meta)
	if err := os.WriteFile(metaPath, metaData, 0644); err != nil {
		d.logger.Warn("failed to save cache metadata", "error", err, "path", metaPath)
		return
	}

	d.logger.Debug("cached GTFS file",
		"zip_path", zipPath,
		"meta_path", metaPath,
		"etag", meta.ETag,
		"last_modified", meta.LastModified,
	)
}
