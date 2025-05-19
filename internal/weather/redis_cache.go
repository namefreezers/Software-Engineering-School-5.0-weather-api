package weather

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/namefreezers/Software-Engineering-School-5.0-weather-api/internal/weather/types"
	redis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"time"
)

// CachingFetcher decorates another Fetcher with a Redis cache.
type CachingFetcher struct {
	inner  Fetcher
	redis  *redis.Client
	ttl    time.Duration
	logger *zap.Logger
}

// NewCachingFetcher returns a Fetcher that first looks in Redis,
// falling back to inner (e.g. a MainConcurrentFetcher) on cache-miss.
func NewCachingFetcher(inner Fetcher, rdb *redis.Client, ttl time.Duration, logger *zap.Logger) *CachingFetcher {
	return &CachingFetcher{inner: inner, redis: rdb, ttl: ttl, logger: logger}
}

func (c *CachingFetcher) FetchCurrent(ctx context.Context, city string) (types.Weather, error) {
	key := "weather:" + city

	// 1) Try cache
	raw, err := c.redis.Get(ctx, key).Result()
	if err == nil {
		var w types.Weather
		if uerr := json.Unmarshal([]byte(raw), &w); uerr == nil {
			c.logger.Debug("cache hit", zap.String("city", city))
			return w, nil
		} else {
			c.logger.Warn("cache unmarshal failed", zap.Error(uerr))
		}
	} else if !errors.Is(err, redis.Nil) {
		c.logger.Warn("redis GET failed", zap.Error(err))
	}

	// 2) Cache-miss -> delegate to inner
	w, err := c.inner.FetchCurrent(ctx, city)
	if err != nil {
		return w, err
	}

	// 3) Store in cache
	blob, merr := json.Marshal(w)
	if merr != nil {
		c.logger.Warn("json marshal failed", zap.Error(merr))
	} else if serr := c.redis.Set(ctx, key, blob, c.ttl).Err(); serr != nil {
		c.logger.Warn("redis SET failed", zap.Error(serr))
	}

	return w, nil
}
