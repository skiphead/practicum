package audit

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/skiphead/practicum/internal/pkg/transport/httpclient"
)

// Client interface defines audit service client operations.
// It provides methods for sending audit events both individually and in batches.
type Client interface {
	CreateAuditEvent(ctx context.Context, req *CreateAuditRequest) error
	CreateAuditEventWithRetry(ctx context.Context, req *CreateAuditRequest, retryOpts ...httpclient.RetryOption) error
	BatchCreateAuditEvents(ctx context.Context, events []*CreateAuditRequest) error
}

// Service represents an audit service for sending audit events to a remote server.
// It handles validation, batching, and retry logic for reliable event delivery.
type Service struct {
	httpClient httpclient.Client // HTTP client for sending requests
	config     ServiceConfig     // Service configuration
}

// ServiceConfig contains configuration parameters for the audit service.
type ServiceConfig struct {
	MaxBatchSize int // Maximum number of events to send in a single batch
}

// DefaultServiceConfig returns the default service configuration.
// Suitable for most production scenarios with reasonable batch sizes.
//
// Returns:
//   - ServiceConfig: Default configuration with MaxBatchSize: 1000
func DefaultServiceConfig() ServiceConfig {
	return ServiceConfig{
		MaxBatchSize: 1000,
	}
}

// CreateAuditRequest represents a request to create an audit event.
// It contains all necessary information for auditing user actions.
type CreateAuditRequest struct {
	TS     int    `json:"ts"`      // Unix timestamp of the event
	Action string `json:"action"`  // Action type (e.g., "shorten", "follow")
	UserID string `json:"user_id"` // User identifier
	URL    string `json:"url"`     // URL involved in the action
}

// NewService creates a new audit service instance.
//
// Parameters:
//   - httpClient: HTTP client for making requests
//   - config: Service configuration
//
// Returns:
//   - *Service: Initialized audit service
func NewService(httpClient *httpclient.HTTPClient, config ServiceConfig) *Service {
	return &Service{
		httpClient: httpClient,
		config:     config,
	}
}

// CreateAuditEvent creates a single audit event.
// Uses default retry options for reliable delivery.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - req: Audit event request
//
// Returns:
//   - error: If validation fails or request cannot be sent
func (s *Service) CreateAuditEvent(ctx context.Context, req *CreateAuditRequest) error {
	return s.CreateAuditEventWithRetry(ctx, req)
}

// CreateAuditEventWithRetry creates an audit event with customizable retry options.
// This allows fine-grained control over retry behavior for different scenarios.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - req: Audit event request
//   - retryOpts: Optional retry configuration overrides
//
// Returns:
//   - error: If validation fails or request cannot be sent after retries
func (s *Service) CreateAuditEventWithRetry(ctx context.Context, req *CreateAuditRequest, retryOpts ...httpclient.RetryOption) error {
	// Validate request
	if err := s.validateAuditRequest(req); err != nil {
		return fmt.Errorf("invalid audit request: %w", err)
	}

	// Apply retry options
	opts := httpclient.DefaultRetryOptions
	for _, opt := range retryOpts {
		opt(&opts)
	}

	// Execute with retry logic
	return s.executeWithRetry(ctx, func() error {
		return s.httpClient.SendRequest(ctx, "POST", "/api/audit/events", req)
	}, opts)
}

// BatchCreateAuditEvents creates multiple audit events in a single batch request.
// This is more efficient than sending individual requests for high-volume scenarios.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - events: Slice of audit event requests
//
// Returns:
//   - error: If validation fails, batch size exceeds limit, or request fails
//
// The method validates all events before sending and ensures the batch size
// doesn't exceed the configured maximum.
func (s *Service) BatchCreateAuditEvents(ctx context.Context, events []*CreateAuditRequest) error {
	if len(events) == 0 {
		return nil
	}

	// Check batch size
	if len(events) > s.config.MaxBatchSize {
		return fmt.Errorf("batch size %d exceeds maximum allowed size %d", len(events), s.config.MaxBatchSize)
	}

	// Validate all events
	for i, event := range events {
		if err := s.validateAuditRequest(event); err != nil {
			return fmt.Errorf("invalid audit request at index %d: %w", i, err)
		}
	}

	// Execute with retry logic
	return s.executeWithRetry(ctx, func() error {
		return s.httpClient.SendRequest(ctx, "POST", "/api/audit/events/batch", events)
	}, httpclient.DefaultRetryOptions)
}

// validateAuditRequest validates an audit request for correctness and security.
// Ensures all required fields are present and within reasonable limits.
//
// Parameters:
//   - req: Audit request to validate
//
// Returns:
//   - error: If any validation check fails
//
// Validation rules:
//  1. Request cannot be nil
//  2. Timestamp must be positive
//  3. Action must be non-empty and ≤100 characters
//  4. UserID must be non-empty and ≤100 characters
//  5. URL must be non-empty, ≤2000 characters, and valid format
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

	// Validate URL format
	if _, err := url.Parse(req.URL); err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	return nil
}

// executeWithRetry executes an operation with retry logic and exponential backoff.
// Implements robust retry mechanism with context-aware timeout handling.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - operation: Function to execute with retries
//   - opts: Retry configuration options
//
// Returns:
//   - error: Last error if all retries fail, nil on success
func (s *Service) executeWithRetry(ctx context.Context, operation func() error, opts httpclient.RetryOptions) error {
	var lastErr error

	// Create context with overall timeout for all attempts
	if opts.MaxWaitTime > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.MaxWaitTime)
		defer cancel()
	}

	attempt := 0
	for {
		// Execute operation
		err := operation()
		if err == nil {
			return nil
		}
		// Save last error
		lastErr = err

		// Check if we should retry
		shouldRetry, delay := s.httpClient.ShouldRetry(err, attempt, opts)
		if !shouldRetry {
			return err
		}

		// Increment attempt counter
		attempt++

		// Wait before next attempt
		if delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return fmt.Errorf("operation failed: %w, context cancelled: %v", lastErr, ctx.Err())
			case <-timer.C:
				// Continue loop
			}
		}
	}
}
