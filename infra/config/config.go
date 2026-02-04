// Package config provides configuration management for the URL shortener service.
// It handles loading, validating, and providing access to application settings.
package config

import (
	"fmt"
	"net"
)

// Config holds the application configuration loaded from YAML or environment variables.
// It defines all configurable parameters for the URL shortener service.
type Config struct {
	ServerAddr      string `yaml:"server_addr"`       // HTTP server address (host:port)
	BaseURL         string `yaml:"base_url"`          // Base URL for shortened links (e.g., http://localhost:8080)
	FileStoragePath string `yaml:"file_storage_path"` // Path to file storage for URL persistence
	DatabaseDSN     string `yaml:"database_dsn"`      // PostgreSQL database connection string
	SessionKey      string `yaml:"session_key"`       // Secret key for JWT session encryption
	AuditFile       string `yaml:"audit_file"`        // Path to audit log file
	AuditURL        string `yaml:"audit_url"`         // URL for remote audit logging endpoint
}

// NewDefaultConfig creates a new Config instance with default values.
// This provides sensible defaults for development and testing environments.
//
// Returns:
//   - *Config: Configuration instance with default server address
//
// Default values:
//   - ServerAddr: "localhost:8080" (listens on all interfaces, port 8080)
func NewDefaultConfig() *Config {
	return &Config{
		ServerAddr: "localhost:8080",
	}
}

// Validate performs validation checks on the configuration.
// It ensures required fields are present and have valid formats.
//
// Returns:
//   - error: Validation error if any field is invalid, nil otherwise
//
// Validation checks:
//  1. ServerAddr must be a valid host:port format
//  2. Host component must not be empty
//  3. Port component must not be empty
//
// Note: This validation only checks format, not whether the host is resolvable
// or the port is available. DNS resolution and port availability are checked
// at runtime during server startup.
func (c *Config) Validate() error {
	// Check the format without resolving the hostname
	host, port, err := net.SplitHostPort(c.ServerAddr)
	if err != nil {
		return fmt.Errorf("error parsing server address: %w", err)
	}
	if host == "" {
		return fmt.Errorf("missing host in address %q", c.ServerAddr)
	}
	if port == "" {
		return fmt.Errorf("missing port in address %q", c.ServerAddr)
	}
	return nil
}
