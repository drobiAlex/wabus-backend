package cache

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisCache struct {
	client *redis.Client
	prefix string
	logger *slog.Logger
}

func NewRedisCache(addr, password string, db int, logger *slog.Logger) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis connection failed: %w", err)
	}

	return &RedisCache{
		client: client,
		prefix: "wabus:",
		logger: logger.With("component", "redis_cache"),
	}, nil
}

func (c *RedisCache) Close() error {
	return c.client.Close()
}

func (c *RedisCache) key(k string) string {
	return c.prefix + k
}

func (c *RedisCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	start := time.Now()
	err := c.client.Set(ctx, c.key(key), value, ttl).Err()
	if err != nil {
		c.logger.Error("cache set failed", "key", key, "error", err)
		return err
	}
	c.logger.Debug("cache set", "key", key, "size_bytes", len(value), "ttl", ttl, "duration_ms", time.Since(start).Milliseconds())
	return nil
}

func (c *RedisCache) Get(ctx context.Context, key string) ([]byte, error) {
	start := time.Now()
	val, err := c.client.Get(ctx, c.key(key)).Bytes()
	if err == redis.Nil {
		c.logger.Debug("cache miss", "key", key)
		return nil, nil
	}
	if err != nil {
		c.logger.Error("cache get failed", "key", key, "error", err)
		return nil, err
	}
	c.logger.Debug("cache hit", "key", key, "size_bytes", len(val), "duration_ms", time.Since(start).Milliseconds())
	return val, nil
}

func (c *RedisCache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, c.key(key)).Err()
}

func (c *RedisCache) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}
	return c.Set(ctx, key, data, ttl)
}

func (c *RedisCache) GetJSON(ctx context.Context, key string, dest interface{}) (bool, error) {
	data, err := c.Get(ctx, key)
	if err != nil {
		return false, err
	}
	if data == nil {
		return false, nil
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return false, fmt.Errorf("json unmarshal: %w", err)
	}
	return true, nil
}

func (c *RedisCache) SetCompressed(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	compressed, err := gzipCompress(value)
	if err != nil {
		return fmt.Errorf("compress: %w", err)
	}
	c.logger.Debug("compressed data", "key", key, "original_size", len(value), "compressed_size", len(compressed))
	return c.Set(ctx, key, compressed, ttl)
}

func (c *RedisCache) GetCompressed(ctx context.Context, key string) ([]byte, error) {
	data, err := c.Get(ctx, key)
	if err != nil || data == nil {
		return data, err
	}
	return gzipDecompress(data)
}

func (c *RedisCache) SetJSONCompressed(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}
	return c.SetCompressed(ctx, key, data, ttl)
}

func (c *RedisCache) GetJSONCompressed(ctx context.Context, key string, dest interface{}) (bool, error) {
	data, err := c.GetCompressed(ctx, key)
	if err != nil {
		return false, err
	}
	if data == nil {
		return false, nil
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return false, fmt.Errorf("json unmarshal: %w", err)
	}
	return true, nil
}

func (c *RedisCache) DeletePattern(ctx context.Context, pattern string) error {
	iter := c.client.Scan(ctx, 0, c.key(pattern), 0).Iterator()
	for iter.Next(ctx) {
		if err := c.client.Del(ctx, iter.Val()).Err(); err != nil {
			return err
		}
	}
	return iter.Err()
}

func gzipCompress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(data); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func gzipDecompress(data []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	return io.ReadAll(gz)
}
