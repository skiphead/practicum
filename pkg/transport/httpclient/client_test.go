package httpclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name:    "valid config",
			config:  DefaultConfig(),
			wantErr: false,
		},
		{
			name: "empty BaseURL",
			config: Config{
				BaseURL:         "",
				Timeout:         30,
				UserAgent:       "test",
				MaxResponseSize: 1024,
				MaxBatchSize:    100,
			},
			wantErr: true,
		},
		{
			name: "invalid BaseURL",
			config: Config{
				BaseURL:         "://invalid",
				Timeout:         30,
				UserAgent:       "test",
				MaxResponseSize: 1024,
				MaxBatchSize:    100,
			},
			wantErr: true,
		},
		{
			name: "zero Timeout",
			config: Config{
				BaseURL:         "http://localhost",
				Timeout:         0,
				UserAgent:       "test",
				MaxResponseSize: 1024,
				MaxBatchSize:    100,
			},
			wantErr: true,
		},
		{
			name: "negative MaxRetries",
			config: Config{
				BaseURL:         "http://localhost",
				Timeout:         30,
				MaxRetries:      -1,
				UserAgent:       "test",
				MaxResponseSize: 1024,
				MaxBatchSize:    100,
			},
			wantErr: true,
		},
		{
			name: "empty UserAgent",
			config: Config{
				BaseURL:         "http://localhost",
				Timeout:         30,
				UserAgent:       "",
				MaxResponseSize: 1024,
				MaxBatchSize:    100,
			},
			wantErr: true,
		},
		{
			name: "zero MaxResponseSize",
			config: Config{
				BaseURL:         "http://localhost",
				Timeout:         30,
				UserAgent:       "test",
				MaxResponseSize: 0,
				MaxBatchSize:    100,
			},
			wantErr: true,
		},
		{
			name: "too large MaxResponseSize",
			config: Config{
				BaseURL:         "http://localhost",
				Timeout:         30,
				UserAgent:       "test",
				MaxResponseSize: 200 * 1024 * 1024,
				MaxBatchSize:    100,
			},
			wantErr: true,
		},
		{
			name: "zero MaxBatchSize",
			config: Config{
				BaseURL:         "http://localhost",
				Timeout:         30,
				UserAgent:       "test",
				MaxResponseSize: 1024,
				MaxBatchSize:    0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNew(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		config := DefaultConfig()
		client, err := New(config)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		if client == nil {
			t.Fatal("New() returned nil client")
		}
		if client.config.BaseURL != config.BaseURL {
			t.Errorf("BaseURL = %v, want %v", client.config.BaseURL, config.BaseURL)
		}
	})

	t.Run("invalid config", func(t *testing.T) {
		config := Config{}
		client, err := New(config)
		if err == nil {
			t.Fatal("New() expected error, got nil")
		}
		if client != nil {
			t.Fatal("New() expected nil client on error")
		}
	})

	t.Run("with custom http client", func(t *testing.T) {
		config := DefaultConfig()
		customClient := &http.Client{Timeout: 5 * time.Second}
		client, err := New(config, WithHTTPClient(customClient))
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		if client.httpClient != customClient {
			t.Error("WithHTTPClient() didn't set custom client")
		}
	})

	t.Run("with nil http client", func(t *testing.T) {
		config := DefaultConfig()
		_, err := New(config, WithHTTPClient(nil))
		if err == nil {
			t.Fatal("WithHTTPClient(nil) expected error, got nil")
		}
	})

	t.Run("with base url option", func(t *testing.T) {
		config := DefaultConfig()
		newURL := "http://example.com"
		client, err := New(config, WithBaseURL(newURL))
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		if client.config.BaseURL != newURL {
			t.Errorf("BaseURL = %v, want %v", client.config.BaseURL, newURL)
		}
	})

	t.Run("with invalid base url option", func(t *testing.T) {
		config := DefaultConfig()
		_, err := New(config, WithBaseURL("://invalid"))
		if err == nil {
			t.Fatal("WithBaseURL(invalid) expected error, got nil")
		}
	})

	t.Run("with timeout option", func(t *testing.T) {
		config := DefaultConfig()
		timeout := 10 * time.Second
		client, err := New(config, WithTimeout(timeout))
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		if client.httpClient.Timeout != timeout {
			t.Errorf("Timeout = %v, want %v", client.httpClient.Timeout, timeout)
		}
	})

	t.Run("with zero timeout option", func(t *testing.T) {
		config := DefaultConfig()
		_, err := New(config, WithTimeout(0))
		if err == nil {
			t.Fatal("WithTimeout(0) expected error, got nil")
		}
	})
}

func TestHTTPClient_SendRequest(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/success":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		case "/created":
			w.WriteHeader(http.StatusCreated)
		case "/bad-request":
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("invalid request"))
		case "/unauthorized":
			w.WriteHeader(http.StatusUnauthorized)
		case "/forbidden":
			w.WriteHeader(http.StatusForbidden)
		case "/rate-limit":
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
		case "/server-error":
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal server error"))
		case "/large-response":
			// Генерируем ответ больше лимита
			largeData := strings.Repeat("x", 1024*1024) // 1MB
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(largeData))
		case "/truncated-response":
			// Ответ точно на границе лимита
			w.Header().Set("Content-Length", "1025")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(strings.Repeat("x", 1024))) // ровно 1KB
		case "/slow":
			time.Sleep(2 * time.Second)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer testServer.Close()

	config := DefaultConfig()
	config.BaseURL = testServer.URL
	config.MaxResponseSize = 1024 // 1KB для тестов

	client, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	tests := []struct {
		name        string
		method      string
		path        string
		body        interface{}
		wantErr     bool
		wantErrType ErrorType
	}{
		{
			name:    "successful request",
			method:  http.MethodPost,
			path:    "/success",
			body:    map[string]string{"test": "data"},
			wantErr: false,
		},
		{
			name:    "created",
			method:  http.MethodPost,
			path:    "/created",
			body:    map[string]string{"test": "data"},
			wantErr: false,
		},
		{
			name:        "bad request",
			method:      http.MethodPost,
			path:        "/bad-request",
			body:        map[string]string{"test": "data"},
			wantErr:     true,
			wantErrType: ErrorTypeClient,
		},
		{
			name:        "unauthorized",
			method:      http.MethodGet,
			path:        "/unauthorized",
			wantErr:     true,
			wantErrType: ErrorTypeClient,
		},
		{
			name:        "forbidden",
			method:      http.MethodGet,
			path:        "/forbidden",
			wantErr:     true,
			wantErrType: ErrorTypeClient,
		},
		{
			name:        "rate limit",
			method:      http.MethodGet,
			path:        "/rate-limit",
			wantErr:     true,
			wantErrType: ErrorTypeRateLimit,
		},
		{
			name:        "server error",
			method:      http.MethodGet,
			path:        "/server-error",
			wantErr:     true,
			wantErrType: ErrorTypeServer,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := client.SendRequest(ctx, tt.method, tt.path, tt.body)

			if (err != nil) != tt.wantErr {
				t.Errorf("SendRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil {
				var apiErr *APIError
				if errors.As(err, &apiErr) {
					if apiErr.Type != tt.wantErrType {
						t.Errorf("Error type = %v, want %v", apiErr.Type, tt.wantErrType)
					}
				} else {
					t.Error("Error is not of type APIError")
				}
			}
		})
	}

	t.Run("response too large", func(t *testing.T) {
		ctx := context.Background()
		err := client.SendRequest(ctx, http.MethodGet, "/large-response", nil)
		if err == nil {
			t.Fatal("Expected error for large response")
		}

		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			t.Fatal("Error is not of type APIError")
		}
		if apiErr.Type != ErrorTypeResponseTooLarge {
			t.Errorf("Error type = %v, want %v", apiErr.Type, ErrorTypeResponseTooLarge)
		}
	})

	t.Run("context timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := client.SendRequest(ctx, http.MethodGet, "/slow", nil)
		if err == nil {
			t.Fatal("Expected timeout error")
		}

		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			t.Fatal("Error is not of type APIError")
		}
		if apiErr.Type != ErrorTypeTimeout {
			t.Errorf("Error type = %v, want %v", apiErr.Type, ErrorTypeTimeout)
		}
	})

	t.Run("canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := client.SendRequest(ctx, http.MethodGet, "/success", nil)
		if err == nil {
			t.Fatal("Expected error for canceled context")
		}

		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			t.Fatal("Error is not of type APIError")
		}
		if apiErr.Type != ErrorTypeTimeout {
			t.Errorf("Error type = %v, want %v", apiErr.Type, ErrorTypeTimeout)
		}
	})
}

func TestAPIError_Error(t *testing.T) {
	tests := []struct {
		name    string
		err     *APIError
		wantStr string
	}{
		{
			name: "with original error",
			err: &APIError{
				Type:        ErrorTypeClient,
				StatusCode:  400,
				Message:     "bad request",
				OriginalErr: fmt.Errorf("validation failed"),
			},
			wantStr: "client (status: 400): bad request: validation failed",
		},
		{
			name: "without original error",
			err: &APIError{
				Type:       ErrorTypeServer,
				StatusCode: 500,
				Message:    "internal error",
			},
			wantStr: "server (status: 500): internal error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.wantStr {
				t.Errorf("Error() = %v, want %v", got, tt.wantStr)
			}
		})
	}
}

func TestHTTPClient_ShouldRetry(t *testing.T) {
	config := DefaultConfig()
	client, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	tests := []struct {
		name      string
		err       error
		attempt   int
		opts      RetryOptions
		wantRetry bool
		wantDelay bool
	}{
		{
			name:      "max retries exceeded",
			err:       &APIError{Type: ErrorTypeServer, StatusCode: 500},
			attempt:   3,
			opts:      RetryOptions{MaxRetries: 3},
			wantRetry: false,
			wantDelay: false,
		},
		{
			name:      "client error not retried",
			err:       &APIError{Type: ErrorTypeClient, StatusCode: 400},
			attempt:   0,
			opts:      DefaultRetryOptions,
			wantRetry: false,
			wantDelay: false,
		},
		{
			name:      "timeout error not retried",
			err:       &APIError{Type: ErrorTypeTimeout},
			attempt:   0,
			opts:      DefaultRetryOptions,
			wantRetry: false,
			wantDelay: false,
		},
		{
			name:      "rate limit retried",
			err:       &APIError{Type: ErrorTypeRateLimit, StatusCode: 429},
			attempt:   0,
			opts:      DefaultRetryOptions,
			wantRetry: true,
			wantDelay: true,
		},
		{
			name:      "server error retried",
			err:       &APIError{Type: ErrorTypeServer, StatusCode: 500},
			attempt:   0,
			opts:      DefaultRetryOptions,
			wantRetry: true,
			wantDelay: true,
		},
		{
			name:      "not API error",
			err:       fmt.Errorf("some other error"),
			attempt:   0,
			opts:      DefaultRetryOptions,
			wantRetry: false,
			wantDelay: false,
		},
		{
			name:      "status code not in retry list",
			err:       &APIError{Type: ErrorTypeClient, StatusCode: 404},
			attempt:   0,
			opts:      RetryOptions{MaxRetries: 3, RetryOn: []int{500, 502}},
			wantRetry: false,
			wantDelay: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retry, delay := client.ShouldRetry(tt.err, tt.attempt, tt.opts)

			if retry != tt.wantRetry {
				t.Errorf("ShouldRetry() retry = %v, want %v", retry, tt.wantRetry)
			}

			if tt.wantDelay && delay <= 0 {
				t.Error("ShouldRetry() expected positive delay")
			}
			if !tt.wantDelay && delay != 0 {
				t.Errorf("ShouldRetry() delay = %v, want 0", delay)
			}
		})
	}
}

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		name        string
		retryAfter  string
		wantErr     bool
		wantMinSecs int
		wantMaxSecs int
	}{
		{
			name:        "valid seconds",
			retryAfter:  "60",
			wantErr:     false,
			wantMinSecs: 59,
			wantMaxSecs: 61,
		},
		{
			name:       "negative seconds",
			retryAfter: "-1",
			wantErr:    true,
		},
		{
			name:       "too large seconds",
			retryAfter: "100000",
			wantErr:    true,
		},
		{
			name:        "RFC1123 date",
			retryAfter:  time.Now().Add(30 * time.Second).Format(time.RFC1123),
			wantErr:     false,
			wantMinSecs: 29,
			wantMaxSecs: 31,
		},
		{
			name:       "empty string",
			retryAfter: "",
			wantErr:    true,
		},
		{
			name:       "invalid format",
			retryAfter: "invalid",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			duration, err := parseRetryAfter(tt.retryAfter)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseRetryAfter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				secs := int(duration.Seconds())
				if secs < tt.wantMinSecs || secs > tt.wantMaxSecs {
					t.Errorf("parseRetryAfter() duration = %v seconds, want between %v and %v",
						secs, tt.wantMinSecs, tt.wantMaxSecs)
				}
			}
		})
	}
}

func TestSendRequest_MarshalError(t *testing.T) {
	config := DefaultConfig()
	client, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Тело, которое нельзя сериализовать
	invalidBody := make(chan int)

	ctx := context.Background()
	err = client.SendRequest(ctx, http.MethodPost, "/test", invalidBody)
	if err == nil {
		t.Fatal("Expected marshaling error")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatal("Error is not of type APIError")
	}
	if apiErr.Type != ErrorTypeClient {
		t.Errorf("Error type = %v, want %v", apiErr.Type, ErrorTypeClient)
	}
}

func TestSendRequest_NetworkError(t *testing.T) {
	// Сервер, который закрывается сразу
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Немедленно закрываем соединение
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("Cannot hijack connection")
		}
		conn, _, _ := hj.Hijack()
		conn.Close()
	}))
	testServer.Close() // Закрываем сразу, чтобы соединение было недоступно

	config := DefaultConfig()
	config.BaseURL = testServer.URL
	client, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()
	err = client.SendRequest(ctx, http.MethodGet, "/", nil)
	if err == nil {
		t.Fatal("Expected network error")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatal("Error is not of type APIError")
	}
	if apiErr.Type != ErrorTypeNetwork {
		t.Errorf("Error type = %v, want %v", apiErr.Type, ErrorTypeNetwork)
	}
}

func TestWithRetryOptions(t *testing.T) {
	opts := DefaultRetryOptions

	// Тестируем WithMaxRetries
	WithMaxRetries(5)(&opts)
	if opts.MaxRetries != 5 {
		t.Errorf("MaxRetries = %v, want 5", opts.MaxRetries)
	}

	// Тестируем WithMaxRetries с отрицательным значением
	WithMaxRetries(-1)(&opts)
	if opts.MaxRetries != 0 {
		t.Errorf("MaxRetries with negative = %v, want 0", opts.MaxRetries)
	}

	// Тестируем WithRetryDelay
	WithRetryDelay(2 * time.Second)(&opts)
	if opts.RetryDelay != 2*time.Second {
		t.Errorf("RetryDelay = %v, want 2s", opts.RetryDelay)
	}

	// Тестируем WithMaxWaitTime
	WithMaxWaitTime(60 * time.Second)(&opts)
	if opts.MaxWaitTime != 60*time.Second {
		t.Errorf("MaxWaitTime = %v, want 60s", opts.MaxWaitTime)
	}

	// Тестируем WithRetryOn
	WithRetryOn(408, 500)(&opts)
	if len(opts.RetryOn) != 2 || opts.RetryOn[0] != 408 || opts.RetryOn[1] != 500 {
		t.Errorf("RetryOn = %v, want [408, 500]", opts.RetryOn)
	}
}

func TestSendRequest_ReadBodyError(t *testing.T) {
	// Сервер, который отправляет неполный ответ
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("short response")) // Меньше чем Content-Length
		// Соединение обрывается
	}))
	defer testServer.Close()

	config := DefaultConfig()
	config.BaseURL = testServer.URL
	client, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()
	err = client.SendRequest(ctx, http.MethodGet, "/", nil)
	if err == nil {
		t.Fatal("Expected read error")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatal("Error is not of type APIError")
	}
	if apiErr.Type != ErrorTypeNetwork {
		t.Errorf("Error type = %v, want %v", apiErr.Type, ErrorTypeNetwork)
	}
}
