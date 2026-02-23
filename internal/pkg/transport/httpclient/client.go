// Package httpclient provides an HTTP client for working with audit APIs.
// The client supports retries, error handling, response size limiting,
// and other features for reliable operation with external APIs.
package httpclient

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

// Client interface for the HTTP client.
// Defines the main methods for sending requests and managing retries.
type Client interface {
	// SendRequest sends an HTTP request with the given method, path, and body.
	// Returns an error on failure. Handles various error types,
	// including network errors, timeouts, client and server errors.
	SendRequest(ctx context.Context, method, path string, body interface{}) error

	// ShouldRetry determines whether a request should be retried when an error occurs.
	// Returns a boolean value and the delay until the next attempt.
	ShouldRetry(err error, attempt int, opts RetryOptions) (bool, time.Duration)
}

// HTTPClient represents an HTTP client for audit APIs.
// Implements the Client interface with support for configuration, retries,
// and error handling.
type HTTPClient struct {
	config     Config
	httpClient *http.Client
}

// Check that HTTPClient implements the Client interface.
var _ Client = (*HTTPClient)(nil)

// Config contains configuration parameters for the HTTP client.
type Config struct {
	BaseURL         string // Base URL of the API
	Timeout         int    // Request timeout in seconds
	MaxRetries      int    // Maximum number of retry attempts
	RetryDelay      int    // Base delay between attempts in seconds
	UserAgent       string // User-Agent header
	MaxResponseSize int    // Maximum response size in bytes
	MaxBatchSize    int    // Maximum batch size for batch operations
}

// DefaultConfig returns the default configuration.
// Used as a starting point for client setup.
func DefaultConfig() Config {
	return Config{
		BaseURL:         "http://localhost:8081",
		Timeout:         30,
		MaxRetries:      3,
		RetryDelay:      1,
		UserAgent:       "AuditClient/1.0",
		MaxResponseSize: 10 * 1024 * 1024, // 10 MB
		MaxBatchSize:    1000,
	}
}

// Validate checks the configuration for correctness.
// Returns an error if any parameters are invalid.
func (c Config) Validate() error {
	if c.BaseURL == "" {
		return fmt.Errorf("BaseURL cannot be empty")
	}

	if _, err := url.Parse(c.BaseURL); err != nil {
		return fmt.Errorf("invalid BaseURL: %w", err)
	}

	if c.Timeout <= 0 {
		return fmt.Errorf("validete error timeout must be positive")
	}

	if c.MaxRetries < 0 {
		return fmt.Errorf("validete error MaxRetries cannot be negative")
	}

	if c.RetryDelay < 0 {
		return fmt.Errorf("validete error RetryAfter cannot be negative")
	}

	if c.UserAgent == "" {
		return fmt.Errorf("validete error UserAgent cannot be empty")
	}

	if c.MaxResponseSize <= 0 {
		return fmt.Errorf("validete error MaxResponseSize must be positive")
	}

	if c.MaxResponseSize > 100*1024*1024 { // 100 MB
		return fmt.Errorf("validete error MaxResponseSize is too large")
	}

	if c.MaxBatchSize <= 0 {
		return fmt.Errorf("validete error MaxBatchSize must be positive")
	}

	return nil
}

// Option function for configuring the client using functional options.
type Option func(*HTTPClient) error

// New creates a new instance of the HTTP client with the given configuration.
// Accepts optional parameters for additional configuration.
// Returns an error if the configuration is invalid.
//
// Example of creating a client:
//
//	config := httpclient.DefaultConfig()
//	client, err := httpclient.New(config)
//	if err != nil {
//	    log.Fatal(err)
//	}
func New(config Config, opts ...Option) (*HTTPClient, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	client := &HTTPClient{
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

// WithHTTPClient sets a custom HTTP client.
// Useful for injecting mocks in tests or using clients with special settings.
//
// Example:
//
//	customClient := &http.Client{
//	    Timeout: 60 * time.Second,
//	}
//	client, err := httpclient.New(config, httpclient.WithHTTPClient(customClient))
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *HTTPClient) error {
		if httpClient == nil {
			return fmt.Errorf("httpClient cannot be nil")
		}
		c.httpClient = httpClient
		return nil
	}
}

// WithBaseURL sets the base URL for the API.
// Overrides the URL from the configuration.
//
// Example:
//
//	client, err := httpclient.New(config, httpclient.WithBaseURL("https://api.example.com"))
func WithBaseURL(baseURL string) Option {
	return func(c *HTTPClient) error {
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

// WithTimeout sets the timeout for HTTP requests.
// Overrides the timeout from the configuration.
//
// Example:
//
//	client, err := httpclient.New(config, httpclient.WithTimeout(60*time.Second))
func WithTimeout(timeout time.Duration) Option {
	return func(c *HTTPClient) error {
		if timeout <= 0 {
			return fmt.Errorf("timeout must be positive")
		}
		c.httpClient.Timeout = timeout
		return nil
	}
}

// SendRequest sends a request to the audit API.
// Supports all main HTTP methods (GET, POST, PUT, DELETE, etc.).
// Automatically serializes the request body to JSON and handles the response.
//
// Example of sending a POST request:
//
//	ctx := context.Background()
//	auditEvent := map[string]interface{}{
//	    "action":    "user_login",
//	    "user_id":   123,
//	    "timestamp": time.Now(),
//	}
//
//	err := client.SendRequest(ctx, "POST", "/api/v1/audit", auditEvent)
//	if err != nil {
//	    // Handle error
//	    var apiErr *httpclient.APIError
//	    if errors.As(err, &apiErr) {
//	        switch apiErr.Type {
//	        case httpclient.ErrorTypeRateLimit:
//	            log.Printf("Rate limit exceeded, retry after: %s", apiErr.RetryAfter)
//	        case httpclient.ErrorTypeServer:
//	            log.Printf("Server error: %s", apiErr.Message)
//	        }
//	    }
//	}
//
// Example of sending a GET request:
//
//	err := client.SendRequest(ctx, "GET", "/api/v1/audit/123", nil)
func (c *HTTPClient) SendRequest(ctx context.Context, method, path string, body interface{}) error {
	// Serialize the request body
	var bodyBytes []byte
	var err error
	if body != nil {
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return &APIError{
				Type:        ErrorTypeClient,
				Message:     "failed to marshal request body",
				OriginalErr: err,
			}
		}
	}

	// Form the request URL
	urlStr := fmt.Sprintf("%s%s", c.config.BaseURL, path)

	// Create the HTTP request
	var bodyReader io.Reader
	if bodyBytes != nil {
		bodyReader = bytes.NewReader(bodyBytes)
	}
	req, err := http.NewRequestWithContext(ctx, method, urlStr, bodyReader)
	if err != nil {
		return &APIError{
			Type:        ErrorTypeNetwork,
			Message:     "failed to create request",
			OriginalErr: err,
		}
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.config.UserAgent)

	// Execute the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Check if the error was due to context timeout
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

	// Read the response body with size limitation
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

	// Check if the response was truncated
	if len(responseBody) == c.config.MaxResponseSize {
		// Try to read one more byte to confirm the response was truncated
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

	// Handle the status code
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

// APIError represents an API error.
// Contains the error type, status code, message, and original error (if any).
type APIError struct {
	Type        ErrorType
	StatusCode  int
	Message     string
	OriginalErr error
	RetryAfter  string
}

// Error returns the string representation of the error.
func (e *APIError) Error() string {
	if e.OriginalErr != nil {
		return fmt.Sprintf("%s (status: %d): %s: %v", e.Type, e.StatusCode, e.Message, e.OriginalErr)
	}
	return fmt.Sprintf("%s (status: %d): %s", e.Type, e.StatusCode, e.Message)
}

// Unwrap returns the original error.
// Allows using errors.As and errors.Is to check the error type.
func (e *APIError) Unwrap() error {
	return e.OriginalErr
}

// ErrorType type of error.
type ErrorType string

const (
	ErrorTypeNetwork          ErrorType = "network"
	ErrorTypeTimeout          ErrorType = "timeout"
	ErrorTypeClient           ErrorType = "client"
	ErrorTypeServer           ErrorType = "server"
	ErrorTypeRateLimit        ErrorType = "rate_limit"
	ErrorTypeResponseTooLarge ErrorType = "response_too_large"
	ErrorTypeUnknown          ErrorType = "unknown"
)

// RetryOption function for configuring retry parameters.
type RetryOption func(*RetryOptions)

// RetryOptions contains parameters for retries.
type RetryOptions struct {
	MaxRetries  int           // Maximum number of attempts
	RetryDelay  time.Duration // Base delay between attempts
	MaxWaitTime time.Duration // Maximum total wait time
	RetryOn     []int         // Status codes for retries
}

// DefaultRetryOptions default settings for retries.
var DefaultRetryOptions = RetryOptions{
	MaxRetries:  3,
	RetryDelay:  1 * time.Second,
	MaxWaitTime: 30 * time.Second,
	RetryOn:     []int{429, 500, 502, 503, 504},
}

// WithMaxRetries sets the maximum number of retry attempts.
//
// Example usage:
//
//	opts := httpclient.DefaultRetryOptions
//	opts = httpclient.WithMaxRetries(5)(opts)
func WithMaxRetries(maxRetries int) RetryOption {
	return func(opts *RetryOptions) {
		if maxRetries < 0 {
			maxRetries = 0
		}
		opts.MaxRetries = maxRetries
	}
}

// WithRetryDelay sets the delay between attempts.
//
// Example:
//
//	opts := httpclient.DefaultRetryOptions
//	opts = httpclient.WithRetryDelay(2*time.Second)(opts)
func WithRetryDelay(delay time.Duration) RetryOption {
	return func(opts *RetryOptions) {
		if delay < 0 {
			delay = 0
		}
		opts.RetryDelay = delay
	}
}

// WithMaxWaitTime sets the maximum waiting time for all attempts.
//
// Example:
//
//	opts := httpclient.DefaultRetryOptions
//	opts = httpclient.WithMaxWaitTime(60*time.Second)(opts)
func WithMaxWaitTime(maxWaitTime time.Duration) RetryOption {
	return func(opts *RetryOptions) {
		if maxWaitTime <= 0 {
			maxWaitTime = 0
		}
		opts.MaxWaitTime = maxWaitTime
	}
}

// WithRetryOn sets status codes for retries.
//
// Example:
//
//	opts := httpclient.DefaultRetryOptions
//	opts = httpclient.WithRetryOn(408, 429, 500, 502, 503, 504)(opts)
func WithRetryOn(statusCodes ...int) RetryOption {
	return func(opts *RetryOptions) {
		opts.RetryOn = statusCodes
	}
}

// ShouldRetry determines whether to retry on error.
// Returns true and the delay until the next attempt if retry is needed.
// Uses exponential delay with jitter to prevent stampeding herd.
//
// Example usage in a retry loop:
//
//	for attempt := 0; attempt <= maxRetries; attempt++ {
//	    err := client.SendRequest(ctx, method, path, body)
//	    if err == nil {
//	        break // Success
//	    }
//
//	    shouldRetry, delay := client.ShouldRetry(err, attempt, retryOpts)
//	    if !shouldRetry {
//	        return err // Cannot retry
//	    }
//
//	    time.Sleep(delay) // Wait before the next attempt
//	}
func (c *HTTPClient) ShouldRetry(err error, attempt int, opts RetryOptions) (bool, time.Duration) {
	// If maximum attempts reached, do not retry
	if attempt >= opts.MaxRetries {
		return false, 0
	}

	var apiErr *APIError
	ok := errors.As(err, &apiErr)
	if !ok {
		// If this is not an APIError, do not retry
		return false, 0
	}

	// Do not retry for timeout errors
	if apiErr.Type == ErrorTypeTimeout {
		return false, 0
	}

	// Do not retry for client errors (except 429)
	if apiErr.Type == ErrorTypeClient && apiErr.StatusCode != 429 {
		return false, 0
	}

	// Check if we should retry for this status code
	shouldRetryStatusCode := false
	for _, statusCode := range opts.RetryOn {
		if apiErr.StatusCode == statusCode {
			shouldRetryStatusCode = true
			break
		}
	}

	if !shouldRetryStatusCode {
		return false, 0
	}

	// Calculate base delay
	delay := opts.RetryDelay

	// For rate limit, use the Retry-After header
	if apiErr.Type == ErrorTypeRateLimit && apiErr.RetryAfter != "" {
		if retryDelay, parseErr := parseRetryAfter(apiErr.RetryAfter); parseErr == nil {
			delay = retryDelay
		}
	}

	// Exponential delay
	delay = delay * (1 << uint(attempt)) // Multiply by 2^attempt

	// Add random jitter (±20%)
	jitterPercent := 0.4 // ±20%
	jitterMultiplier := 1 + (rand.Float64()*jitterPercent - jitterPercent/2)
	delay = time.Duration(float64(delay) * jitterMultiplier)

	// Limit delay from below (minimum 10 ms)
	if delay < 10*time.Millisecond {
		delay = 10 * time.Millisecond
	}

	// Limit delay from above
	if delay > 30*time.Second {
		delay = 30 * time.Second
	}

	return true, delay
}

// parseRetryAfter parses the Retry-After header.
// Supports parsing both as seconds (integer) and dates in RFC1123, RFC1123Z and RFC3339 formats.
func parseRetryAfter(retryAfter string) (time.Duration, error) {
	if retryAfter == "" {
		return 0, fmt.Errorf("empty retry-after")
	}

	// Try to parse as seconds (integer)
	if seconds, err := strconv.Atoi(retryAfter); err == nil {
		if seconds < 0 {
			return 0, fmt.Errorf("retry-after seconds cannot be negative")
		}
		if seconds > 86400 { // 24 hours
			return 0, fmt.Errorf("retry-after seconds too large: %d", seconds)
		}
		return time.Duration(seconds) * time.Second, nil
	}

	// Try to parse as RFC1123 date
	if date, err := time.Parse(time.RFC1123, retryAfter); err == nil {
		return calculateRetryDuration(date)
	}

	// Try to parse as RFC1123Z (with timezone)
	if date, err := time.Parse(time.RFC1123Z, retryAfter); err == nil {
		return calculateRetryDuration(date)
	}

	// Try to parse as RFC3339
	if date, err := time.Parse(time.RFC3339, retryAfter); err == nil {
		return calculateRetryDuration(date)
	}

	return 0, fmt.Errorf("invalid retry-after format: %s", retryAfter)
}

// calculateRetryDuration calculates the duration until the specified date.
// Returns an error if the date is in the past or too far in the future.
func calculateRetryDuration(date time.Time) (time.Duration, error) {
	now := time.Now()
	if date.After(now) {
		duration := date.Sub(now)
		// Limit maximum delay (e.g., 24 hours)
		if duration > 24*time.Hour {
			return 0, fmt.Errorf("retry-after date too far in the future: %v", date)
		}
		return duration, nil
	}
	return 0, nil
}
