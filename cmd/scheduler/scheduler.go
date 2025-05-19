package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"github.com/namefreezers/Software-Engineering-School-5.0-weather-api/internal/config"
	"github.com/namefreezers/Software-Engineering-School-5.0-weather-api/internal/email"
	"github.com/namefreezers/Software-Engineering-School-5.0-weather-api/internal/repository"
	"github.com/namefreezers/Software-Engineering-School-5.0-weather-api/internal/weather"
)

func main() {
	// 1) Load config (includes BASE_URL)
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}

	// 2) Init logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("cannot initialize logger: %v", err)
	}
	defer logger.Sync()

	// 3) Open DB
	db, err := repository.OpenDB(cfg.DatabaseURL)
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}

	// 4) Wire up repository, email sender, weather fetcher
	subRepo := repository.NewSubscriptionRepository(db, logger)

	smtpSender, err := email.NewSMTPSender(cfg, logger)
	if err != nil {
		logger.Fatal("failed to initialize SMTP sender", zap.Error(err))
	}

	weatherFetcher, err := weather.BuildCachingFetcher(cfg, logger)
	if err != nil {
		logger.Fatal("failed to initialize weather fetcher", zap.Error(err))
	}

	// 5) Build cron (standard 5-field, minute resolution)
	c := cron.New()
	const spec = "* * * * *" // every minute, at second 0

	_, err = c.AddFunc(spec, func() {
		// Add 30s to avoid rolling edge cases (e.g. 12:05:59.999)
		now := time.Now().Add(30 * time.Second)
		minute := now.Minute()
		hour := now.Hour()

		ctx := context.Background()

		// 5a) Hourly subscribers
		hourlySubs, err := subRepo.HourlyBatch(ctx, minute)
		if err != nil {
			logger.Error("failed to fetch hourly subscriptions",
				zap.Int("minute", minute), zap.Error(err))
		} else {
			sendWeatherUpdates(ctx, hourlySubs, weatherFetcher, smtpSender, cfg.BaseURL, logger)
		}

		// 5b) Daily subscribers
		dailySubs, err := subRepo.DailyBatch(ctx, hour, minute)
		if err != nil {
			logger.Error("failed to fetch daily subscriptions",
				zap.Int("hour", hour), zap.Int("minute", minute), zap.Error(err))
		} else {
			sendWeatherUpdates(ctx, dailySubs, weatherFetcher, smtpSender, cfg.BaseURL, logger)
		}
	})
	if err != nil {
		logger.Fatal("unable to schedule cron job", zap.Error(err))
	}

	logger.Info("starting scheduler", zap.String("cronSpec", spec))
	c.Start()

	// block forever
	select {}
}

// sendWeatherUpdates fetches weather for each subscription and
// sends all emails in one batch (one SMTP session), including an unsubscribe link.
func sendWeatherUpdates(
	ctx context.Context,
	subs []repository.Subscription,
	fetcher weather.Fetcher,
	sender email.EmailSender,
	baseURL string,
	logger *zap.Logger,
) {
	if len(subs) == 0 {
		return
	}

	var messages []email.EmailMessage
	for _, sub := range subs {
		w, err := fetcher.FetchCurrent(ctx, sub.City)
		if err != nil {
			logger.Error("weather fetch failed",
				zap.String("email", sub.Email),
				zap.String("city", sub.City),
				zap.Error(err))
			continue
		}

		confirmUnsubURL := fmt.Sprintf("%s/api/unsubscribe/%s", baseURL, sub.UnsubscribeToken.String())

		body := fmt.Sprintf(
			`<p>Current weather in <b>%s</b>:</p>
<ul>
  <li>Temperature: %.2f°C</li>
  <li>Humidity: %d%%</li>
  <li>Description: %s</li>
</ul>
<p><a href="%s">Unsubscribe</a> from these updates.</p>`,
			sub.City, w.Temp, w.Humidity, w.Description,
			confirmUnsubURL,
		)

		messages = append(messages, email.EmailMessage{
			To:      []string{sub.Email},
			Subject: fmt.Sprintf("Weather update for %s", sub.City),
			Body:    body,
		})
	}

	if len(messages) == 0 {
		return
	}
	if err := sender.SendBatch(messages); err != nil {
		logger.Error("failed to send weather update emails", zap.Error(err))
	} else {
		logger.Info("sent weather update emails", zap.Int("count", len(messages)))
	}
}
