package postgresql

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"os"
)

// ConnectionConfig contains database connection settings.
type ConnectionConfig struct {
	Host              string        // Database host
	Port              string        // Database port
	Timeout           time.Duration // Connection timeout
	MaxRetries        int           // Maximum retry attempts
	RetryInterval     time.Duration // Initial retry interval
	BackoffMultiplier float64       // Multiplier for exponential backoff
}

// TCPChecker interface for checking TCP connectivity.
type TCPChecker interface {
	Check(host, port string, timeout time.Duration) error
}

// RealTCPChecker implements TCPChecker for actual network connections.
type RealTCPChecker struct{}

// Check verifies TCP connectivity to a host:port combination.
//
// Parameters:
//   - host: Target hostname or IP address
//   - port: Target port number
//   - timeout: Maximum time to wait for connection
//
// Returns:
//   - error: Connection error if host:port is unreachable
func (r *RealTCPChecker) Check(host, port string, timeout time.Duration) error {
	address := net.JoinHostPort(host, port)
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return fmt.Errorf("cannot reach %s: %w", address, err)
	}
	conn.Close()
	return nil
}

// RetryConfig contains configuration for retry behavior.
type RetryConfig struct {
	MaxRetries        int           // Maximum number of retry attempts
	RetryInterval     time.Duration // Initial interval between retries
	BackoffMultiplier float64       // Multiplier for exponential backoff
}

// DefaultRetryConfig returns a default retry configuration.
// Suitable for most production scenarios with gradual backoff.
//
// Returns:
//   - RetryConfig: Default configuration with:
//   - MaxRetries: 3
//   - RetryInterval: 2 seconds
//   - BackoffMultiplier: 1.5
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:        3,
		RetryInterval:     2 * time.Second,
		BackoffMultiplier: 1.5,
	}
}

// extractHostPort extracts host and port from a PostgreSQL connection string.
// Supports both URL format (postgresql://user:pass@host:port/db) and DSN format.
//
// Parameters:
//   - config: PostgreSQL connection string
//
// Returns:
//   - string: Hostname or IP address
//   - string: Port number
//   - error: Parsing error if format is invalid
func extractHostPort(config string) (string, string, error) {
	if strings.Contains(config, "://") {
		start := strings.Index(config, "@")
		if start == -1 {
			return "", "", fmt.Errorf("invalid connection URL format")
		}
		end := strings.Index(config[start+1:], "/")
		if end == -1 {
			return "", "", fmt.Errorf("invalid connection URL format")
		}

		hostPort := config[start+1 : start+1+end]
		parts := strings.Split(hostPort, ":")
		if len(parts) != 2 {
			return "", "", fmt.Errorf("invalid host:port format")
		}
		return parts[0], parts[1], nil
	}

	var host, port string
	pairs := strings.Fields(config)
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "host":
			host = kv[1]
		case "port":
			port = kv[1]
		}
	}

	if host == "" {
		host = "localhost"
	}
	if port == "" {
		port = "5432"
	}

	return host, port, nil
}

// retryOperation executes an operation with retry logic and exponential backoff.
//
// Parameters:
//   - operation: Function to execute with retries
//   - config: Retry configuration parameters
//
// Returns:
//   - error: Last error if all retries fail, nil on success
func retryOperation(operation func() error, config RetryConfig) error {
	var lastErr error
	currentInterval := config.RetryInterval

	for i := 0; i <= config.MaxRetries; i++ {
		lastErr = operation()
		if lastErr == nil {
			return nil // Success
		}

		// If this is the last attempt, don't wait
		if i == config.MaxRetries {
			break
		}

		fmt.Printf("Connection attempt %d/%d failed: %v. Retrying in %v...\n",
			i+1, config.MaxRetries, lastErr, currentInterval)

		time.Sleep(currentInterval)

		// Increase interval for next attempt (exponential backoff)
		currentInterval = time.Duration(float64(currentInterval) * config.BackoffMultiplier)
	}

	return fmt.Errorf("after %d attempts: %w", config.MaxRetries+1, lastErr)
}

// SafeConn creates a connection pool to PostgreSQL with availability checks and retries.
// Uses default timeout and retry configuration for most scenarios.
//
// Parameters:
//   - config: PostgreSQL connection string
//
// Returns:
//   - *pgxpool.Pool: Connection pool or nil if connection fails
//
// Note: If connection fails, the function prints to stderr but doesn't exit.
// Callers should handle nil return values appropriately.
func SafeConn(config string) *pgxpool.Pool {
	return ConnWithRetry(config, &RealTCPChecker{}, 5*time.Second, DefaultRetryConfig())
}

// ConnWithRetry creates a connection pool with custom settings and retry logic.
// Performs TCP connectivity check before attempting database connection.
//
// Parameters:
//   - config: PostgreSQL connection string
//   - checker: TCPChecker implementation for connectivity verification
//   - timeout: Connection timeout duration
//   - retryConfig: Retry behavior configuration
//
// Returns:
//   - *pgxpool.Pool: Connection pool or nil if connection fails
//
// The function performs these steps:
// 1. Parses host and port from connection string
// 2. Validates TCP connectivity
// 3. Establishes database connection with retries
// 4. Performs connection ping to verify functionality
func ConnWithRetry(config string, checker TCPChecker, timeout time.Duration, retryConfig RetryConfig) *pgxpool.Pool {
	host, port, err := extractHostPort(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to parse connection string: %v\n", err)
		// os.Exit(1) // Commented out to allow caller to handle error
	}

	var pool *pgxpool.Pool

	// Attempt connection with retries
	err = retryOperation(func() error {
		// Check TCP availability
		if tcpErr := checker.Check(host, port, timeout); tcpErr != nil {
			return fmt.Errorf("TCP check failed: %w", tcpErr)
		}

		// Connect to database
		var connErr error
		pool, connErr = pgxpool.New(context.Background(), config)
		if connErr != nil {
			return fmt.Errorf("database connection failed: %w", connErr)
		}

		// Verify connection with ping
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		if pingErr := pool.Ping(ctx); pingErr != nil {
			pool.Close()
			return fmt.Errorf("database ping failed: %w", pingErr)
		}

		return nil
	}, retryConfig)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to establish database connection: %v\n", err)
		// os.Exit(1) // Commented out to allow caller to handle error
	}

	return pool
}

// ConnWithConfig creates a connection pool with custom settings (backward compatibility).
// Uses default retry configuration.
//
// Parameters:
//   - config: PostgreSQL connection string
//   - checker: TCPChecker implementation
//   - timeout: Connection timeout
//
// Returns:
//   - *pgxpool.Pool: Connection pool or nil if connection fails
func ConnWithConfig(config string, checker TCPChecker, timeout time.Duration) *pgxpool.Pool {
	return ConnWithRetry(config, checker, timeout, DefaultRetryConfig())
}
