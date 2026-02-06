package gtfs

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type Downloader struct {
	url    string
	client *http.Client
	logger *slog.Logger
}

func NewDownloader(url string, logger *slog.Logger) *Downloader {
	return &Downloader{
		url: url,
		client: &http.Client{
			Timeout: 2 * time.Minute,
		},
		logger: logger.With("component", "gtfs_downloader"),
	}
}

func (d *Downloader) Download(ctx context.Context) (*zip.Reader, []byte, error) {
	start := time.Now()
	d.logger.Info("starting GTFS download",
		"url", d.url,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.url, nil)
	if err != nil {
		d.logger.Error("failed to create request", "error", err)
		return nil, nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("User-Agent", "WaBus-Backend/1.0")

	d.logger.Debug("sending HTTP request",
		"method", req.Method,
		"url", req.URL.String(),
	)

	resp, err := d.client.Do(req)
	if err != nil {
		d.logger.Error("failed to download GTFS",
			"error", err,
			"duration_ms", time.Since(start).Milliseconds(),
		)
		return nil, nil, fmt.Errorf("download gtfs: %w", err)
	}
	defer resp.Body.Close()

	d.logger.Debug("received HTTP response",
		"status_code", resp.StatusCode,
		"content_length", resp.ContentLength,
		"content_type", resp.Header.Get("Content-Type"),
	)

	if resp.StatusCode != http.StatusOK {
		d.logger.Error("unexpected HTTP status",
			"status_code", resp.StatusCode,
			"status", resp.Status,
		)
		return nil, nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

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

	d.logger.Info("GTFS download completed",
		"size_mb", fmt.Sprintf("%.2f", float64(len(data))/(1024*1024)),
		"files_in_archive", len(reader.File),
		"total_duration_ms", time.Since(start).Milliseconds(),
	)

	return reader, data, nil
}
