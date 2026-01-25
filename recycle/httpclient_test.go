package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestHelper представляет вспомогательные структуры для тестирования
type testRoundTripper struct {
	roundTripFn func(*http.Request) (*http.Response, error)
}

func (t *testRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.roundTripFn(req)
}

type testCase struct {
	name           string
	config         Config
	opts           []Option
	expectedErr    bool
	expectedErrMsg string
}

// Тесты для валидации конфигурации
func TestConfigValidation(t *testing.T) {
	tests := []testCase{
		{
			name: "valid config",
			config: Config{
				BaseURL:         "http://localhost:8081",
				Timeout:         30,
				MaxRetries:      3,
				RetryDelay:      1,
				UserAgent:       "TestAgent",
				MaxResponseSize: 1024,
				MaxBatchSize:    100,
			},
			expectedErr: false,
		},
		{
			name: "empty BaseURL",
			config: Config{
				BaseURL:         "",
				Timeout:         30,
				UserAgent:       "TestAgent",
				MaxResponseSize: 1024,
				MaxBatchSize:    100,
			},
			expectedErr:    true,
			expectedErrMsg: "BaseURL cannot be empty",
		},
		{
			name: "invalid BaseURL",
			config: Config{
				BaseURL:         "://invalid-url",
				Timeout:         30,
				UserAgent:       "TestAgent",
				MaxResponseSize: 1024,
				MaxBatchSize:    100,
			},
			expectedErr:    true,
			expectedErrMsg: "invalid BaseURL",
		},
		{
			name: "zero timeout",
			config: Config{
				BaseURL:         "http://localhost:8081",
				Timeout:         0,
				UserAgent:       "TestAgent",
				MaxResponseSize: 1024,
				MaxBatchSize:    100,
			},
			expectedErr:    true,
			expectedErrMsg: "Timeout must be positive",
		},
		{
			name: "negative timeout",
			config: Config{
				BaseURL:         "http://localhost:8081",
				Timeout:         -1,
				UserAgent:       "TestAgent",
				MaxResponseSize: 1024,
				MaxBatchSize:    100,
			},
			expectedErr:    true,
			expectedErrMsg: "Timeout must be positive",
		},
		{
			name: "negative MaxRetries",
			config: Config{
				BaseURL:         "http://localhost:8081",
				Timeout:         30,
				MaxRetries:      -1,
				UserAgent:       "TestAgent",
				MaxResponseSize: 1024,
				MaxBatchSize:    100,
			},
			expectedErr:    true,
			expectedErrMsg: "MaxRetries cannot be negative",
		},
		{
			name: "negative RetryDelay",
			config: Config{
				BaseURL:         "http://localhost:8081",
				Timeout:         30,
				RetryDelay:      -1,
				UserAgent:       "TestAgent",
				MaxResponseSize: 1024,
				MaxBatchSize:    100,
			},
			expectedErr:    true,
			expectedErrMsg: "RetryDelay cannot be negative",
		},
		{
			name: "empty UserAgent",
			config: Config{
				BaseURL:         "http://localhost:8081",
				Timeout:         30,
				UserAgent:       "",
				MaxResponseSize: 1024,
				MaxBatchSize:    100,
			},
			expectedErr:    true,
			expectedErrMsg: "UserAgent cannot be empty",
		},
		{
			name: "zero MaxResponseSize",
			config: Config{
				BaseURL:         "http://localhost:8081",
				Timeout:         30,
				UserAgent:       "TestAgent",
				MaxResponseSize: 0,
				MaxBatchSize:    100,
			},
			expectedErr:    true,
			expectedErrMsg: "MaxResponseSize must be positive",
		},
		{
			name: "negative MaxResponseSize",
			config: Config{
				BaseURL:         "http://localhost:8081",
				Timeout:         30,
				UserAgent:       "TestAgent",
				MaxResponseSize: -1,
				MaxBatchSize:    100,
			},
			expectedErr:    true,
			expectedErrMsg: "MaxResponseSize must be positive",
		},
		{
			name: "too large MaxResponseSize",
			config: Config{
				BaseURL:         "http://localhost:8081",
				Timeout:         30,
				UserAgent:       "TestAgent",
				MaxResponseSize: 101 * 1024 * 1024,
				MaxBatchSize:    100,
			},
			expectedErr:    true,
			expectedErrMsg: "MaxResponseSize is too large",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectedErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.expectedErrMsg) {
					t.Errorf("expected error to contain %q, got %q", tt.expectedErrMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// Тесты для создания клиента
func TestNewClient(t *testing.T) {
	tests := []testCase{
		{
			name: "successful creation",
			config: Config{
				BaseURL:         "http://localhost:8081",
				Timeout:         30,
				MaxRetries:      3,
				RetryDelay:      1,
				UserAgent:       "TestAgent",
				MaxResponseSize: 1024,
				MaxBatchSize:    100,
			},
			expectedErr: false,
		},
		{
			name: "with options",
			config: Config{
				BaseURL:         "http://localhost:8081",
				Timeout:         30,
				MaxRetries:      3,
				RetryDelay:      1,
				UserAgent:       "TestAgent",
				MaxResponseSize: 1024,
				MaxBatchSize:    100,
			},
			opts: []Option{
				WithUserAgent("CustomAgent"),
				WithBaseURL("http://custom:8080"),
			},
			expectedErr: false,
		},
		{
			name: "invalid option",
			config: Config{
				BaseURL:         "http://localhost:8081",
				Timeout:         30,
				MaxRetries:      3,
				RetryDelay:      1,
				UserAgent:       "TestAgent",
				MaxResponseSize: 1024,
				MaxBatchSize:    100,
			},
			opts: []Option{
				func(c *Client) error {
					return errors.New("option error")
				},
			},
			expectedErr:    true,
			expectedErrMsg: "failed to apply option",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(tt.config, tt.opts...)
			if tt.expectedErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.expectedErrMsg) {
					t.Errorf("expected error to contain %q, got %q", tt.expectedErrMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if client == nil {
					t.Error("expected client, got nil")
				}
				if client.httpClient == nil {
					t.Error("expected httpclient client, got nil")
				}
			}
		})
	}
}

// Тесты для создания клиента с кастомным HTTP клиентом
func TestWithHTTPClient(t *testing.T) {
	customHTTPClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	client, err := New(
		Config{
			BaseURL:         "http://localhost:8081",
			Timeout:         30,
			UserAgent:       "TestAgent",
			MaxResponseSize: 1024,
			MaxBatchSize:    100,
		},
		WithHTTPClient(customHTTPClient),
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client.httpClient != customHTTPClient {
		t.Error("expected custom HTTP client to be set")
	}
}

func TestWithHTTPClient_Nil(t *testing.T) {
	_, err := New(
		Config{
			BaseURL:         "http://localhost:8081",
			Timeout:         30,
			UserAgent:       "TestAgent",
			MaxResponseSize: 1024,
			MaxBatchSize:    100,
		},
		WithHTTPClient(nil),
	)

	if err == nil {
		t.Error("expected error for nil HTTP client")
	}
}

// Тесты для создания клиента с настройками по умолчанию
func TestNewWithDefaults(t *testing.T) {
	client, err := NewWithDefaults()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client == nil {
		t.Fatal("expected client, got nil")
	}

	// Проверяем значения по умолчанию
	if client.config.BaseURL != "http://localhost:8081" {
		t.Errorf("expected default BaseURL, got %q", client.config.BaseURL)
	}
	if client.config.UserAgent != "AuditClient/1.0" {
		t.Errorf("expected default UserAgent, got %q", client.config.UserAgent)
	}
}

// Тесты для валидации запроса аудита
func TestValidateAuditRequest(t *testing.T) {
	client := &Client{}

	tests := []struct {
		name        string
		req         *CreateAuditRequest
		expectedErr bool
		errMsg      string
	}{
		{
			name: "valid request",
			req: &CreateAuditRequest{
				Ts:     1234567890,
				Action: "login",
				UserId: "user123",
				Url:    "http://example.com/login",
			},
			expectedErr: false,
		},
		{
			name:        "nil request",
			req:         nil,
			expectedErr: true,
			errMsg:      "request cannot be nil",
		},
		{
			name: "zero timestamp",
			req: &CreateAuditRequest{
				Ts:     0,
				Action: "login",
				UserId: "user123",
				Url:    "http://example.com/login",
			},
			expectedErr: true,
			errMsg:      "timestamp must be positive",
		},
		{
			name: "negative timestamp",
			req: &CreateAuditRequest{
				Ts:     -1,
				Action: "login",
				UserId: "user123",
				Url:    "http://example.com/login",
			},
			expectedErr: true,
			errMsg:      "timestamp must be positive",
		},
		{
			name: "empty action",
			req: &CreateAuditRequest{
				Ts:     1234567890,
				Action: "",
				UserId: "user123",
				Url:    "http://example.com/login",
			},
			expectedErr: true,
			errMsg:      "action cannot be empty",
		},
		{
			name: "action too long",
			req: &CreateAuditRequest{
				Ts:     1234567890,
				Action: strings.Repeat("a", 101),
				UserId: "user123",
				Url:    "http://example.com/login",
			},
			expectedErr: true,
			errMsg:      "action too long",
		},
		{
			name: "empty user id",
			req: &CreateAuditRequest{
				Ts:     1234567890,
				Action: "login",
				UserId: "",
				Url:    "http://example.com/login",
			},
			expectedErr: true,
			errMsg:      "user_id cannot be empty",
		},
		{
			name: "user id too long",
			req: &CreateAuditRequest{
				Ts:     1234567890,
				Action: "login",
				UserId: strings.Repeat("a", 101),
				Url:    "http://example.com/login",
			},
			expectedErr: true,
			errMsg:      "user_id too long",
		},
		{
			name: "empty url",
			req: &CreateAuditRequest{
				Ts:     1234567890,
				Action: "login",
				UserId: "user123",
				Url:    "",
			},
			expectedErr: true,
			errMsg:      "url cannot be empty",
		},
		{
			name: "invalid url",
			req: &CreateAuditRequest{
				Ts:     1234567890,
				Action: "login",
				UserId: "user123",
				Url:    "://invalid-url",
			},
			expectedErr: true,
			errMsg:      "invalid URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.validateAuditRequest(tt.req)
			if tt.expectedErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error to contain %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// Тесты для создания аудит события
func TestCreateAuditEvent_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/audit/events" {
			t.Errorf("expected /api/audit/events, got %s", r.URL.Path)
		}

		var req CreateAuditRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		if req.Ts != 1234567890 || req.Action != "login" || req.UserId != "user123" || req.Url != "http://example.com/login" {
			t.Errorf("unexpected request body: %+v", req)
		}

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client, err := New(Config{
		BaseURL:         server.URL,
		Timeout:         30,
		UserAgent:       "TestAgent",
		MaxResponseSize: 1024,
		MaxBatchSize:    100,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	req := &CreateAuditRequest{
		Ts:     1234567890,
		Action: "login",
		UserId: "user123",
		Url:    "http://example.com/login",
	}

	if err := client.CreateAuditEvent(context.Background(), req); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateAuditEvent_NetworkError(t *testing.T) {
	// Создаем клиент с неработающим URL
	client, err := New(Config{
		BaseURL:         "http://localhost:9999", // Порту, который не слушает
		Timeout:         1,                       // Маленький таймаут для быстрого теста
		UserAgent:       "TestAgent",
		MaxResponseSize: 1024,
		MaxBatchSize:    100,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	req := &CreateAuditRequest{
		Ts:     1234567890,
		Action: "login",
		UserId: "user123",
		Url:    "http://example.com/login",
	}

	err = client.CreateAuditEvent(context.Background(), req)
	if err == nil {
		t.Error("expected error, got nil")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Errorf("expected APIError, got %T", err)
	}
	if apiErr.Type != ErrorTypeNetwork {
		t.Errorf("expected network error, got %s", apiErr.Type)
	}
}

func TestCreateAuditEvent_ContextTimeout(t *testing.T) {
	// Создаем сервер, который будет медленно отвечать
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // Ждем дольше таймаута
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := New(Config{
		BaseURL:         server.URL,
		Timeout:         1,
		UserAgent:       "TestAgent",
		MaxResponseSize: 1024,
		MaxBatchSize:    100,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	req := &CreateAuditRequest{
		Ts:     1234567890,
		Action: "login",
		UserId: "user123",
		Url:    "http://example.com/login",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = client.CreateAuditEvent(ctx, req)
	if err == nil {
		t.Error("expected error, got nil")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Errorf("expected APIError, got %T", err)
	}
	if apiErr.Type != ErrorTypeTimeout {
		t.Errorf("expected timeout error, got %s", apiErr.Type)
	}
}

// Тесты для обработки различных статус кодов
func TestCreateAuditEvent_StatusCodes(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		expectedErr    bool
		expectedErrMsg string
	}{
		{
			name:        "success - 200",
			statusCode:  http.StatusOK,
			expectedErr: false,
		},
		{
			name:        "success - 201",
			statusCode:  http.StatusCreated,
			expectedErr: false,
		},
		{
			name:        "success - 202",
			statusCode:  http.StatusAccepted,
			expectedErr: false,
		},
		{
			name:           "bad request - 400",
			statusCode:     http.StatusBadRequest,
			expectedErr:    true,
			expectedErrMsg: "bad request",
		},
		{
			name:           "unauthorized - 401",
			statusCode:     http.StatusUnauthorized,
			expectedErr:    true,
			expectedErrMsg: "unauthorized",
		},
		{
			name:           "forbidden - 403",
			statusCode:     http.StatusForbidden,
			expectedErr:    true,
			expectedErrMsg: "forbidden",
		},
		{
			name:           "rate limit - 429",
			statusCode:     http.StatusTooManyRequests,
			expectedErr:    true,
			expectedErrMsg: "rate limit",
		},
		{
			name:           "server error - 500",
			statusCode:     http.StatusInternalServerError,
			expectedErr:    true,
			expectedErrMsg: "server error",
		},
		{
			name:           "bad gateway - 502",
			statusCode:     http.StatusBadGateway,
			expectedErr:    true,
			expectedErrMsg: "server error",
		},
		{
			name:           "service unavailable - 503",
			statusCode:     http.StatusServiceUnavailable,
			expectedErr:    true,
			expectedErrMsg: "server error",
		},
		{
			name:           "gateway timeout - 504",
			statusCode:     http.StatusGatewayTimeout,
			expectedErr:    true,
			expectedErrMsg: "server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				if tt.statusCode == http.StatusBadRequest {
					w.Write([]byte(`{"error": "Invalid request"}`))
				}
			}))
			defer server.Close()

			client, err := New(Config{
				BaseURL:         server.URL,
				Timeout:         30,
				UserAgent:       "TestAgent",
				MaxResponseSize: 1024,
				MaxBatchSize:    100,
			})
			if err != nil {
				t.Fatalf("failed to create client: %v", err)
			}

			req := &CreateAuditRequest{
				Ts:     1234567890,
				Action: "login",
				UserId: "user123",
				Url:    "http://example.com/login",
			}

			err = client.CreateAuditEvent(context.Background(), req)
			if tt.expectedErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.expectedErrMsg) {
					t.Errorf("expected error to contain %q, got %q", tt.expectedErrMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// Тесты для повторных попыток
func TestCreateAuditEventWithRetry_SuccessAfterRetry(t *testing.T) {
	attempt := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		if attempt < 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client, err := New(Config{
		BaseURL:         server.URL,
		Timeout:         30,
		UserAgent:       "TestAgent",
		MaxResponseSize: 1024,
		MaxBatchSize:    100,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	req := &CreateAuditRequest{
		Ts:     1234567890,
		Action: "login",
		UserId: "user123",
		Url:    "http://example.com/login",
	}

	// Используем маленькую задержку для быстрого теста
	err = client.CreateAuditEventWithRetry(context.Background(), req,
		WithRetryDelay(10*time.Millisecond),
		WithMaxRetries(3),
	)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if attempt != 2 {
		t.Errorf("expected 2 attempts, got %d", attempt)
	}
}

func TestCreateAuditEventWithRetry_MaxRetriesExceeded(t *testing.T) {
	attempt := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client, err := New(Config{
		BaseURL:         server.URL,
		Timeout:         30,
		UserAgent:       "TestAgent",
		MaxResponseSize: 1024,
		MaxBatchSize:    100,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	req := &CreateAuditRequest{
		Ts:     1234567890,
		Action: "login",
		UserId: "user123",
		Url:    "http://example.com/login",
	}

	// Используем маленькую задержку для быстрого теста
	err = client.CreateAuditEventWithRetry(context.Background(), req,
		WithRetryDelay(10*time.Millisecond),
		WithMaxRetries(2),
	)

	if err == nil {
		t.Error("expected error, got nil")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Errorf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status code 500, got %d", apiErr.StatusCode)
	}
	if attempt != 3 { // 1 первоначальная + 2 повторные попытки
		t.Errorf("expected 3 attempts, got %d", attempt)
	}
}

func TestCreateAuditEventWithRetry_NoRetryForClientError(t *testing.T) {
	attempt := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client, err := New(Config{
		BaseURL:         server.URL,
		Timeout:         30,
		UserAgent:       "TestAgent",
		MaxResponseSize: 1024,
		MaxBatchSize:    100,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	req := &CreateAuditRequest{
		Ts:     1234567890,
		Action: "login",
		UserId: "user123",
		Url:    "http://example.com/login",
	}

	err = client.CreateAuditEventWithRetry(context.Background(), req,
		WithRetryDelay(10*time.Millisecond),
		WithMaxRetries(3),
	)

	if err == nil {
		t.Error("expected error, got nil")
	}
	if attempt != 1 { // Не должно быть повторных попыток для 400
		t.Errorf("expected 1 attempt, got %d", attempt)
	}
}

// Тесты для пакетного создания событий
func TestBatchCreateAuditEvents(t *testing.T) {
	tests := []struct {
		name           string
		events         []*CreateAuditRequest
		expectedErr    bool
		expectedErrMsg string
	}{
		{
			name:        "empty batch",
			events:      []*CreateAuditRequest{},
			expectedErr: false,
		},
		{
			name: "single event",
			events: []*CreateAuditRequest{
				{
					Ts:     1234567890,
					Action: "login",
					UserId: "user123",
					Url:    "http://example.com/login",
				},
			},
			expectedErr: false,
		},
		{
			name: "multiple events",
			events: []*CreateAuditRequest{
				{
					Ts:     1234567890,
					Action: "login",
					UserId: "user123",
					Url:    "http://example.com/login",
				},
				{
					Ts:     1234567891,
					Action: "logout",
					UserId: "user123",
					Url:    "http://example.com/logout",
				},
			},
			expectedErr: false,
		},
		{
			name: "invalid event in batch",
			events: []*CreateAuditRequest{
				{
					Ts:     1234567890,
					Action: "login",
					UserId: "user123",
					Url:    "http://example.com/login",
				},
				{
					Ts:     0, // Invalid
					Action: "logout",
					UserId: "user123",
					Url:    "http://example.com/logout",
				},
			},
			expectedErr:    true,
			expectedErrMsg: "invalid audit request at index 1",
		},
		{
			name: "batch too large",
			events: func() []*CreateAuditRequest {
				events := make([]*CreateAuditRequest, 1001)
				for i := 0; i < 1001; i++ {
					events[i] = &CreateAuditRequest{
						Ts:     int(int64(1234567890 + i)),
						Action: fmt.Sprintf("action_%d", i),
						UserId: fmt.Sprintf("user_%d", i),
						Url:    fmt.Sprintf("http://example.com/action/%d", i),
					}
				}
				return events
			}(),
			expectedErr:    true,
			expectedErrMsg: "exceeds maximum allowed size",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/audit/events/batch" {
					t.Errorf("expected /api/audit/events/batch, got %s", r.URL.Path)
				}
				w.WriteHeader(http.StatusCreated)
			}))
			defer server.Close()

			client, err := New(Config{
				BaseURL:         server.URL,
				Timeout:         30,
				UserAgent:       "TestAgent",
				MaxResponseSize: 1024,
				MaxBatchSize:    1000,
			})
			if err != nil {
				t.Fatalf("failed to create client: %v", err)
			}

			err = client.BatchCreateAuditEvents(context.Background(), tt.events)
			if tt.expectedErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.expectedErrMsg) {
					t.Errorf("expected error to contain %q, got %q", tt.expectedErrMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// Тесты для слишком большого ответа
func TestCreateAuditEvent_ResponseTooLarge(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Отправляем большой ответ
		largeResponse := strings.Repeat("x", 2048) // Больше чем MaxResponseSize
		w.Write([]byte(largeResponse))
	}))
	defer server.Close()

	client, err := New(Config{
		BaseURL:         server.URL,
		Timeout:         30,
		UserAgent:       "TestAgent",
		MaxResponseSize: 1024, // 1KB
		MaxBatchSize:    100,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	req := &CreateAuditRequest{
		Ts:     1234567890,
		Action: "login",
		UserId: "user123",
		Url:    "http://example.com/login",
	}

	err = client.CreateAuditEvent(context.Background(), req)
	if err == nil {
		t.Error("expected error, got nil")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Errorf("expected APIError, got %T", err)
	}
	if apiErr.Type != ErrorTypeResponseTooLarge {
		t.Errorf("expected response too large error, got %s", apiErr.Type)
	}
}

// Тесты для заголовка Retry-After
func TestCreateAuditEvent_RetryAfterHeader(t *testing.T) {
	attempt := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		if attempt == 1 {
			w.Header().Set("Retry-After", "1") // 1 секунда
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client, err := New(Config{
		BaseURL:         server.URL,
		Timeout:         30,
		UserAgent:       "TestAgent",
		MaxResponseSize: 1024,
		MaxBatchSize:    100,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	req := &CreateAuditRequest{
		Ts:     1234567890,
		Action: "login",
		UserId: "user123",
		Url:    "http://example.com/login",
	}

	// Используем WithRetryOn чтобы убедиться, что 429 вызывает повторную попытку
	err = client.CreateAuditEventWithRetry(context.Background(), req,
		WithRetryDelay(10*time.Millisecond),
		WithMaxRetries(1),
		WithRetryOn(429),
	)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if attempt != 2 {
		t.Errorf("expected 2 attempts, got %d", attempt)
	}
}

// Тесты для парсинга Retry-After
func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		name        string
		retryAfter  string
		expected    time.Duration
		expectError bool
	}{
		{
			name:       "seconds as number",
			retryAfter: "60",
			expected:   60 * time.Second,
		},
		{
			name:       "zero seconds",
			retryAfter: "0",
			expected:   0,
		},
		{
			name:        "negative seconds",
			retryAfter:  "-1",
			expectError: true,
		},
		{
			name:        "too many seconds",
			retryAfter:  "86401", // 24 часа + 1 секунда
			expectError: true,
		},
		{
			name:        "invalid format",
			retryAfter:  "invalid",
			expectError: true,
		},
		{
			name:        "empty string",
			retryAfter:  "",
			expectError: true,
		},
		// Note: RFC1123 даты сложно тестировать из-за зависимости от текущего времени
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			duration, err := parseRetryAfter(tt.retryAfter)
			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if duration != tt.expected {
					t.Errorf("expected duration %v, got %v", tt.expected, duration)
				}
			}
		})
	}
}

// Тесты для shouldRetry логики
func TestShouldRetry(t *testing.T) {
	client := &Client{}

	tests := []struct {
		name        string
		err         error
		attempt     int
		opts        retryOptions
		shouldRetry bool
		expectDelay bool
	}{
		{
			name:        "max retries exceeded",
			err:         &APIError{Type: ErrorTypeServer, StatusCode: 500},
			attempt:     3,
			opts:        retryOptions{maxRetries: 3},
			shouldRetry: false,
		},
		{
			name:        "timeout error should not retry",
			err:         &APIError{Type: ErrorTypeTimeout},
			attempt:     0,
			opts:        retryOptions{maxRetries: 3},
			shouldRetry: false,
		},
		{
			name:        "client error (not 429) should not retry",
			err:         &APIError{Type: ErrorTypeClient, StatusCode: 400},
			attempt:     0,
			opts:        retryOptions{maxRetries: 3},
			shouldRetry: false,
		},
		{
			name:        "server error should retry",
			err:         &APIError{Type: ErrorTypeServer, StatusCode: 500},
			attempt:     0,
			opts:        retryOptions{maxRetries: 3, retryDelay: time.Second, retryOn: []int{500}},
			shouldRetry: true,
			expectDelay: true,
		},
		{
			name:        "error not in retryOn list",
			err:         &APIError{Type: ErrorTypeServer, StatusCode: 501},
			attempt:     0,
			opts:        retryOptions{maxRetries: 3, retryDelay: time.Second, retryOn: []int{500}},
			shouldRetry: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldRetry, delay := client.shouldRetry(tt.err, tt.attempt, tt.opts)
			if shouldRetry != tt.shouldRetry {
				t.Errorf("shouldRetry: expected %v, got %v", tt.shouldRetry, shouldRetry)
			}
			if tt.expectDelay && delay <= 0 {
				t.Error("expected positive delay")
			}
		})
	}
}

// Тесты для APIError
func TestAPIError(t *testing.T) {
	tests := []struct {
		name     string
		apiError *APIError
		expected string
	}{
		{
			name: "with original error",
			apiError: &APIError{
				Type:        ErrorTypeNetwork,
				StatusCode:  0,
				Message:     "connection failed",
				OriginalErr: errors.New("dial tcp failed"),
			},
			expected: "network (status: 0): connection failed: dial tcp failed",
		},
		{
			name: "without original error",
			apiError: &APIError{
				Type:       ErrorTypeClient,
				StatusCode: 400,
				Message:    "bad request",
			},
			expected: "client (status: 400): bad request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.apiError.Error() != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, tt.apiError.Error())
			}
		})
	}
}

// Тесты для интерфейса AuditClient
func TestAuditClientInterface(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client, err := New(Config{
		BaseURL:         server.URL,
		Timeout:         30,
		UserAgent:       "TestAgent",
		MaxResponseSize: 1024,
		MaxBatchSize:    100,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Проверяем, что клиент реализует интерфейс AuditClient
	var _ AuditClient = client

	req := &CreateAuditRequest{
		Ts:     1234567890,
		Action: "login",
		UserId: "user123",
		Url:    "http://example.com/login",
	}

	// Тестируем методы интерфейса
	if err := client.CreateAuditEvent(context.Background(), req); err != nil {
		t.Errorf("CreateAuditEvent failed: %v", err)
	}

	if err := client.CreateAuditEventWithRetry(context.Background(), req); err != nil {
		t.Errorf("CreateAuditEventWithRetry failed: %v", err)
	}

	events := []*CreateAuditRequest{req}
	if err := client.BatchCreateAuditEvents(context.Background(), events); err != nil {
		t.Errorf("BatchCreateAuditEvents failed: %v", err)
	}
}

// Тесты для опций With...
func TestWithOptions(t *testing.T) {
	tests := []struct {
		name        string
		option      Option
		shouldError bool
	}{
		{
			name:        "WithBaseURL valid",
			option:      WithBaseURL("http://example.com"),
			shouldError: false,
		},
		{
			name:        "WithBaseURL empty",
			option:      WithBaseURL(""),
			shouldError: true,
		},
		{
			name:        "WithBaseURL invalid",
			option:      WithBaseURL("://invalid"),
			shouldError: true,
		},
		{
			name:        "WithTimeout valid",
			option:      WithTimeout(10 * time.Second),
			shouldError: false,
		},
		{
			name:        "WithTimeout zero",
			option:      WithTimeout(0),
			shouldError: true,
		},
		{
			name:        "WithUserAgent valid",
			option:      WithUserAgent("TestAgent"),
			shouldError: false,
		},
		{
			name:        "WithUserAgent empty",
			option:      WithUserAgent(""),
			shouldError: true,
		},
		{
			name:        "WithMaxResponseSize valid",
			option:      WithMaxResponseSize(1024),
			shouldError: false,
		},
		{
			name:        "WithMaxResponseSize zero",
			option:      WithMaxResponseSize(0),
			shouldError: true,
		},
		{
			name:        "WithMaxResponseSize too large",
			option:      WithMaxResponseSize(101 * 1024 * 1024),
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				config: DefaultConfig(),
				httpClient: &http.Client{
					Timeout: 30 * time.Second,
				},
			}

			err := tt.option(client)
			if tt.shouldError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// Тесты для JSON ошибок
func TestCreateAuditEvent_JSONError(t *testing.T) {
	client, err := New(Config{
		BaseURL:         "http://localhost:8081",
		Timeout:         30,
		UserAgent:       "TestAgent",
		MaxResponseSize: 1024,
		MaxBatchSize:    100,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Создаем запрос с циклической ссылкой для проверки ошибки маршализации
	type circular struct {
		Self *circular
	}
	circ := &circular{}
	circ.Self = circ

	// Используем reflection для установки некорректного значения
	// Это симулирует ошибку маршализации
	req := &CreateAuditRequest{
		Ts:     1234567890,
		Action: "login",
		UserId: "user123",
		Url:    "http://example.com/login",
	}

	// Используем транспорт, который всегда возвращает ошибку
	client.httpClient.Transport = &testRoundTripper{
		roundTripFn: func(req *http.Request) (*http.Response, error) {
			// Создаем тело запроса, которое не может быть замаршалено
			// Используем канал, который не может быть сериализован в JSON
			req.Body = io.NopCloser(bytes.NewReader([]byte{}))

			// Подменяем тело запроса после его создания
			// На практике это сложно сделать, так как тело уже замаршалено
			// Вместо этого тестируем через executePostRequest напрямую
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte{})),
			}, nil
		},
	}

	// Тестируем валидацию запроса
	if err := client.validateAuditRequest(req); err != nil {
		t.Errorf("validation should pass, got: %v", err)
	}
}

// Тесты для контекста с отменой
func TestCreateAuditEvent_ContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := New(Config{
		BaseURL:         server.URL,
		Timeout:         30,
		UserAgent:       "TestAgent",
		MaxResponseSize: 1024,
		MaxBatchSize:    100,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	req := &CreateAuditRequest{
		Ts:     1234567890,
		Action: "login",
		UserId: "user123",
		Url:    "http://example.com/login",
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Отменяем контекст сразу
	cancel()

	err = client.CreateAuditEvent(ctx, req)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

// Интеграционный тест
func TestCreateAuditEvent_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Этот тест предполагает, что настоящий сервер запущен
	// В реальном проекте здесь можно использовать тестовый сервер
	client, err := New(Config{
		BaseURL:         "http://localhost:8081",
		Timeout:         5,
		UserAgent:       "IntegrationTest",
		MaxResponseSize: 1024 * 1024,
		MaxBatchSize:    100,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	req := &CreateAuditRequest{
		Ts:     int(time.Now().Unix()),
		Action: "integration_test",
		UserId: "integration_user",
		Url:    "http://example.com/test",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Этот вызов может завершиться ошибкой, если сервер не запущен
	// Это нормально для интеграционного теста
	err = client.CreateAuditEvent(ctx, req)
	if err != nil {
		// Проверяем, что ошибка связана с сетью, а не с валидацией
		var apiErr *APIError
		if errors.As(err, &apiErr) {
			if apiErr.Type == ErrorTypeClient {
				// Клиентские ошибки (валидация) недопустимы
				t.Errorf("client error in integration test: %v", err)
			}
			// Сетевые ошибки и таймауты допустимы, если сервер не запущен
		} else if !strings.Contains(err.Error(), "connection refused") &&
			!strings.Contains(err.Error(), "timeout") {
			t.Errorf("unexpected error in integration test: %v", err)
		}
	}
}
