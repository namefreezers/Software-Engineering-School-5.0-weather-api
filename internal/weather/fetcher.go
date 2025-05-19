package weather

import (
	"context"
	"fmt"
	"github.com/namefreezers/Software-Engineering-School-5.0-weather-api/internal/weather/types"
	"strings"

	"go.uber.org/zap"
)

type Fetcher interface {
	FetchCurrent(ctx context.Context, city string) (types.Weather, error)
}

// MainConcurrentFetcher will try all its Fetchers in parallel and return the first success.
type MainConcurrentFetcher struct {
	fetchers []Fetcher
	logger   *zap.Logger
}

// NewMainConcurrentFetcher constructs a MainConcurrentFetcher.
func NewMainConcurrentFetcher(logger *zap.Logger, fetchers ...Fetcher) *MainConcurrentFetcher {
	return &MainConcurrentFetcher{
		fetchers: fetchers,
		logger:   logger,
	}
}

func (m *MainConcurrentFetcher) FetchCurrent(ctx context.Context, city string) (types.Weather, error) {
	return RaceFetch(ctx, city, m.fetchers, m.logger)
}

// RaceFetch runs all fetchers in parallel and returns the first successful result.
// It logs each fetcher’s error or success, and aggregates errors if all fail.
func RaceFetch(ctx context.Context, city string, fetchers []Fetcher, logger *zap.Logger) (types.Weather, error) {
	if len(fetchers) == 0 {
		err := fmt.Errorf("no weather providers configured")
		logger.Error("no fetchers", zap.Error(err))
		return types.Weather{}, err
	}

	// Create a cancelable context to stop slow fetchers once we have a winner.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	type result struct {
		w   types.Weather
		err error
	}
	ch := make(chan result, len(fetchers))

	// Fire off one goroutine per provider.
	for _, f := range fetchers {
		go func(f Fetcher) {
			w, err := f.FetchCurrent(ctx, city)
			if err != nil {
				logger.Debug("weather fetcher failed or cancelled", zap.Error(err))
			} else {
				logger.Debug("weather fetcher succeeded",
					zap.Float64("temp", w.Temp),
					zap.Int("humidity", w.Humidity),
					zap.String("desc", w.Description),
				)
			}
			ch <- result{w, err}
		}(f)
	}

	var errs []string
	// Collect the first nil-error result, or aggregate all errors.
	for i := 0; i < len(fetchers); i++ {
		r := <-ch
		if r.err == nil {
			cancel() // stop other fetchers
			logger.Info("using weather result",
				zap.Float64("temp", r.w.Temp),
				zap.Int("humidity", r.w.Humidity),
				zap.String("desc", r.w.Description),
			)
			return r.w, nil
		}
		errs = append(errs, r.err.Error())
	}

	// All providers failed:
	agg := fmt.Errorf("all providers failed: %s", strings.Join(errs, "; "))
	logger.Error("weather fetch failed", zap.Error(agg))
	return types.Weather{}, agg
}
