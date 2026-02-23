// Package audit provides an adapter for sending audit events to multiple destinations.
// It supports file-based logging and HTTP-based remote logging with batching,
// asynchronous processing, and graceful error handling for reliable audit trail.
package audit

import (
	"context"
	"fmt"
	"sync"
	"time"

	audit2 "github.com/skiphead/practicum/internal/infra/audit"
	"github.com/skiphead/practicum/internal/pkg/transport/httpclient"
	"go.uber.org/zap"
)

// Config represents the configuration for the audit adapter.
// It defines where audit events should be sent and how they should be processed.
type Config struct {
	FilePath     string // Path to file for audit logging (empty for no file logging)
	HTTPEndpoint string // URL for sending audit events via HTTP (empty for no HTTP logging)
	Enabled      bool   // Whether audit logging is enabled
	MaxBatchSize int    // Maximum batch size for HTTP events
	QueueSize    int    // Size of the event processing queue
}

// DefaultConfig returns a default configuration for the audit adapter.
// This provides sensible defaults for most use cases.
//
// Returns:
//   - Config: Default configuration with:
//   - FilePath: "/var/log/audit.log"
//   - HTTPEndpoint: "" (disabled by default)
//   - Enabled: true
//   - MaxBatchSize: 1000
//   - QueueSize: 10000
func DefaultConfig() Config {
	return Config{
		FilePath:     "/var/log/audit.log",
		HTTPEndpoint: "",
		Enabled:      true,
		MaxBatchSize: 1000,
		QueueSize:    10000,
	}
}

// Validate checks the configuration for correctness.
// It ensures that numerical parameters are positive and within reasonable bounds.
//
// Returns:
//   - error: Validation error if configuration is invalid, nil otherwise
//
// Validation rules:
//   - MaxBatchSize must be positive (>0)
//   - QueueSize must be positive (>0)
//   - Both FilePath and HTTPEndpoint can be empty (adapter will run in no-op mode)
func (c Config) Validate() error {
	// Remove check for at least one receiver
	// If both are empty, adapter will work in "no-op" mode

	if c.MaxBatchSize <= 0 {
		return fmt.Errorf("MaxBatchSize must be positive")
	}

	if c.QueueSize <= 0 {
		return fmt.Errorf("QueueSize must be positive")
	}

	return nil
}

// Event represents an audit event in the adapter layer.
// It contains all necessary information for audit trail recording.
type Event struct {
	Timestamp int64  `json:"ts"`      // Unix timestamp of the event
	Action    string `json:"action"`  // Action type (e.g., "shorten", "follow")
	UserID    string `json:"user_id"` // User identifier
	URL       string `json:"url"`     // URL involved in the action
}

// Adapter adapts audit events for delivery to different receivers (file, HTTP).
// It provides asynchronous event processing with batching for HTTP transport.
type Adapter struct {
	fileLogger *audit2.Logger // File logger instance
	httpClient audit2.Client  // HTTP client for remote audit logging
	config     Config         // Adapter configuration
	enabled    bool           // Whether audit is currently enabled
	mutex      sync.RWMutex   // Mutex for thread-safe state changes
	queue      chan *Event    // Channel for queuing audit events
	wg         sync.WaitGroup // WaitGroup for graceful shutdown
	done       chan struct{}  // Channel for signaling shutdown
}

// NewAdapter creates a new audit adapter with the given configuration.
// It initializes file and/or HTTP logging based on the configuration.
//
// Parameters:
//   - cfg: Audit adapter configuration
//
// Returns:
//   - *Adapter: Initialized audit adapter
//   - error: If configuration is invalid or initialization fails
//
// The adapter starts in enabled state only if both:
// 1. Enabled is true in config
// 2. At least one receiver (file or HTTP) is configured
func NewAdapter(cfg Config) (*Adapter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	adapter := &Adapter{
		config:  cfg,
		enabled: cfg.Enabled,
		queue:   make(chan *Event, cfg.QueueSize),
		done:    make(chan struct{}),
	}

	var err error

	// Initialize file logger only if file path is specified
	if cfg.FilePath != "" {
		adapter.fileLogger, err = audit2.GetInstance(cfg.FilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to create file logger: %w", err)
		}
	}

	// Initialize HTTP client only if endpoint is specified
	if cfg.HTTPEndpoint != "" {
		httpCfg := httpclient.DefaultConfig()
		httpCfg.BaseURL = cfg.HTTPEndpoint
		httpCfg.MaxRetries = 3
		httpCfg.Timeout = 5

		httpClient, err := httpclient.New(httpCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTP client: %w", err)
		}

		adapter.httpClient = audit2.NewService(httpClient, audit2.ServiceConfig{
			MaxBatchSize: cfg.MaxBatchSize,
		})
	}

	// Start queue processor if audit is enabled
	// and at least one receiver is configured
	if cfg.Enabled && (cfg.FilePath != "" || cfg.HTTPEndpoint != "") {
		adapter.startProcessor()
	} else if cfg.Enabled {
		// If enabled but no receivers configured, log this
		// but don't create queue and processor
		adapter.enabled = false
	}

	return adapter, nil
}

// LogEvent adds an audit event to the processing queue.
// The method is non-blocking and returns immediately.
//
// Parameters:
//   - ctx: Context for cancellation/timeout
//   - event: Audit event to log
//
// Returns:
//   - error: If context is cancelled or queue is full
//
// Events are processed asynchronously. If the queue is full,
// new events are dropped to prevent memory exhaustion.
func (a *Adapter) LogEvent(ctx context.Context, event *Event) error {
	// Check context first
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	a.mutex.RLock()
	if !a.enabled {
		a.mutex.RUnlock()
		return nil
	}
	a.mutex.RUnlock()

	select {
	case a.queue <- event:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Queue is full, drop the event
		return fmt.Errorf("audit queue is full, event dropped")
	}
}

// startProcessor starts the event queue processor in a goroutine.
// The processor handles batching for HTTP events and direct file writing.
func (a *Adapter) startProcessor() {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()

		// If no HTTP client, process without batching
		if a.httpClient == nil {
			a.processWithoutBatching()
			return
		}

		// Process with batching for HTTP
		a.processWithBatching()
	}()
}

// processWithoutBatching processes events without HTTP batching.
// Used when only file logging is configured.
func (a *Adapter) processWithoutBatching() {
	for {
		select {
		case <-a.done:
			return

		case event := <-a.queue:
			// Send only to file
			if err := a.logToFile(event); err != nil {
				// Log error but continue
				zap.L().Error("failed to log audit event", zap.Error(err))
			}
		}
	}
}

// processWithBatching processes events with batching for HTTP transport.
// Implements time-based and size-based batching for efficient HTTP transmission.
func (a *Adapter) processWithBatching() {
	batch := make([]*audit2.CreateAuditRequest, 0, a.config.MaxBatchSize)
	batchTimer := time.NewTimer(1 * time.Second)

	defer batchTimer.Stop()

	for {
		select {
		case <-a.done:
			// Send remaining events before shutdown
			a.flushBatch(batch)
			return

		case event := <-a.queue:
			// Send to file (synchronously)
			if err := a.logToFile(event); err != nil {
				// Log error but continue
				zap.L().Error("Failed to write to file: %v", zap.Error(err))
			}

			// Add to batch for HTTP
			if a.httpClient != nil {
				req := a.convertToAuditRequest(event)
				batch = append(batch, req)

				// Send batch if it reaches maximum size
				if len(batch) >= a.config.MaxBatchSize {
					a.flushBatchHTTP(batch)
					batch = batch[:0]

					// Stop and reset timer
					if !batchTimer.Stop() {
						<-batchTimer.C
					}
					batchTimer.Reset(1 * time.Second)
				}
			}

		case <-batchTimer.C:
			// Send accumulated events on timer
			if len(batch) > 0 {
				a.flushBatchHTTP(batch)
				batch = batch[:0]
			}
			batchTimer.Reset(1 * time.Second)
		}
	}
}

// flushBatch sends a batch of audit events.
// This is a wrapper method for different batch types.
func (a *Adapter) flushBatch(batch []*audit2.CreateAuditRequest) {
	if len(batch) == 0 {
		return
	}

	// Send via HTTP
	a.flushBatchHTTP(batch)
}

// flushBatchHTTP sends a batch of audit events via HTTP.
// Implements retry logic and error handling.
func (a *Adapter) flushBatchHTTP(batch []*audit2.CreateAuditRequest) {
	if a.httpClient == nil || len(batch) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := a.httpClient.BatchCreateAuditEvents(ctx, batch); err != nil {
		// Log error but don't interrupt execution
		// In production, implement dead letter queue
		zap.L().Error("failed to batch audit event", zap.Error(err))
	}
}

// logToFile writes an audit event to the file.
// Handles different event types with appropriate logging methods.
func (a *Adapter) logToFile(event *Event) error {
	if a.fileLogger == nil {
		return nil
	}

	switch event.Action {
	case "shorten":
		return a.fileLogger.LogShorten(event.UserID, event.URL)
	case "follow":
		return a.fileLogger.LogFollow(event.UserID, event.URL)
	default:
		return a.fileLogger.Log(audit2.LogAction(event.Action), event.UserID, event.URL)
	}
}

// convertToAuditRequest converts an adapter Event to an audit service request.
func (a *Adapter) convertToAuditRequest(event *Event) *audit2.CreateAuditRequest {
	return &audit2.CreateAuditRequest{
		TS:     int(event.Timestamp),
		Action: event.Action,
		UserID: event.UserID,
		URL:    event.URL,
	}
}

// Enable enables audit logging.
// Starts the processor if it's not already running and receivers are configured.
func (a *Adapter) Enable() {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if !a.enabled {
		a.enabled = true
		// Start processor only if receivers are configured
		if a.config.FilePath != "" || a.config.HTTPEndpoint != "" {
			a.startProcessor()
		}
	}
}

// Disable disables audit logging.
// Stops the processor and waits for pending events to be processed.
func (a *Adapter) Disable() {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if a.enabled {
		a.enabled = false
		close(a.done)
		a.wg.Wait()
		a.done = make(chan struct{})
	}
}

// Close gracefully shuts down the audit adapter.
// Disables logging and releases all resources.
//
// Returns:
//   - error: If file logger fails to close
func (a *Adapter) Close() error {
	a.Disable()

	if a.fileLogger != nil {
		return a.fileLogger.Close()
	}

	return nil
}
