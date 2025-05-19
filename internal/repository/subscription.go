package repository

import (
	"context"
	"database/sql"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
	"time"
)

type Subscription struct {
	ID               int       `db:"id"`
	Email            string    `db:"email"`
	City             string    `db:"city"`
	Frequency        string    `db:"frequency"` // 'hourly' | 'daily'
	Confirmed        bool      `db:"confirmed"`
	ConfirmToken     uuid.UUID `db:"confirm_token"`
	UnsubscribeToken uuid.UUID `db:"unsubscribe_token"`
	ScheduledMinute  int16     `db:"scheduled_minute"`
	ScheduledHour    int16     `db:"scheduled_hour"`
	CreatedAt        time.Time `db:"created_at"`
}

// SubscriptionRepository defines the five interactions you listed.
type SubscriptionRepository interface {
	Create(ctx context.Context, email, city, freq string) (confirmToken uuid.UUID, unsubscribeToken uuid.UUID, err error)
	Confirm(ctx context.Context, token uuid.UUID) error
	DeleteByUnsubToken(ctx context.Context, token uuid.UUID) error
	HourlyBatch(ctx context.Context, minute int) ([]Subscription, error)
	DailyBatch(ctx context.Context, hour, minute int) ([]Subscription, error)
}

type pgRepo struct {
	db     *sqlx.DB
	logger *zap.Logger
}

func NewSubscriptionRepository(db *sqlx.DB, logger *zap.Logger) SubscriptionRepository {
	return &pgRepo{db: db, logger: logger}
}

// ErrEmailAlreadyExists is returned when attempting to subscribe an email that already exists.
var ErrEmailAlreadyExists = errors.New("email already subscribed")

func (r *pgRepo) Create(ctx context.Context, email, city, freq string,
) (confirmToken uuid.UUID, unsubscribeToken uuid.UUID, err error) {
	const q = `
        INSERT INTO subscriptions (email, city, frequency)
        VALUES ($1, $2, $3)
        RETURNING confirm_token, unsubscribe_token;
    `

	// Scan both tokens in one go
	row := r.db.QueryRowContext(ctx, q, email, city, freq)
	if err := row.Scan(&confirmToken, &unsubscribeToken); err != nil {
		// Check for Postgres unique‐violation on the email column (SQLSTATE 23505)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			r.logger.Warn("duplicate email subscription attempt",
				zap.String("email", email),
			)
			return uuid.Nil, uuid.Nil, ErrEmailAlreadyExists
		}

		r.logger.Error("failed to create subscription",
			zap.String("email", email),
			zap.String("city", city),
			zap.String("frequency", freq),
			zap.Error(err),
		)
		return uuid.Nil, uuid.Nil, err
	}

	r.logger.Debug("subscription created",
		zap.String("email", email),
		zap.String("city", city),
		zap.String("frequency", freq),
		zap.String("confirm_token", confirmToken.String()),
		zap.String("unsubscribe_token", unsubscribeToken.String()),
	)
	return confirmToken, unsubscribeToken, nil
}

func (r *pgRepo) Confirm(ctx context.Context, token uuid.UUID) error {
	// We are advancing scheduled_hour, scheduled_minute one minute ahead to receive first email in ~30 seconds
	const q = `
        UPDATE subscriptions
        SET confirmed        = TRUE,
            confirm_token    = NULL,
            scheduled_hour   = EXTRACT(HOUR   FROM now() + INTERVAL '1 minute')::smallint,
            scheduled_minute = EXTRACT(MINUTE FROM now() + INTERVAL '1 minute')::smallint
        WHERE confirm_token = $1 AND confirmed = FALSE;
    `
	res, err := r.db.ExecContext(ctx, q, token)
	if err != nil {
		r.logger.Error("failed to confirm subscription", zap.String("token", token.String()), zap.Error(err))
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		r.logger.Error("failed to get rows affected on confirm", zap.Error(err))
		return err
	}
	if n == 0 {
		r.logger.Warn("confirm token not found or already confirmed", zap.String("token", token.String()))
		return sql.ErrNoRows
	}
	r.logger.Info("subscription confirmed", zap.String("token", token.String()))
	return nil
}

func (r *pgRepo) DeleteByUnsubToken(ctx context.Context, token uuid.UUID) error {
	const q = `DELETE FROM subscriptions WHERE unsubscribe_token = $1;`
	res, err := r.db.ExecContext(ctx, q, token)
	if err != nil {
		r.logger.Error("failed to delete subscription", zap.String("unsubscribe_token", token.String()), zap.Error(err))
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		r.logger.Error("failed to get rows affected on delete", zap.Error(err))
		return err
	}
	if n == 0 {
		r.logger.Warn("unsubscribe token not found", zap.String("unsubscribe_token", token.String()))
		return sql.ErrNoRows
	}
	r.logger.Info("subscription deleted", zap.String("unsubscribe_token", token.String()))
	return nil
}

func (r *pgRepo) HourlyBatch(ctx context.Context, minute int) ([]Subscription, error) {
	const q = `
        SELECT * FROM subscriptions
        WHERE confirmed       = TRUE
          AND frequency       = 'hourly'
          AND scheduled_minute= $1;
    `
	var subs []Subscription
	if err := r.db.SelectContext(ctx, &subs, q, minute); err != nil {
		r.logger.Error("failed to fetch hourly batch", zap.Int("minute", minute), zap.Error(err))
		return nil, err
	}
	r.logger.Debug("fetched hourly batch", zap.Int("minute", minute), zap.Int("count", len(subs)))
	return subs, nil
}

func (r *pgRepo) DailyBatch(ctx context.Context, hour, minute int) ([]Subscription, error) {
	const q = `
        SELECT * FROM subscriptions
        WHERE confirmed        = TRUE
          AND frequency        = 'daily'
          AND scheduled_hour   = $1
          AND scheduled_minute = $2;
    `
	var subs []Subscription
	if err := r.db.SelectContext(ctx, &subs, q, hour, minute); err != nil {
		r.logger.Error("failed to fetch daily batch", zap.Int("hour", hour), zap.Int("minute", minute), zap.Error(err))
		return nil, err
	}
	r.logger.Debug("fetched daily batch", zap.Int("hour", hour), zap.Int("minute", minute), zap.Int("count", len(subs)))
	return subs, nil
}
