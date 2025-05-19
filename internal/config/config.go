package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all the environment‐driven settings for the application.
type Config struct {
	// Database (Postgres)
	PostgresUser     string
	PostgresPassword string
	PostgresDB       string
	PostgresHost     string
	PostgresPort     int
	DatabaseURL      string

	// SMTP
	SMTPHost string
	SMTPPort int
	SMTPUser string
	SMTPPass string
	SMTPFrom string

	// Weather API keys
	WeatherAPIComKey     string
	OpenWeatherMapOrgKey string

	// Redis
	RedisPassword string
	RedisAddr     string

	// API
	BaseURL string
}

// Load reads and validates all required environment variables, applying defaults
// where appropriate. It returns an error if any required variable is missing or malformed.
func Load() (*Config, error) {
	var err error

	// Postgres settings
	pgUser := os.Getenv("POSTGRES_USER")
	if pgUser == "" {
		return nil, fmt.Errorf("POSTGRES_USER is required")
	}
	pgPass := os.Getenv("POSTGRES_PASSWORD")
	if pgPass == "" {
		return nil, fmt.Errorf("POSTGRES_PASSWORD is required")
	}
	pgDB := os.Getenv("POSTGRES_DB")
	if pgDB == "" {
		return nil, fmt.Errorf("POSTGRES_DB is required")
	}
	pgHost := os.Getenv("POSTGRES_HOST")
	if pgHost == "" {
		pgHost = "db"
	}
	pgPortStr := os.Getenv("POSTGRES_PORT")
	if pgPortStr == "" {
		pgPortStr = "5432"
	}
	pgPort, err := strconv.Atoi(pgPortStr)
	if err != nil {
		return nil, fmt.Errorf("invalid POSTGRES_PORT %q: %w", pgPortStr, err)
	}
	databaseURL := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		pgUser, pgPass, pgHost, pgPort, pgDB,
	)

	// SMTP settings
	smtpHost := os.Getenv("SMTP_HOST")
	if smtpHost == "" {
		return nil, fmt.Errorf("SMTP_HOST is required")
	}
	smtpPortStr := os.Getenv("SMTP_PORT")
	if smtpPortStr == "" {
		return nil, fmt.Errorf("SMTP_PORT is required")
	}
	smtpPort, err := strconv.Atoi(smtpPortStr)
	if err != nil {
		return nil, fmt.Errorf("invalid SMTP_PORT %q: %w", smtpPortStr, err)
	}
	smtpUser := os.Getenv("SMTP_USER")
	if smtpUser == "" {
		return nil, fmt.Errorf("SMTP_USER is required")
	}
	smtpPass := os.Getenv("SMTP_PASS")
	if smtpPass == "" {
		return nil, fmt.Errorf("SMTP_PASS is required")
	}
	smtpFrom := os.Getenv("SMTP_FROM")
	if smtpFrom == "" {
		// default to the authenticated user
		smtpFrom = smtpUser
	}

	// Weather API keys. Might be present only one of them.
	weatherApiComKey := os.Getenv("WEATHERAPI_COM_API_KEY")
	openWeatherMapOrgKey := os.Getenv("OPENWEATHERMAP_ORG_API_KEY")

	// Redis settings
	redisPass := os.Getenv("REDIS_PASSWORD")
	if redisPass == "" {
		return nil, fmt.Errorf("REDIS_PASSWORD is required")
	}
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "redis:6379"
	}

	// Base URL for constructing confirmation/unsubscribe links
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		return nil, fmt.Errorf("BASE_URL is required")
	}

	return &Config{
		PostgresUser:     pgUser,
		PostgresPassword: pgPass,
		PostgresDB:       pgDB,
		PostgresHost:     pgHost,
		PostgresPort:     pgPort,
		DatabaseURL:      databaseURL,

		SMTPHost: smtpHost,
		SMTPPort: smtpPort,
		SMTPUser: smtpUser,
		SMTPPass: smtpPass,
		SMTPFrom: smtpFrom,

		WeatherAPIComKey:     weatherApiComKey,
		OpenWeatherMapOrgKey: openWeatherMapOrgKey,

		RedisPassword: redisPass,
		RedisAddr:     redisAddr,

		BaseURL: baseURL,
	}, nil
}
