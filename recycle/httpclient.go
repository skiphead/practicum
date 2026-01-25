package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Client представляет клиент API аудита
type Client struct {
	config     Config
	httpClient *http.Client
}

// Option функция для настройки клиента
type Option func(*Client) error

// RetryOption параметры для повторных попыток
type RetryOption func(*retryOptions)

type retryOptions struct {
	maxRetries  int
	retryDelay  time.Duration
	maxWaitTime time.Duration
	retryOn     []int // статус коды для повторных попыток
}

// DefaultRetryOptions дефолтные настройки для повторных попыток
var DefaultRetryOptions = retryOptions{
	maxRetries:  3,
	retryDelay:  1 * time.Second,
	maxWaitTime: 30 * time.Second,
	retryOn:     []int{429, 500, 502, 503, 504},
}

// WithMaxRetries устанавливает максимальное количество повторных попыток
func WithMaxRetries(maxRetries int) RetryOption {
	return func(opts *retryOptions) {
		if maxRetries < 0 {
			maxRetries = 0
		}
		opts.maxRetries = maxRetries
	}
}

// WithRetryDelay устанавливает задержку между попытками
func WithRetryDelay(delay time.Duration) RetryOption {
	return func(opts *retryOptions) {
		if delay < 0 {
			delay = 0
		}
		opts.retryDelay = delay
	}
}

// WithMaxWaitTime устанавливает максимальное время ожидания всех попыток
func WithMaxWaitTime(maxWaitTime time.Duration) RetryOption {
	return func(opts *retryOptions) {
		if maxWaitTime <= 0 {
			maxWaitTime = 0
		}
		opts.maxWaitTime = maxWaitTime
	}
}

// WithRetryOn устанавливает статус коды для повторных попыток
func WithRetryOn(statusCodes ...int) RetryOption {
	return func(opts *retryOptions) {
		opts.retryOn = statusCodes
	}
}

// Validate проверяет конфигурацию на корректность
func (c Config) Validate() error {
	if c.BaseURL == "" {
		return fmt.Errorf("BaseURL cannot be empty")
	}

	if _, err := url.Parse(c.BaseURL); err != nil {
		return fmt.Errorf("invalid BaseURL: %w", err)
	}

	if c.Timeout <= 0 {
		return fmt.Errorf("Timeout must be positive")
	}

	if c.MaxRetries < 0 {
		return fmt.Errorf("MaxRetries cannot be negative")
	}

	if c.RetryDelay < 0 {
		return fmt.Errorf("RetryDelay cannot be negative")
	}

	if c.UserAgent == "" {
		return fmt.Errorf("UserAgent cannot be empty")
	}

	if c.MaxResponseSize <= 0 {
		return fmt.Errorf("MaxResponseSize must be positive")
	}

	if c.MaxResponseSize > 100*1024*1024 { // 100 MB
		return fmt.Errorf("MaxResponseSize is too large")
	}

	return nil
}

// New создает новый экземпляр клиента
func New(config Config, opts ...Option) (*Client, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	client := &Client{
		config: config,
		httpClient: &http.Client{
			Timeout: time.Duration(config.Timeout) * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				IdleConnTimeout:     30 * time.Second,
				TLSHandshakeTimeout: 10 * time.Second,
			},
		},
	}

	for _, opt := range opts {
		if err := opt(client); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	return client, nil
}

// NewWithDefaults создает клиент с настройками по умолчанию
func NewWithDefaults(opts ...Option) (*Client, error) {
	return New(DefaultConfig(), opts...)
}

// WithHTTPClient устанавливает кастомный HTTP клиент
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) error {
		if httpClient == nil {
			return fmt.Errorf("httpClient cannot be nil")
		}
		c.httpClient = httpClient
		return nil
	}
}

// WithBaseURL устанавливает базовый URL
func WithBaseURL(baseURL string) Option {
	return func(c *Client) error {
		if baseURL == "" {
			return fmt.Errorf("baseURL cannot be empty")
		}
		if _, err := url.Parse(baseURL); err != nil {
			return fmt.Errorf("invalid baseURL: %w", err)
		}
		c.config.BaseURL = baseURL
		return nil
	}
}

// WithTimeout устанавливает таймаут
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) error {
		if timeout <= 0 {
			return fmt.Errorf("timeout must be positive")
		}
		c.httpClient.Timeout = timeout
		return nil
	}
}

// WithUserAgent устанавливает User-Agent
func WithUserAgent(userAgent string) Option {
	return func(c *Client) error {
		if userAgent == "" {
			return fmt.Errorf("userAgent cannot be empty")
		}
		c.config.UserAgent = userAgent
		return nil
	}
}

// WithMaxResponseSize устанавливает максимальный размер ответа
func WithMaxResponseSize(maxSize int) Option {
	return func(c *Client) error {
		if maxSize <= 0 {
			return fmt.Errorf("maxResponseSize must be positive")
		}
		if maxSize > 100*1024*1024 { // 100 MB
			return fmt.Errorf("maxResponseSize is too large")
		}
		c.config.MaxResponseSize = maxSize
		return nil
	}
}

// CreateAuditEvent создает аудит-событие
func (c *Client) CreateAuditEvent(ctx context.Context, req *CreateAuditRequest) error {
	return c.createAuditEventWithRetry(ctx, req, DefaultRetryOptions)
}

// CreateAuditEventWithRetry создает аудит-событие с кастомными параметрами повторных попыток
func (c *Client) CreateAuditEventWithRetry(ctx context.Context, req *CreateAuditRequest, retryOpts ...RetryOption) error {
	opts := DefaultRetryOptions
	for _, opt := range retryOpts {
		opt(&opts)
	}

	return c.createAuditEventWithRetry(ctx, req, opts)
}

// BatchCreateAuditEvents создает несколько аудит-событий одной пачкой
func (c *Client) BatchCreateAuditEvents(ctx context.Context, events []*CreateAuditRequest) error {
	if len(events) == 0 {
		return nil
	}

	// Проверяем размер пачки
	if len(events) > c.config.MaxBatchSize {
		return fmt.Errorf("batch size %d exceeds maximum allowed size %d", len(events), c.config.MaxBatchSize)
	}

	return c.createAuditBatchWithRetry(ctx, events, DefaultRetryOptions)
}

// createAuditEventWithRetry внутренний метод с поддержкой повторных попыток
func (c *Client) createAuditEventWithRetry(ctx context.Context, req *CreateAuditRequest, opts retryOptions) error {
	var lastErr error

	// Валидируем запрос
	if err := c.validateAuditRequest(req); err != nil {
		return fmt.Errorf("invalid audit request: %w", err)
	}

	// Создаем контекст с общим таймаутом для всех попыток
	if opts.maxWaitTime > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.maxWaitTime)
		defer cancel()
	}

	attempt := 0
	for {
		// Проверяем, не отменен ли контекст перед каждой попыткой
		if err := ctx.Err(); err != nil {
			if lastErr != nil {
				return lastErr
			}
			return err
		}

		// Выполняем запрос
		err := c.executeCreateRequest(ctx, req)

		// Если запрос успешен, возвращаем результат
		if err == nil {
			return nil
		}

		// Сохраняем последнюю ошибку
		lastErr = err

		// Проверяем, нужно ли делать повторную попытку
		shouldRetry, delay := c.shouldRetry(err, attempt, opts)
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
				if lastErr != nil {
					return lastErr
				}
				return ctx.Err()
			case <-timer.C:
				// Продолжаем цикл
			}
		}
	}
}

// createAuditBatchWithRetry внутренний метод для пакетной отправки с поддержкой повторных попыток
func (c *Client) createAuditBatchWithRetry(ctx context.Context, events []*CreateAuditRequest, opts retryOptions) error {
	var lastErr error

	// Валидируем все события
	for i, event := range events {
		if err := c.validateAuditRequest(event); err != nil {
			return fmt.Errorf("invalid audit request at index %d: %w", i, err)
		}
	}

	// Создаем контекст с общим таймаутом для всех попыток
	if opts.maxWaitTime > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.maxWaitTime)
		defer cancel()
	}

	attempt := 0
	for {
		// Проверяем, не отменен ли контекст перед каждой попыткой
		if err := ctx.Err(); err != nil {
			if lastErr != nil {
				return lastErr
			}
			return err
		}

		// Выполняем запрос
		err := c.executeBatchRequest(ctx, events)

		// Если запрос успешен, возвращаем результат
		if err == nil {
			return nil
		}

		// Сохраняем последнюю ошибку
		lastErr = err

		// Проверяем, нужно ли делать повторную попытку
		shouldRetry, delay := c.shouldRetry(err, attempt, opts)
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
				if lastErr != nil {
					return lastErr
				}
				return ctx.Err()
			case <-timer.C:
				// Продолжаем цикл
			}
		}
	}
}

// validateAuditRequest валидирует запрос на создание аудита
func (c *Client) validateAuditRequest(req *CreateAuditRequest) error {
	if req == nil {
		return fmt.Errorf("request cannot be nil")
	}

	if req.Ts <= 0 {
		return fmt.Errorf("timestamp must be positive")
	}

	if req.Action == "" {
		return fmt.Errorf("action cannot be empty")
	}

	if len(req.Action) > 100 {
		return fmt.Errorf("action too long, max 100 characters")
	}

	if req.UserId == "" {
		return fmt.Errorf("user_id cannot be empty")
	}

	if len(req.UserId) > 100 {
		return fmt.Errorf("user_id too long, max 100 characters")
	}

	if req.Url == "" {
		return fmt.Errorf("url cannot be empty")
	}

	if len(req.Url) > 2000 {
		return fmt.Errorf("url too long, max 2000 characters")
	}

	// Проверяем, что URL валидный
	if _, err := url.Parse(req.Url); err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	return nil
}

// executeCreateRequest выполняет одиночный HTTP запрос для создания аудита
func (c *Client) executeCreateRequest(ctx context.Context, req *CreateAuditRequest) error {
	// Сериализуем запрос в JSON
	body, err := json.Marshal(req)
	if err != nil {
		return &APIError{
			Type:        ErrorTypeClient,
			Message:     "failed to marshal request body",
			OriginalErr: err,
		}
	}

	return c.executePostRequest(ctx, "/api/audit/events", body)
}

// executeBatchRequest выполняет HTTP запрос для пакетного создания аудитов
func (c *Client) executeBatchRequest(ctx context.Context, events []*CreateAuditRequest) error {
	// Сериализуем запрос в JSON
	body, err := json.Marshal(events)
	if err != nil {
		return &APIError{
			Type:        ErrorTypeClient,
			Message:     "failed to marshal batch request body",
			OriginalErr: err,
		}
	}

	return c.executePostRequest(ctx, "/api/audit/events/batch", body)
}

// executePostRequest выполняет POST запрос с ограничением размера ответа
func (c *Client) executePostRequest(ctx context.Context, path string, body []byte) error {
	// Формируем URL запроса
	urlStr := fmt.Sprintf("%s%s", c.config.BaseURL, path)

	// Создаем HTTP запрос
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, urlStr, bytes.NewReader(body))
	if err != nil {
		return &APIError{
			Type:        ErrorTypeNetwork,
			Message:     "failed to create request",
			OriginalErr: err,
		}
	}

	// Устанавливаем заголовки
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.config.UserAgent)

	// Выполняем запрос
	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Проверяем, была ли ошибка из-за таймаута контекста
		if ctxErr := ctx.Err(); ctxErr != nil {
			return &APIError{
				Type:        ErrorTypeTimeout,
				Message:     "request timed out",
				OriginalErr: ctxErr,
			}
		}

		return &APIError{
			Type:        ErrorTypeNetwork,
			Message:     "request failed",
			OriginalErr: err,
		}
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Читаем тело ответа с ограничением размера
	limitedReader := io.LimitReader(resp.Body, int64(c.config.MaxResponseSize))
	responseBody, errReadAll := io.ReadAll(limitedReader)
	if errReadAll != nil {
		return &APIError{
			Type:        ErrorTypeNetwork,
			StatusCode:  resp.StatusCode,
			Message:     "failed to read response body",
			OriginalErr: errReadAll,
		}
	}

	// Проверяем, не был ли ответ обрезан
	if len(responseBody) == c.config.MaxResponseSize {
		// Пытаемся прочитать еще один байт, чтобы убедиться, что ответ был обрезан
		var extraByte [1]byte
		n, _ := resp.Body.Read(extraByte[:])
		if n > 0 {
			return &APIError{
				Type:       ErrorTypeResponseTooLarge,
				StatusCode: resp.StatusCode,
				Message:    fmt.Sprintf("response exceeds maximum size of %d bytes", c.config.MaxResponseSize),
			}
		}
	}

	// Обрабатываем статус код
	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusAccepted:
		return nil

	case http.StatusBadRequest:
		return &APIError{
			Type:       ErrorTypeClient,
			StatusCode: http.StatusBadRequest,
			Message:    fmt.Sprintf("bad request: %s", string(responseBody)),
		}

	case http.StatusUnauthorized:
		return &APIError{
			Type:       ErrorTypeClient,
			StatusCode: http.StatusUnauthorized,
			Message:    "unauthorized",
		}

	case http.StatusForbidden:
		return &APIError{
			Type:       ErrorTypeClient,
			StatusCode: http.StatusForbidden,
			Message:    "forbidden",
		}

	case http.StatusTooManyRequests:
		retryAfter := resp.Header.Get("Retry-After")
		return &APIError{
			Type:       ErrorTypeRateLimit,
			StatusCode: http.StatusTooManyRequests,
			Message:    "rate limit exceeded",
			RetryAfter: retryAfter,
		}

	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable:
		return &APIError{
			Type:       ErrorTypeServer,
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("server error: %s", string(responseBody)),
		}

	default:
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return &APIError{
				Type:       ErrorTypeClient,
				StatusCode: resp.StatusCode,
				Message:    fmt.Sprintf("client error: %d", resp.StatusCode),
			}
		} else if resp.StatusCode >= 500 {
			return &APIError{
				Type:       ErrorTypeServer,
				StatusCode: resp.StatusCode,
				Message:    fmt.Sprintf("server error: %d", resp.StatusCode),
			}
		}
		return &APIError{
			Type:       ErrorTypeUnknown,
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("unexpected status code: %d", resp.StatusCode),
		}
	}
}

// shouldRetry определяет, нужно ли делать повторную попытку
func (c *Client) shouldRetry(err error, attempt int, opts retryOptions) (bool, time.Duration) {
	// Если достигнуто максимальное количество попыток, не повторяем
	if attempt >= opts.maxRetries {
		return false, 0
	}

	var apiErr *APIError
	ok := errors.As(err, &apiErr)
	if !ok {
		// Если это не APIError, не повторяем
		return false, 0
	}

	// Не повторяем для ошибок таймаута
	if apiErr.Type == ErrorTypeTimeout {
		return false, 0
	}

	// Не повторяем для клиентских ошибок (кроме 429)
	if apiErr.Type == ErrorTypeClient && apiErr.StatusCode != 429 {
		return false, 0
	}

	// Проверяем, нужно ли повторять для данного статус кода
	shouldRetryStatusCode := false
	for _, statusCode := range opts.retryOn {
		if apiErr.StatusCode == statusCode {
			shouldRetryStatusCode = true
			break
		}
	}

	if !shouldRetryStatusCode {
		return false, 0
	}

	// Вычисляем базовую задержку
	delay := opts.retryDelay

	// Для rate limit используем заголовок Retry-After
	if apiErr.Type == ErrorTypeRateLimit && apiErr.RetryAfter != "" {
		if retryDelay, parseErr := parseRetryAfter(apiErr.RetryAfter); parseErr == nil {
			delay = retryDelay
		}
	}

	// Экспоненциальная задержка
	delay = delay * (1 << uint(attempt)) // Умножаем на 2^attempt

	// Добавляем случайный джиттер (±20%)
	// rand.Float64() безопасен для конкурентного доступа в Go 1.20+
	jitterPercent := 0.4 // ±20%
	jitterMultiplier := 1 + (rand.Float64()*jitterPercent - jitterPercent/2)
	delay = time.Duration(float64(delay) * jitterMultiplier)

	// Ограничиваем задержку снизу (минимум 10 мс)
	if delay < 10*time.Millisecond {
		delay = 10 * time.Millisecond
	}

	// Ограничиваем задержку сверху
	if delay > 30*time.Second {
		delay = 30 * time.Second
	}

	return true, delay
}

// parseRetryAfter парсит заголовок Retry-After
func parseRetryAfter(retryAfter string) (time.Duration, error) {
	if retryAfter == "" {
		return 0, fmt.Errorf("empty retry-after")
	}

	// Пробуем парсить как секунды (целое число)
	if seconds, err := strconv.Atoi(retryAfter); err == nil {
		if seconds < 0 {
			return 0, fmt.Errorf("retry-after seconds cannot be negative")
		}
		if seconds > 86400 { // 24 часа
			return 0, fmt.Errorf("retry-after seconds too large: %d", seconds)
		}
		return time.Duration(seconds) * time.Second, nil
	}

	// Пробуем парсить как RFC1123 дату
	if date, err := time.Parse(time.RFC1123, retryAfter); err == nil {
		return calculateRetryDuration(date)
	}

	// Пробуем парсить как RFC1123Z (с часовым поясом)
	if date, err := time.Parse(time.RFC1123Z, retryAfter); err == nil {
		return calculateRetryDuration(date)
	}

	// Пробуем парсить как RFC3339
	if date, err := time.Parse(time.RFC3339, retryAfter); err == nil {
		return calculateRetryDuration(date)
	}

	return 0, fmt.Errorf("invalid retry-after format: %s", retryAfter)
}

// calculateRetryDuration вычисляет длительность до указанной даты
func calculateRetryDuration(date time.Time) (time.Duration, error) {
	now := time.Now()
	if date.After(now) {
		duration := date.Sub(now)
		// Ограничиваем максимальную задержку (например, 24 часа)
		if duration > 24*time.Hour {
			return 0, fmt.Errorf("retry-after date too far in the future: %v", date)
		}
		return duration, nil
	}
	return 0, nil
}

// APIError представляет ошибку API
type APIError struct {
	Type        ErrorType
	StatusCode  int
	Message     string
	OriginalErr error
	RetryAfter  string
}

func (e *APIError) Error() string {
	if e.OriginalErr != nil {
		return fmt.Sprintf("%s (status: %d): %s: %v", e.Type, e.StatusCode, e.Message, e.OriginalErr)
	}
	return fmt.Sprintf("%s (status: %d): %s", e.Type, e.StatusCode, e.Message)
}

func (e *APIError) Unwrap() error {
	return e.OriginalErr
}

// ErrorType тип ошибки
type ErrorType string

const (
	ErrorTypeNetwork          ErrorType = "network"
	ErrorTypeTimeout          ErrorType = "timeout"
	ErrorTypeClient           ErrorType = "client"
	ErrorTypeServer           ErrorType = "server"
	ErrorTypeRateLimit        ErrorType = "rate_limit"
	ErrorTypeParse            ErrorType = "parse"
	ErrorTypeResponseTooLarge ErrorType = "response_too_large"
	ErrorTypeUnknown          ErrorType = "unknown"
)

// Config Конфигурация с максимальным размером ответа
type Config struct {
	BaseURL         string
	Timeout         int // Секунды, храним только для конфигурации
	MaxRetries      int
	RetryDelay      int
	UserAgent       string
	MaxResponseSize int // в байтах
	MaxBatchSize    int // максимальный размер пачки
}

// DefaultConfig возвращает конфигурацию по умолчанию
func DefaultConfig() Config {
	return Config{
		BaseURL:         "http://localhost:8081",
		Timeout:         30,
		MaxRetries:      3,
		RetryDelay:      1,
		UserAgent:       "AuditClient/1.0",
		MaxResponseSize: 10 * 1024 * 1024, // 10 MB
		MaxBatchSize:    1000,             // максимальное количество событий в пачке
	}
}
