package repository

import (
	"context"
	"database/sql"
	"errors"
	"go.uber.org/zap"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

func setupMockDB(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock, func()) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	sqlxDB := sqlx.NewDb(db, "pgx")
	cleanup := func() {
		sqlxDB.Close()
	}
	return sqlxDB, mock, cleanup
}

func TestSubscriptionRepository_Create_Success(t *testing.T) {
	sqlxDB, mock, cleanup := setupMockDB(t)
	defer cleanup()

	logger := zap.NewNop()
	repo := NewSubscriptionRepository(sqlxDB, logger)

	// Prepare expected tokens
	wantConfirm := uuid.New()
	wantUnsub := uuid.New()
	rows := sqlmock.NewRows([]string{"confirm_token", "unsubscribe_token"}).
		AddRow(wantConfirm, wantUnsub)

	// Expect the INSERT ... RETURNING both tokens
	mock.ExpectQuery(regexp.QuoteMeta(
		"INSERT INTO subscriptions (email, city, frequency) VALUES ($1, $2, $3) RETURNING confirm_token, unsubscribe_token",
	)).
		WithArgs("foo@bar.com", "Paris", "daily").
		WillReturnRows(rows)

	// Call Create
	gotConfirm, gotUnsub, err := repo.Create(context.Background(), "foo@bar.com", "Paris", "daily")
	if err != nil {
		t.Fatalf("Create() unexpected error: %v", err)
	}
	if gotConfirm != wantConfirm {
		t.Errorf("Create() confirmToken = %v, want %v", gotConfirm, wantConfirm)
	}
	if gotUnsub != wantUnsub {
		t.Errorf("Create() unsubscribeToken = %v, want %v", gotUnsub, wantUnsub)
	}

	// Ensure all expectations met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

func TestSubscriptionRepository_Create_DBError(t *testing.T) {
	sqlxDB, mock, cleanup := setupMockDB(t)
	defer cleanup()

	logger := zap.NewNop()
	repo := NewSubscriptionRepository(sqlxDB, logger)

	// Simulate a DB error on the RETURNING query
	mock.ExpectQuery(regexp.QuoteMeta(
		"INSERT INTO subscriptions (email, city, frequency) VALUES ($1, $2, $3) RETURNING confirm_token, unsubscribe_token",
	)).
		WithArgs("foo@bar.com", "Paris", "daily").
		WillReturnError(sql.ErrConnDone)

	// Call Create
	gotConfirm, gotUnsub, err := repo.Create(context.Background(), "foo@bar.com", "Paris", "daily")
	if err == nil {
		t.Fatalf("Create() expected error, got nil")
	}
	if !errors.Is(err, sql.ErrConnDone) {
		t.Errorf("Create() error = %v, want %v", err, sql.ErrConnDone)
	}
	// tokens should be zero when err != nil
	if gotConfirm != uuid.Nil {
		t.Errorf("Create() confirmToken = %v, want %v", gotConfirm, uuid.Nil)
	}
	if gotUnsub != uuid.Nil {
		t.Errorf("Create() unsubscribeToken = %v, want %v", gotUnsub, uuid.Nil)
	}

	// Ensure all expectations met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

func TestSubscriptionRepository_Confirm_Success(t *testing.T) {
	sqlxDB, mock, cleanup := setupMockDB(t)
	defer cleanup()
	logger := zap.NewNop()
	repo := NewSubscriptionRepository(sqlxDB, logger)

	// Expect Exec to update 1 row
	mock.ExpectExec(regexp.QuoteMeta(`
        UPDATE subscriptions
        SET confirmed        = TRUE,
            confirm_token    = NULL,
            scheduled_hour   = EXTRACT(HOUR   FROM now() + INTERVAL '1 minute')::smallint,
            scheduled_minute = EXTRACT(MINUTE FROM now() + INTERVAL '1 minute')::smallint
        WHERE confirm_token = $1 AND confirmed = FALSE;
    `)).
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.Confirm(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("Confirm() unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestSubscriptionRepository_Confirm_NotFound(t *testing.T) {
	sqlxDB, mock, cleanup := setupMockDB(t)
	defer cleanup()
	logger := zap.NewNop()
	repo := NewSubscriptionRepository(sqlxDB, logger)

	// Expect Exec to affect 0 rows
	mock.ExpectExec(regexp.QuoteMeta(`
        UPDATE subscriptions
        SET confirmed        = TRUE,
            confirm_token    = NULL,
            scheduled_hour   = EXTRACT(HOUR   FROM now() + INTERVAL '1 minute')::smallint,
            scheduled_minute = EXTRACT(MINUTE FROM now() + INTERVAL '1 minute')::smallint
        WHERE confirm_token = $1 AND confirmed = FALSE;
    `)).
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.Confirm(context.Background(), uuid.New())
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("Confirm() error = %v, want sql.ErrNoRows", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestSubscriptionRepository_Confirm_DBError(t *testing.T) {
	sqlxDB, mock, cleanup := setupMockDB(t)
	defer cleanup()
	logger := zap.NewNop()
	repo := NewSubscriptionRepository(sqlxDB, logger)

	// Simulate a database error
	mock.ExpectExec(regexp.QuoteMeta(`
        UPDATE subscriptions
        SET confirmed        = TRUE,
            confirm_token    = NULL,
            scheduled_hour   = EXTRACT(HOUR   FROM now() + INTERVAL '1 minute')::smallint,
            scheduled_minute = EXTRACT(MINUTE FROM now() + INTERVAL '1 minute')::smallint
        WHERE confirm_token = $1 AND confirmed = FALSE;
    `)).
		WithArgs(sqlmock.AnyArg()).
		WillReturnError(sql.ErrConnDone)

	err := repo.Confirm(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("Confirm() expected an error, got nil")
	}
	if !errors.Is(err, sql.ErrConnDone) {
		t.Fatalf("Confirm() error = %v, want %v", err, sql.ErrConnDone)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestSubscriptionRepository_DeleteByUnsubToken_Success(t *testing.T) {
	sqlxDB, mock, cleanup := setupMockDB(t)
	defer cleanup()
	logger := zap.NewNop()
	repo := NewSubscriptionRepository(sqlxDB, logger)

	// Expect the DELETE to affect 1 row
	mock.ExpectExec(regexp.QuoteMeta(
		"DELETE FROM subscriptions WHERE unsubscribe_token = $1",
	)).
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.DeleteByUnsubToken(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("DeleteByUnsubToken() unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

func TestSubscriptionRepository_DeleteByUnsubToken_NotFound(t *testing.T) {
	sqlxDB, mock, cleanup := setupMockDB(t)
	defer cleanup()
	logger := zap.NewNop()
	repo := NewSubscriptionRepository(sqlxDB, logger)

	// Expect the DELETE to affect 0 rows
	mock.ExpectExec(regexp.QuoteMeta(
		"DELETE FROM subscriptions WHERE unsubscribe_token = $1",
	)).
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.DeleteByUnsubToken(context.Background(), uuid.New())
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("DeleteByUnsubToken() error = %v, want sql.ErrNoRows", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

func TestSubscriptionRepository_DeleteByUnsubToken_DBError(t *testing.T) {
	sqlxDB, mock, cleanup := setupMockDB(t)
	defer cleanup()
	logger := zap.NewNop()
	repo := NewSubscriptionRepository(sqlxDB, logger)

	// Simulate a DB error on Exec
	mock.ExpectExec(regexp.QuoteMeta(
		"DELETE FROM subscriptions WHERE unsubscribe_token = $1",
	)).
		WithArgs(sqlmock.AnyArg()).
		WillReturnError(sql.ErrConnDone)

	err := repo.DeleteByUnsubToken(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("DeleteByUnsubToken() expected an error, got nil")
	}
	if !errors.Is(err, sql.ErrConnDone) {
		t.Fatalf("DeleteByUnsubToken() error = %v, want %v", err, sql.ErrConnDone)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}
func TestSubscriptionRepository_HourlyBatch_ReturnsRows(t *testing.T) {
	sqlxDB, mock, cleanup := setupMockDB(t)
	defer cleanup()
	logger := zap.NewNop()
	repo := NewSubscriptionRepository(sqlxDB, logger)

	// Prepare a fake subscription row
	id := 1
	email := "test@example.com"
	city := "TestCity"
	frequency := "hourly"
	confirmed := true
	confirmToken := uuid.New()
	unsubToken := uuid.New()
	scheduledMinute := 15
	scheduledHour := 0
	createdAt := time.Now().UTC().Truncate(time.Second)

	rows := sqlmock.NewRows([]string{
		"id", "email", "city", "frequency", "confirmed",
		"confirm_token", "unsubscribe_token",
		"scheduled_minute", "scheduled_hour", "created_at",
	}).AddRow(
		id, email, city, frequency, confirmed,
		confirmToken, unsubToken,
		scheduledMinute, scheduledHour, createdAt,
	)

	// Expect the SELECT ... WHERE ... hourly query
	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT * FROM subscriptions WHERE confirmed       = TRUE AND frequency       = 'hourly' AND scheduled_minute= $1",
	)).
		WithArgs(scheduledMinute).
		WillReturnRows(rows)

	// Call HourlyBatch
	subs, err := repo.HourlyBatch(context.Background(), scheduledMinute)
	if err != nil {
		t.Fatalf("HourlyBatch() unexpected error: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("HourlyBatch() returned %d rows, want 1", len(subs))
	}
	s := subs[0]
	if s.ID != id || s.Email != email || s.City != city ||
		s.Frequency != frequency || !s.Confirmed ||
		s.ConfirmToken != confirmToken || s.UnsubscribeToken != unsubToken ||
		int(s.ScheduledMinute) != scheduledMinute {
		t.Errorf("HourlyBatch() returned row %+v, want matching test data", s)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

func TestSubscriptionRepository_HourlyBatch_Empty(t *testing.T) {
	sqlxDB, mock, cleanup := setupMockDB(t)
	defer cleanup()
	logger := zap.NewNop()
	repo := NewSubscriptionRepository(sqlxDB, logger)

	// Expect an empty result set
	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT * FROM subscriptions WHERE confirmed       = TRUE AND frequency       = 'hourly' AND scheduled_minute= $1",
	)).
		WithArgs(42).
		WillReturnRows(sqlmock.NewRows(nil))

	subs, err := repo.HourlyBatch(context.Background(), 42)
	if err != nil {
		t.Fatalf("HourlyBatch() unexpected error: %v", err)
	}
	if len(subs) != 0 {
		t.Fatalf("HourlyBatch() returned %d rows, want 0", len(subs))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

func TestSubscriptionRepository_HourlyBatch_DBError(t *testing.T) {
	sqlxDB, mock, cleanup := setupMockDB(t)
	defer cleanup()
	logger := zap.NewNop()
	repo := NewSubscriptionRepository(sqlxDB, logger)

	// Simulate a DB error on query
	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT * FROM subscriptions WHERE confirmed       = TRUE AND frequency       = 'hourly' AND scheduled_minute= $1",
	)).
		WithArgs(30).
		WillReturnError(sql.ErrConnDone)

	_, err := repo.HourlyBatch(context.Background(), 30)
	if err == nil {
		t.Fatal("HourlyBatch() expected error, got nil")
	}
	if !errors.Is(err, sql.ErrConnDone) {
		t.Fatalf("HourlyBatch() error = %v, want %v", err, sql.ErrConnDone)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

func TestSubscriptionRepository_DailyBatch_ReturnsRows(t *testing.T) {
	sqlxDB, mock, cleanup := setupMockDB(t)
	defer cleanup()
	logger := zap.NewNop()
	repo := NewSubscriptionRepository(sqlxDB, logger)

	// Prepare a fake subscription row
	id := 1
	email := "daily@example.com"
	city := "TestDaily"
	frequency := "daily"
	confirmed := true
	confirmToken := uuid.New()
	unsubToken := uuid.New()
	scheduledMinute := 30
	scheduledHour := 9
	createdAt := time.Now().UTC().Truncate(time.Second)

	rows := sqlmock.NewRows([]string{
		"id", "email", "city", "frequency", "confirmed",
		"confirm_token", "unsubscribe_token",
		"scheduled_minute", "scheduled_hour", "created_at",
	}).AddRow(
		id, email, city, frequency, confirmed,
		confirmToken, unsubToken,
		scheduledMinute, scheduledHour, createdAt,
	)

	// Expect the SELECT ... WHERE ... daily query
	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT * FROM subscriptions WHERE confirmed        = TRUE AND frequency        = 'daily' AND scheduled_hour   = $1 AND scheduled_minute = $2",
	)).
		WithArgs(scheduledHour, scheduledMinute).
		WillReturnRows(rows)

	// Call DailyBatch
	subs, err := repo.DailyBatch(context.Background(), scheduledHour, scheduledMinute)
	if err != nil {
		t.Fatalf("DailyBatch() unexpected error: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("DailyBatch() returned %d rows, want 1", len(subs))
	}
	s := subs[0]
	if s.ID != id || s.Email != email || s.City != city ||
		s.Frequency != frequency || !s.Confirmed ||
		s.ConfirmToken != confirmToken || s.UnsubscribeToken != unsubToken ||
		int(s.ScheduledHour) != scheduledHour || int(s.ScheduledMinute) != scheduledMinute {
		t.Errorf("DailyBatch() returned row %+v, want matching test data", s)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

func TestSubscriptionRepository_DailyBatch_Empty(t *testing.T) {
	sqlxDB, mock, cleanup := setupMockDB(t)
	defer cleanup()
	logger := zap.NewNop()
	repo := NewSubscriptionRepository(sqlxDB, logger)

	// Expect an empty result set
	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT * FROM subscriptions WHERE confirmed        = TRUE AND frequency        = 'daily' AND scheduled_hour   = $1 AND scheduled_minute = $2",
	)).
		WithArgs(23, 59).
		WillReturnRows(sqlmock.NewRows(nil))

	subs, err := repo.DailyBatch(context.Background(), 23, 59)
	if err != nil {
		t.Fatalf("DailyBatch() unexpected error: %v", err)
	}
	if len(subs) != 0 {
		t.Fatalf("DailyBatch() returned %d rows, want 0", len(subs))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}

func TestSubscriptionRepository_DailyBatch_DBError(t *testing.T) {
	sqlxDB, mock, cleanup := setupMockDB(t)
	defer cleanup()
	logger := zap.NewNop()
	repo := NewSubscriptionRepository(sqlxDB, logger)

	// Simulate a DB error on query
	mock.ExpectQuery(regexp.QuoteMeta(
		"SELECT * FROM subscriptions WHERE confirmed        = TRUE AND frequency        = 'daily' AND scheduled_hour   = $1 AND scheduled_minute = $2",
	)).
		WithArgs(12, 0).
		WillReturnError(sql.ErrConnDone)

	_, err := repo.DailyBatch(context.Background(), 12, 0)
	if err == nil {
		t.Fatal("DailyBatch() expected error, got nil")
	}
	if !errors.Is(err, sql.ErrConnDone) {
		t.Fatalf("DailyBatch() error = %v, want %v", err, sql.ErrConnDone)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet sqlmock expectations: %v", err)
	}
}
