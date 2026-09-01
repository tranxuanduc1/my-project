package cache

import (
	"context"
	"encoding/json"
	"time"

	"myproject/order/internal/domain"

	"github.com/redis/go-redis/v9"
)

type RedisProductCache struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRedisProductCache(addr string, ttl time.Duration) *RedisProductCache {
	return &RedisProductCache{client: redis.NewClient(&redis.Options{Addr: addr}), ttl: ttl}
}

func (c *RedisProductCache) GetProduct(ctx context.Context, id string) (domain.Product, bool) {
	raw, err := c.client.Get(ctx, "product:"+id).Result()
	if err != nil {
		return domain.Product{}, false
	}
	var product domain.Product
	if json.Unmarshal([]byte(raw), &product) != nil {
		return domain.Product{}, false
	}
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
