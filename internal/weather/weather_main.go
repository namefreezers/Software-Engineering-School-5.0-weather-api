package weather

import (
	"context"
	"fmt"
	"github.com/namefreezers/Software-Engineering-School-5.0-weather-api/internal/config"
	"github.com/namefreezers/Software-Engineering-School-5.0-weather-api/internal/weather/openweathermap"
	"github.com/namefreezers/Software-Engineering-School-5.0-weather-api/internal/weather/weatherapi"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// BuildCachingFetcher constructs a Fetcher that:
// 1) Builds the two concrete provider clients (OpenWeatherMap & WeatherAPI.com)
// 2) Wraps them in a concurrent “race to first” fetcher
// 3) Decorates that with a Redis cache (5 minute TTL)
// It reads REDIS_PASSWORD, OPENWEATHERMAP_API_KEY and WEATHERAPI_COM_API_KEY from the environment.
func BuildCachingFetcher(cfg *config.Config, logger *zap.Logger) (Fetcher, error) {
	var fetchers []Fetcher
	var errs []string

	// OpenWeatherMap client
	if owm, err := openweathermap.NewClient(cfg); err != nil {
		logger.Warn("openweathermap client not configured", zap.Error(err))
		errs = append(errs, fmt.Sprintf("owm: %v", err))
	} else {
		fetchers = append(fetchers, owm)
	}

	// WeatherAPI.com client
	if wap, err := weatherapi.NewClient(cfg); err != nil {
		logger.Warn("weatherapi client not configured", zap.Error(err))
		errs = append(errs, fmt.Sprintf("weatherapi: %v", err))
	} else {
		fetchers = append(fetchers, wap)
	}

	if len(fetchers) == 0 {
		return nil, fmt.Errorf("no weather providers available: %s", strings.Join(errs, "; "))
	}

	// 2) Race‐to‐first fetcher
	base := NewMainConcurrentFetcher(logger, fetchers...)

	// 3) Redis client & cache decorator
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       0,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	return NewCachingFetcher(base, rdb, 5*time.Minute, logger), nil
}
