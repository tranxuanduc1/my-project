package cache

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"myproject/order/internal/domain"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type RedisProductCache struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRedisProductCache(addr string, ttl time.Duration) *RedisProductCache {
	client := redis.NewClient(&redis.Options{Addr: addr})
	if err := redisotel.InstrumentTracing(client); err != nil {
		slog.Warn("redis tracing instrumentation failed", "error", err)
	}
	if err := redisotel.InstrumentMetrics(client); err != nil {
		slog.Warn("redis metrics instrumentation failed", "error", err)
	}
	return &RedisProductCache{client: client, ttl: ttl}
}

func (c *RedisProductCache) GetProduct(ctx context.Context, id string) (domain.Product, bool) {
	raw, err := c.client.Get(ctx, "product:"+id).Result()
	if err != nil {
		recordCacheLookup(ctx, "miss")
		return domain.Product{}, false
	}
	var product domain.Product
	if json.Unmarshal([]byte(raw), &product) != nil {
		recordCacheLookup(ctx, "miss")
		return domain.Product{}, false
	}
	recordCacheLookup(ctx, "hit")
	return product, true
}

func (c *RedisProductCache) SetProduct(ctx context.Context, product domain.Product) {
	body, err := json.Marshal(product)
	if err != nil {
		return
	}
	_ = c.client.Set(ctx, "product:"+product.ID.String(), body, c.ttl).Err()
}

func (c *RedisProductCache) DeleteProduct(ctx context.Context, id string) {
	_ = c.client.Del(ctx, "product:"+id).Err()
}

var cacheLookups metric.Int64Counter

func init() {
	var err error
	cacheLookups, err = otel.Meter("myproject/order/cache").Int64Counter(
		"cache.lookup.count",
		metric.WithDescription("Product cache lookups by bounded result."),
		metric.WithUnit("{lookup}"),
	)
	if err != nil {
		slog.Warn("cache metric initialization failed", "error", err)
	}
}

func recordCacheLookup(ctx context.Context, result string) {
	if cacheLookups == nil {
		return
	}
	cacheLookups.Add(ctx, 1, metric.WithAttributes(
		attribute.String("cache.name", "product"),
		attribute.String("result", result),
	))
}
