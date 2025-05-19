package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/namefreezers/Software-Engineering-School-5.0-weather-api/internal/config"
	"github.com/namefreezers/Software-Engineering-School-5.0-weather-api/internal/email"
	"github.com/namefreezers/Software-Engineering-School-5.0-weather-api/internal/repository"
	"github.com/namefreezers/Software-Engineering-School-5.0-weather-api/internal/weather"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Sentinel errors for your HTTP handlers to inspect:
var (
	// error for invalid city
	ErrInvalidCity = errors.New("invalid city")

	// returned when someone tries to subscribe the same email+city twice
	ErrAlreadySubscribed = errors.New("email already subscribed for this city")

	// returned when the token string is malformed (not a UUID)
	ErrInvalidToken = errors.New("invalid token format")

	// returned when no subscription matches the given token
	ErrTokenNotFound = errors.New("subscription not found for this token")
)

// SubscriptionService defines your business operations.
type SubscriptionService interface {
	Subscribe(ctx context.Context, emailAddr, city, frequency string) error
	Confirm(ctx context.Context, token string) error
	Unsubscribe(ctx context.Context, token string) error
}

type subscriptionService struct {
	repo           repository.SubscriptionRepository
	emailSender    email.EmailSender
	weatherFetcher weather.Fetcher
	cfg            *config.Config
	logger         *zap.Logger
}

// NewSubscriptionService wires up service dependencies.
func NewSubscriptionService(
	repo repository.SubscriptionRepository,
	emailSender email.EmailSender,
	weatherFetcher weather.Fetcher,
	cfg *config.Config,
	logger *zap.Logger,
) SubscriptionService {
	return &subscriptionService{repo, emailSender, weatherFetcher, cfg, logger}
}

// validateCity actually tries to fetch once and returns ErrInvalidCity on failure
func (s *subscriptionService) validateCity(ctx context.Context, city string) error {
	if _, err := s.weatherFetcher.FetchCurrent(ctx, city); err != nil {
		return ErrInvalidCity
	}
	return nil
}

// Subscribe creates a new unconfirmed subscription and sends a confirmation email.
func (s *subscriptionService) Subscribe(ctx context.Context, emailAddr, city, frequency string) error {
	// validate the city name by doing a single FetchCurrent first
	if err := s.validateCity(ctx, city); err != nil {
		return ErrInvalidCity
	}

	confirmToken, unsubscribeToken, err := s.repo.Create(ctx, emailAddr, city, frequency)
	if err != nil {
		if errors.Is(err, repository.ErrEmailAlreadyExists) {
			return ErrAlreadySubscribed
		}
		return fmt.Errorf("repo.Create: %w", err)
	}

	// Build the confirmation link (swagger basePath is /api)
	confirmURL := fmt.Sprintf("%s/api/confirm/%s", s.cfg.BaseURL, confirmToken.String())
	unsubscribeURL := fmt.Sprintf("%s/api/unsubscribe/%s", s.cfg.BaseURL, unsubscribeToken.String())

	body := fmt.Sprintf(
		`<p>Please confirm your subscription for <b>%s</b> weather updates:</p>
         <p><a href="%s">Confirm Subscription</a></p>
         <p><a href="%s">Unsubscribe</a></p>`,
		city, confirmURL, unsubscribeURL,
	)

	msg := email.EmailMessage{
		To:      []string{emailAddr},
		Subject: "Confirm your weather subscription",
		Body:    body,
	}
	if err := s.emailSender.SendBatch([]email.EmailMessage{msg}); err != nil {
		return fmt.Errorf("email.SendBatch: %w", err)
	}

	s.logger.Info("confirmation email sent",
		zap.String("email", emailAddr),
		zap.String("confirmToken", confirmToken.String()),
		zap.String("unsubscribeToken", unsubscribeToken.String()),
	)
	return nil
}

// Confirm parses and validates the token, then marks the subscription confirmed.
func (s *subscriptionService) Confirm(ctx context.Context, tokenStr string) error {
	t, err := uuid.Parse(tokenStr)
	if err != nil {
		return ErrInvalidToken
	}

	if err := s.repo.Confirm(ctx, t); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrTokenNotFound
		}
		return fmt.Errorf("repo.Confirm: %w", err)
	}

	s.logger.Info("subscription confirmed", zap.String("token", tokenStr))
	return nil
}

// Unsubscribe parses the token and deletes the associated subscription.
func (s *subscriptionService) Unsubscribe(ctx context.Context, tokenStr string) error {
	t, err := uuid.Parse(tokenStr)
	if err != nil {
		return ErrInvalidToken
	}

	if err := s.repo.DeleteByUnsubToken(ctx, t); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrTokenNotFound
		}
		return fmt.Errorf("repo.DeleteByUnsubToken: %w", err)
	}

	s.logger.Info("subscription unsubscribed", zap.String("token", tokenStr))
	return nil
}
