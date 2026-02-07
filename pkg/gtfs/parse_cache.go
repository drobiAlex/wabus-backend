package gtfs

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

func ParsedCacheDir() string {
	cacheDir := os.Getenv("GTFS_CACHE_DIR")
	if cacheDir == "" {
		cacheDir = filepath.Join(os.TempDir(), "wabus-gtfs-cache")
	}
	return cacheDir
}

func DataFingerprint(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func parsedCachePath(cacheDir, fingerprint string) string {
	return filepath.Join(cacheDir, fmt.Sprintf("gtfs_parsed_%s.gob.gz", fingerprint))
}

func LoadParsedResult(cacheDir, fingerprint string) (*ParseResult, string, error) {
	path := parsedCachePath(cacheDir, fingerprint)
	f, err := os.Open(path)
	if err != nil {
		return nil, path, err
	}
	defer f.Close()

	zr, err := gzip.NewReader(f)
	if err != nil {
		return nil, path, err
	}
	defer zr.Close()

	var result ParseResult
	if err := gob.NewDecoder(zr).Decode(&result); err != nil {
		return nil, path, err
	}

	if result.Routes == nil || result.Stops == nil {
		return nil, path, fmt.Errorf("parsed cache is incomplete")
	}

	return &result, path, nil
}

func SaveParsedResult(cacheDir, fingerprint string, result *ParseResult) (string, error) {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", err
	}

	path := parsedCachePath(cacheDir, fingerprint)
	tmpPath := path + ".tmp"

	f, err := os.Create(tmpPath)
	if err != nil {
		return "", err
	}

	zw, err := gzip.NewWriterLevel(f, gzip.BestSpeed)
	if err != nil {
		f.Close()
		return "", err
	}

	encErr := gob.NewEncoder(zw).Encode(result)
	closeErr := zw.Close()
	fileCloseErr := f.Close()
	if encErr != nil {
		_ = os.Remove(tmpPath)
		return "", encErr
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return "", closeErr
	}
	if fileCloseErr != nil {
		_ = os.Remove(tmpPath)
		return "", fileCloseErr
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}

	return path, nil
}
