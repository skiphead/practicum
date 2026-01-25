package audit

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/skiphead/practicum/pkg/transport/httpclient"
)

// Client Интерфейс клиента для упрощения тестирования
type Client interface {
	CreateAuditEvent(ctx context.Context, req *CreateAuditRequest) error
	CreateAuditEventWithRetry(ctx context.Context, req *CreateAuditRequest, retryOpts ...httpclient.RetryOption) error
	BatchCreateAuditEvents(ctx context.Context, events []*CreateAuditRequest) error
}

// Service представляет сервис аудита
type Service struct {
	httpClient httpclient.Client // Используем интерфейс вместо конкретного типа
	config     ServiceConfig
}

// ServiceConfig Конфигурация сервиса
type ServiceConfig struct {
	MaxBatchSize int
}

// DefaultServiceConfig возвращает конфигурацию по умолчанию
func DefaultServiceConfig() ServiceConfig {
	return ServiceConfig{
		MaxBatchSize: 1000,
	}
}

// CreateAuditRequest структура запроса для создания аудит-события
type CreateAuditRequest struct {
	TS     int    `json:"ts"`
	Action string `json:"action"`
	UserID string `json:"user_id"`
	URL    string `json:"url"`
}

// NewService создает новый сервис аудита
func NewService(httpClient *httpclient.HTTPClient, config ServiceConfig) *Service {
	return &Service{
		httpClient: httpClient,
		config:     config,
	}
}

// CreateAuditEvent создает аудит-событие
func (s *Service) CreateAuditEvent(ctx context.Context, req *CreateAuditRequest) error {
	return s.CreateAuditEventWithRetry(ctx, req)
}

// CreateAuditEventWithRetry создает аудит-событие с кастомными параметрами повторных попыток
func (s *Service) CreateAuditEventWithRetry(ctx context.Context, req *CreateAuditRequest, retryOpts ...httpclient.RetryOption) error {
	// Валидируем запрос
	if err := s.validateAuditRequest(req); err != nil {
		return fmt.Errorf("invalid audit request: %w", err)
	}

	// Применяем параметры повторных попыток
	opts := httpclient.DefaultRetryOptions
	for _, opt := range retryOpts {
		opt(&opts)
	}

	// Выполняем запрос с повторными попытками
	return s.executeWithRetry(ctx, func() error {
		return s.httpClient.SendRequest(ctx, "POST", "/api/audit/events", req)
	}, opts)
}

// BatchCreateAuditEvents создает несколько аудит-событий одной пачкой
func (s *Service) BatchCreateAuditEvents(ctx context.Context, events []*CreateAuditRequest) error {
	if len(events) == 0 {
		return nil
	}

	// Проверяем размер пачки
	if len(events) > s.config.MaxBatchSize {
		return fmt.Errorf("batch size %d exceeds maximum allowed size %d", len(events), s.config.MaxBatchSize)
	}

	// Валидируем все события
	for i, event := range events {
		if err := s.validateAuditRequest(event); err != nil {
			return fmt.Errorf("invalid audit request at index %d: %w", i, err)
		}
	}

	// Выполняем запрос с повторными попытками
	return s.executeWithRetry(ctx, func() error {
		return s.httpClient.SendRequest(ctx, "POST", "/api/audit/events/batch", events)
	}, httpclient.DefaultRetryOptions)
}

// validateAuditRequest валидирует запрос на создание аудита
func (s *Service) validateAuditRequest(req *CreateAuditRequest) error {
	if req == nil {
		return fmt.Errorf("request cannot be nil")
	}

	if req.TS <= 0 {
		return fmt.Errorf("timestamp must be positive")
	}

	if req.Action == "" {
		return fmt.Errorf("action cannot be empty")
	}

	if len(req.Action) > 100 {
		return fmt.Errorf("action too long, max 100 characters")
	}

	if req.UserID == "" {
		return fmt.Errorf("user_id cannot be empty")
	}

	if len(req.UserID) > 100 {
		return fmt.Errorf("user_id too long, max 100 characters")
	}

	if req.URL == "" {
		return fmt.Errorf("url cannot be empty")
	}

	if len(req.URL) > 2000 {
		return fmt.Errorf("url too long, max 2000 characters")
	}

	// Проверяем, что URL валидный
	if _, err := url.Parse(req.URL); err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	return nil
}

// executeWithRetry выполняет операцию с поддержкой повторных попыток
func (s *Service) executeWithRetry(ctx context.Context, operation func() error, opts httpclient.RetryOptions) error {
	var lastErr error

	// Создаем контекст с общим таймаутом для всех попыток
	if opts.MaxWaitTime > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.MaxWaitTime)
		defer cancel()
	}

	attempt := 0
	for {
		// Выполняем операцию
		err := operation()

		if err == nil {
			break
		}

		// Сохраняем последнюю ошибку
		lastErr = err

		// Проверяем, нужно ли делать повторную попытку
		shouldRetry, delay := s.httpClient.ShouldRetry(err, attempt, opts)
		if !shouldRetry {
			return err
		}

		// Увеличиваем счетчик попыток
		attempt++

		// Ждем перед следующей попыткой
		if delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				// Возвращаем ошибку контекста с информацией о последней ошибке операции
				if lastErr != nil {
					return fmt.Errorf("operation failed: %w, context cancelled: %v", lastErr, ctx.Err())
				}
				return ctx.Err()
			case <-timer.C:
				// Продолжаем цикл
			}
		}
	}

	return nil
}
