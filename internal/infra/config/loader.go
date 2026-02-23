package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

const schema = "http" // Default URL scheme for shortened URLs

// LoadConfig loads and merges configuration from multiple sources with the following precedence:
// 1. Command-line flags (highest priority)
// 2. Environment variables
// 3. JSON configuration file
// 4. Default values (lowest priority)
//
// Parameters:
//   - configPath: Path to JSON configuration file (optional, can be empty string)
//
// Returns:
//   - *Config: Fully resolved configuration with defaults applied for missing values
//   - error: Returns error only if JSON file exists but cannot be parsed
//
// Configuration sources and their environment variable mappings:
//   - ServerAddr: SERVER_ADDRESS (flag: -a)
//   - BaseURL: BASE_URL (flag: -b)
//   - DatabaseDSN: DATABASE_DSN (flag: -d)
//   - FileStoragePath: FILE_STORAGE_PATH (flag: -f)
//   - AuditFile: AUDIT_FILE (flag: -audit-file)
//   - AuditURL: AUDIT_URL (flag: -audit-url)
//   - TrustedSubnet: TRUSTED_SUBNET (flag: -t)
//   - EnableTLS: HTTPS (flag: -s) or IsHTTPSSEnabled() function result
//
// Default values applied when no other source provides a value:
//   - ServerAddr: "localhost:8080"
//   - BaseURL: "http://" + ServerAddr
//   - DatabaseDSN: "user=postgres password=postgres host=localhost port=5432 database=pgx_test sslmode=disable"
//   - FileStoragePath: "data.json"
//
// Note: The function expects a JSON configuration file, not YAML as previously
// documented. Command-line flags always override environment variables and
// file configuration, while environment variables override file configuration.
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
		flagDataBaseDSN, flagAuditFile, flagAuditURL, flagTrustedSubnet string
	var flagTLS bool
	flag.StringVar(&flagServerAddr, "a", "", "Port for server startup")
	flag.StringVar(&flagBaseURL, "b", "", "Base address for shortened URLs")
	flag.StringVar(&flagDataBaseDSN, "d", "", "PostgreSQL connection string (user=postgres password=secret host=localhost port=5432 database=pgx_test sslmode=disable)")
	flag.StringVar(&flagFileStoragePath, "f", "", "Path to file storage")
	flag.StringVar(&flagAuditFile, "audit-file", "", "Path to audit log file")
	flag.StringVar(&flagAuditURL, "audit-url", "", "Full URL of remote audit log receiver")
	flag.StringVar(&flagTrustedSubnet, "t", "", "Trusted subnet")
	flag.BoolVar(&flagTLS, "s", false, "Enable TLS on HTTP server")

	flag.Parse()

	// Apply command-line flags (highest priority)
	if flagTrustedSubnet != "" {
		config.TrustedSubnet = flagTrustedSubnet
	}
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
	if env := os.Getenv("TRUSTED_SUBNET"); env != "" {
		config.TrustedSubnet = env
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
