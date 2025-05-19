package repository

import (
	"context"
	"github.com/jmoiron/sqlx"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" driver
)

func OpenDB(dsn string) (*sqlx.DB, error) {
	db, err := sqlx.Open("pgx", dsn) // ← driver name is "pgx"
	if err != nil {
		return nil, err
	}
	db.SetConnMaxLifetime(time.Minute * 5)
	db.SetMaxIdleConns(5)
	db.SetMaxOpenConns(10)
	return db, db.PingContext(context.Background())
}
