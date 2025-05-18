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

	return &Config{
		PostgresUser:     pgUser,
		PostgresPassword: pgPass,
		PostgresDB:       pgDB,
		PostgresHost:     pgHost,
		PostgresPort:     pgPort,
		DatabaseURL:      databaseURL,
	}, nil
}
