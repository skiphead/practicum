// Examples of using the HTTP client for audit API.
package httpclient_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/skiphead/practicum/internal/pkg/transport/httpclient"
)

// Example of creating a client with default configuration
func ExampleNew() {
	// Create default configuration
	config := httpclient.DefaultConfig()

	// Create client
	client, err := httpclient.New(config)
	if err != nil {
		log.Fatal(err)
	}

	// Use client for demonstration
	_ = client // Real code would use the client here

	fmt.Printf("Client created with BaseURL: %s\n", config.BaseURL)
	// Output: Client created with BaseURL: http://localhost:8081
}

// Example of sending an audit event
func ExampleHTTPClient_SendRequest() {
	config := httpclient.DefaultConfig()
	client, err := httpclient.New(config)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	auditEvent := map[string]interface{}{
		"action":    "user_login",
		"user_id":   12345,
		"timestamp": time.Now().UTC(),
		"ip":        "192.168.1.1",
	}

	// Send POST request to create an audit event
	err = client.SendRequest(ctx, "POST", "/api/v1/audit/events", auditEvent)
	if err != nil {
		// Handle error
		var apiErr *httpclient.APIError
		if errors.As(err, &apiErr) {
			switch apiErr.Type {
			case httpclient.ErrorTypeRateLimit:
				fmt.Printf("Rate limited. Retry after: %s\n", apiErr.RetryAfter)
			case httpclient.ErrorTypeServer:
				fmt.Printf("Server error: %s\n", apiErr.Message)
			default:
				fmt.Printf("Error: %v\n", err)
			}
		}
		return
	}

	fmt.Println("Audit event sent successfully")
}

// Example of using retries
func ExampleHTTPClient_ShouldRetry() {
	config := httpclient.DefaultConfig()
	client, err := httpclient.New(config)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Create and configure retry options
	retryOpts := httpclient.RetryOptions{
		MaxRetries:  3,
		RetryDelay:  2 * time.Second,
		MaxWaitTime: 30 * time.Second,
		RetryOn:     []int{429, 500, 502, 503, 504},
	}

	// Example query data
	queryData := map[string]interface{}{
		"user_id": 12345,
		"limit":   10,
	}

	// Loop with retries
	maxAttempts := retryOpts.MaxRetries + 1
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		lastErr = client.SendRequest(ctx, "GET", "/api/v1/audit/events", queryData)
		if lastErr == nil {
			fmt.Println("Request successful")
			return
		}

		shouldRetry, delay := client.ShouldRetry(lastErr, attempt, retryOpts)
		if !shouldRetry {
			fmt.Printf("Fatal error, not retrying: %v\n", lastErr)
			return
		}

		if attempt < maxAttempts-1 { // Don't output for the last failed attempt
			fmt.Printf("Attempt %d failed, retrying in %v: %v\n", attempt+1, delay, lastErr)
			time.Sleep(delay)
		}
	}

	// If we got here, all attempts were exhausted
	fmt.Printf("All %d attempts failed. Last error: %v\n", maxAttempts, lastErr)
}

// Example of handling different error types
func ExampleAPIError() {
	// Create test errors for demonstration
	testErr := &httpclient.APIError{
		Type:       httpclient.ErrorTypeClient,
		StatusCode: http.StatusNotFound,
		Message:    "Event not found",
	}

	var apiErr *httpclient.APIError
	if errors.As(testErr, &apiErr) {
		switch apiErr.Type {
		case httpclient.ErrorTypeClient:
			if apiErr.StatusCode == http.StatusNotFound {
				fmt.Println("Audit event not found")
			} else if apiErr.StatusCode == http.StatusBadRequest {
				fmt.Printf("Bad request: %s\n", apiErr.Message)
			}
		case httpclient.ErrorTypeServer:
			fmt.Printf("Server error (status %d): %s\n", apiErr.StatusCode, apiErr.Message)
		case httpclient.ErrorTypeTimeout:
			fmt.Println("Request timed out")
		case httpclient.ErrorTypeRateLimit:
			fmt.Printf("Rate limited. Please wait %s\n", apiErr.RetryAfter)
		default:
			fmt.Printf("Unknown error: %v\n", testErr)
		}
	}
}

// Example of using with custom options
func ExampleWithHTTPClient() {
	// Create custom HTTP client
	customHTTPClient := &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			IdleConnTimeout:     90 * time.Second,
			DisableCompression:  true,
			TLSHandshakeTimeout: 15 * time.Second,
		},
	}

	config := httpclient.DefaultConfig()
	client, err := httpclient.New(config,
		httpclient.WithHTTPClient(customHTTPClient),
		httpclient.WithBaseURL("https://api.audit.example.com"),
		httpclient.WithTimeout(45*time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Use client for demonstration
	_ = client

	fmt.Printf("Client configured with custom HTTP client\n")
}

// Example of batch sending audit events
func Example_batchAuditEvents() {
	config := httpclient.DefaultConfig()
	client, err := httpclient.New(config)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	// Prepare batch audit events
	batchEvents := []map[string]interface{}{
		{
			"action":    "user_login",
			"user_id":   123,
			"timestamp": time.Now().Add(-5 * time.Minute).UTC(),
		},
		{
			"action":    "file_download",
			"user_id":   123,
			"filename":  "report.pdf",
			"timestamp": time.Now().Add(-3 * time.Minute).UTC(),
		},
		{
			"action":    "permission_change",
			"user_id":   456,
			"new_role":  "admin",
			"timestamp": time.Now().Add(-1 * time.Minute).UTC(),
		},
	}

	// Send batch
	err = client.SendRequest(ctx, "POST", "/api/v1/audit/events/batch", batchEvents)
	if err != nil {
		log.Printf("Failed to send batch: %v", err)
		return
	}

	fmt.Println("Batch audit events sent successfully")
}

// Example of creating client with minimal configuration
func ExampleNew_minimal() {
	config := httpclient.Config{
		BaseURL:         "https://api.example.com",
		Timeout:         10,
		MaxRetries:      2,
		RetryDelay:      1,
		UserAgent:       "MyApp/1.0",
		MaxResponseSize: 5 * 1024 * 1024, // 5MB
		MaxBatchSize:    500,
	}

	client, err := httpclient.New(config)
	if err != nil {
		log.Fatal(err)
	}

	_ = client // Use in real code
	fmt.Println("Client created with custom configuration")
}

// Example of using RetryOptions with functional options
func ExampleRetryOptions() {
	// Create default options
	opts := httpclient.DefaultRetryOptions

	// Apply functional options
	httpclient.WithMaxRetries(5)(&opts)
	httpclient.WithRetryDelay(3 * time.Second)(&opts)
	httpclient.WithRetryOn(408, 429, 500, 502, 503, 504)(&opts)

	fmt.Printf("Max retries: %d\n", opts.MaxRetries)
	fmt.Printf("Retry delay: %v\n", opts.RetryDelay)
	fmt.Printf("Retry on status codes: %v\n", opts.RetryOn)
	// Output:
	// Max retries: 5
	// Retry delay: 3s
	// Retry on status codes: [408 429 500 502 503 504]
}

// Example of creating RetryOptions with chain calls
func ExampleRetryOptions_chain() {
	// Create and configure options in one line
	opts := httpclient.RetryOptions{}

	// Apply all settings sequentially
	httpclient.WithMaxRetries(5)(&opts)
	httpclient.WithRetryDelay(2 * time.Second)(&opts)
	httpclient.WithMaxWaitTime(60 * time.Second)(&opts)
	httpclient.WithRetryOn(429, 500, 502, 503, 504)(&opts)

	fmt.Printf("Configured RetryOptions: %+v\n", opts)
}

// Example of comprehensive usage with proper application of RetryOptions
func Example_comprehensive() {
	// Configure client with custom parameters
	config := httpclient.DefaultConfig()
	config.BaseURL = "https://audit.production.example.com"
	config.Timeout = 15
	config.MaxRetries = 5

	// Create custom transport for better control
	transport := &http.Transport{
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     60 * time.Second,
	}

	customClient := &http.Client{
		Timeout:   time.Duration(config.Timeout) * time.Second,
		Transport: transport,
	}

	// Create audit client
	client, err := httpclient.New(config,
		httpclient.WithHTTPClient(customClient),
		httpclient.WithBaseURL(config.BaseURL),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Configure retry options using functional options
	retryOpts := httpclient.DefaultRetryOptions
	httpclient.WithMaxRetries(config.MaxRetries)(&retryOpts)
	httpclient.WithRetryDelay(time.Duration(config.RetryDelay) * time.Second)(&retryOpts)
	httpclient.WithRetryOn(408, 429, 500, 502, 503, 504)(&retryOpts)

	// Example usage in a real application
	ctx := context.Background()

	// Send test event
	event := map[string]interface{}{
		"action":    "test_action",
		"user_id":   1001,
		"timestamp": time.Now().UTC(),
	}

	err = sendWithRetry(ctx, client, event, retryOpts)
	if err != nil {
		fmt.Printf("Failed to send event after retries: %v\n", err)
	} else {
		fmt.Println("Event sent successfully")
	}
}

// sendWithRetry helper function for sending with retries
func sendWithRetry(ctx context.Context, client *httpclient.HTTPClient,
	event map[string]interface{}, opts httpclient.RetryOptions) error {

	for attempt := 0; attempt <= opts.MaxRetries; attempt++ {
		err := client.SendRequest(ctx, "POST", "/api/v1/audit/events", event)
		if err == nil {
			return nil
		}

		shouldRetry, delay := client.ShouldRetry(err, attempt, opts)
		if !shouldRetry {
			return err
		}

		if attempt < opts.MaxRetries {
			select {
			case <-time.After(delay):
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return fmt.Errorf("max retries exceeded")
}

// Example demonstrating different error types
func Example_errorTypes() {
	// Create various error types for demonstration
	errs := []*httpclient.APIError{
		{
			Type:       httpclient.ErrorTypeNetwork,
			StatusCode: 0,
			Message:    "Connection refused",
		},
		{
			Type:       httpclient.ErrorTypeTimeout,
			StatusCode: 0,
			Message:    "Request timed out",
		},
		{
			Type:       httpclient.ErrorTypeClient,
			StatusCode: 400,
			Message:    "Invalid request parameters",
		},
		{
			Type:       httpclient.ErrorTypeServer,
			StatusCode: 500,
			Message:    "Internal server error",
		},
		{
			Type:       httpclient.ErrorTypeRateLimit,
			StatusCode: 429,
			Message:    "Too many requests",
			RetryAfter: "30",
		},
		{
			Type:       httpclient.ErrorTypeResponseTooLarge,
			StatusCode: 200,
			Message:    "Response exceeds maximum size",
		},
	}

	for _, err := range errs {
		fmt.Printf("%s: %s (Status: %d)\n", err.Type, err.Message, err.StatusCode)
	}
	// Output:
	// network: Connection refused (Status: 0)
	// timeout: Request timed out (Status: 0)
	// client: Invalid request parameters (Status: 400)
	// server: Internal server error (Status: 500)
	// rate_limit: Too many requests (Status: 429)
	// response_too_large: Response exceeds maximum size (Status: 200)
}

// Example of using DefaultRetryOptions
func ExampleDefaultRetryOptions() {
	// Get default settings
	opts := httpclient.DefaultRetryOptions

	fmt.Printf("Default MaxRetries: %d\n", opts.MaxRetries)
	fmt.Printf("Default RetryDelay: %v\n", opts.RetryDelay)
	fmt.Printf("Default MaxWaitTime: %v\n", opts.MaxWaitTime)
	fmt.Printf("Default RetryOn: %v\n", opts.RetryOn)
	// Output:
	// Default MaxRetries: 3
	// Default RetryDelay: 1s
	// Default MaxWaitTime: 30s
	// Default RetryOn: [429 500 502 503 504]
}
