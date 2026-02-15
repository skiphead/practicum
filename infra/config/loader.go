package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

const schema = "http" // Default URL scheme for shortened URLs

// LoadConfig loads and merges configuration from multiple sources with precedence:
// 1. Command-line flags (highest priority)
// 2. Environment variables
// 3. YAML configuration file
// 4. Default values (lowest priority)
//
// Parameters:
//   - configPath: Path to YAML configuration file (optional)
//
// Returns:
//   - *Config: Fully resolved configuration with defaults filled in
//   - error: If YAML parsing fails
//
// Configuration sources precedence (highest to lowest):
//  1. Command-line flags
//  2. Environment variables
//  3. YAML configuration file
//  4. Default values
//
// The function also ensures all required fields have sensible defaults.
func LoadConfig(configPath string) (*Config, error) {
	config := &Config{}

	// Load from JSON file if exists
	if data, err := os.ReadFile(configPath); err == nil {
		if err = json.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("YAML parsing error: %w", err)
		}
	}

	// Define command-line flags
	var flagServerAddr, flagBaseURL, flagFileStoragePath,
		flagDataBaseDSN, flagAuditFile, flagAuditURL string
	var flagTLS bool
	flag.StringVar(&flagServerAddr, "a", "", "Port for server startup")
	flag.StringVar(&flagBaseURL, "b", "", "Base address for shortened URLs")
	flag.StringVar(&flagDataBaseDSN, "d", "", "PostgreSQL connection string (user=postgres password=secret host=localhost port=5432 database=pgx_test sslmode=disable)")
	flag.StringVar(&flagFileStoragePath, "f", "", "Path to file storage")
	flag.StringVar(&flagAuditFile, "audit-file", "", "Path to audit log file")
	flag.StringVar(&flagAuditURL, "audit-url", "", "Full URL of remote audit log receiver")
	flag.BoolVar(&flagTLS, "s", false, "Enable TLS on HTTP server")

	flag.Parse()

	// Apply command-line flags (highest priority)
	if flagTLS {
		config.EnableTLS = true
	}
	if flagServerAddr != "" {
		config.ServerAddr = flagServerAddr
	}
	if flagDataBaseDSN != "" {
		config.DatabaseDSN = flagDataBaseDSN
	}
	if flagBaseURL != "" {
		config.BaseURL = flagBaseURL
	}
	if flagAuditFile != "" {
		config.AuditFile = flagAuditFile
	}
	if flagAuditURL != "" {
		config.AuditURL = flagAuditURL
	}

	// Apply environment variables (medium priority)
	if IsHTTPSSEnabled() {
		config.EnableTLS = true
	}
	if env := os.Getenv("BASE_URL"); env != "" {
		config.BaseURL = env
	}
	if env := os.Getenv("SERVER_ADDRESS"); env != "" {
		config.ServerAddr = env
	}
	if env := os.Getenv("DATABASE_DSN"); env != "" {
		config.DatabaseDSN = env
	}
	if env := os.Getenv("FILE_STORAGE_PATH"); env != "" {
		config.FileStoragePath = env
	}
	if env := os.Getenv("AUDIT_FILE"); env != "" {
		config.AuditFile = env
	}
	if env := os.Getenv("AUDIT_URL"); env != "" {
		config.AuditURL = env
	}

	// Set defaults for empty fields (lowest priority)
	if config.DatabaseDSN == "" {
		config.DatabaseDSN = "user=postgres password=postgres host=localhost port=5432 database=pgx_test sslmode=disable"
	}

	if config.ServerAddr == "" {
		config.ServerAddr = "localhost:8080"
	}

	if config.BaseURL == "" {
		config.BaseURL = fmt.Sprintf("%s://%s", schema, config.ServerAddr)
	}

	if config.FileStoragePath == "" {
		config.FileStoragePath = "data.json"
	}

	return config, nil
}
