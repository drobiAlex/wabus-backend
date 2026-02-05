package gtfs

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Downloader struct {
	url    string
	client *http.Client
}

func NewDownloader(url string) *Downloader {
	return &Downloader{
		url: url,
		client: &http.Client{
			Timeout: 2 * time.Minute,
		},
	}
}

func (d *Downloader) Download(ctx context.Context) (*zip.Reader, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.url, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("download gtfs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read body: %w", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, nil, fmt.Errorf("open zip: %w", err)
	}

	return reader, data, nil
}
