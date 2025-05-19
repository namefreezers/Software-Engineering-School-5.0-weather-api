package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/namefreezers/Software-Engineering-School-5.0-weather-api/internal/config"
	"github.com/namefreezers/Software-Engineering-School-5.0-weather-api/internal/email"
	"github.com/namefreezers/Software-Engineering-School-5.0-weather-api/internal/handlers"
	"github.com/namefreezers/Software-Engineering-School-5.0-weather-api/internal/repository"
	"github.com/namefreezers/Software-Engineering-School-5.0-weather-api/internal/services"
	"github.com/namefreezers/Software-Engineering-School-5.0-weather-api/internal/weather"
)

func main() {
	// 1) Load configuration from environment
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}

	// 2) Initialize structured logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("cannot initialize logger: %v", err)
	}
	defer logger.Sync()

	// 3) Connect to Postgres
	db, err := repository.OpenDB(cfg.DatabaseURL)
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}

	// 4) Initialize SMTP email sender
	smtpSender, err := email.NewSMTPSender(cfg, logger)
	if err != nil {
		logger.Fatal("failed to initialize SMTP sender", zap.Error(err))
	}

	// 5) Build the weather fetcher (with caching & multiple providers)
	weatherFetcher, err := weather.BuildCachingFetcher(cfg, logger)
	if err != nil {
		logger.Fatal("failed to initialize weather fetcher", zap.Error(err))
	}

	// 6) Wire up the subscription service
	subRepo := repository.NewSubscriptionRepository(db, logger)
	subSvc := services.NewSubscriptionService(subRepo, smtpSender, weatherFetcher, cfg, logger)

	// 7) Set up Gin router and handlers
	router := gin.Default()
	api := router.Group("/api")
	{
		api.GET("/weather", handlers.WeatherHandler(weatherFetcher))
		api.POST("/subscribe", handlers.SubscribeHandler(subSvc))
		api.GET("/confirm/:token", handlers.ConfirmHandler(subSvc))
		api.GET("/unsubscribe/:token", handlers.UnsubscribeHandler(subSvc))
	}

	// 8) Start HTTP server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port
	logger.Info("starting API server", zap.String("address", addr))
	if err := router.Run(addr); err != nil {
		logger.Fatal("server error", zap.Error(err))
	}
}
