package config

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// resetFlags completely resets flag state between tests
func resetFlags() {
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
}

func TestLoadConfig(t *testing.T) {
	// Save original arguments
	oldArgs := os.Args
	defer func() {
		os.Args = oldArgs
	}()

	t.Run("Defaults", func(t *testing.T) {
		resetFlags()
		os.Args = []string{"test"}

		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, "nonexistent.yaml")

		config, err := LoadConfig(configPath)
		if err != nil {
			t.Fatalf("LoadConfig failed: %v", err)
		}

		expectedDSN := "user=postgres password=postgres host=localhost port=5432 database=pgx_test sslmode=disable"
		if config.DatabaseDSN != expectedDSN {
			t.Errorf("Expected DatabaseDSN %q, got %q", expectedDSN, config.DatabaseDSN)
		}

		if config.ServerAddr != "localhost:8080" {
			t.Errorf("Expected ServerAddr %q, got %q", "localhost:8080", config.ServerAddr)
		}

		expectedBaseURL := "http://localhost:8080"
		if config.BaseURL != expectedBaseURL {
			t.Errorf("Expected BaseURL %q, got %q", expectedBaseURL, config.BaseURL)
		}

		if config.FileStoragePath != "data.json" {
			t.Errorf("Expected FileStoragePath %q, got %q", "data.json", config.FileStoragePath)
		}
	})

	t.Run("FromYAML", func(t *testing.T) {
		resetFlags()
		os.Args = []string{"test"}

		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, "config.yaml")

		yamlContent := `
server_addr: ":9090"
base_url: "https://example.com"
database_dsn: "user=test password=test host=db port=5432 database=test sslmode=require"
file_storage_path: "/tmp/storage.json"
audit_file: "/var/log/audit.log"
audit_url: "http://audit.example.com/log"
`
		if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
			t.Fatalf("Failed to create config file: %v", err)
		}

		config, err := LoadConfig(configPath)
		if err != nil {
			t.Fatalf("LoadConfig failed: %v", err)
		}

		if config.ServerAddr != ":9090" {
			t.Errorf("Expected ServerAddr %q, got %q", ":9090", config.ServerAddr)
		}
		if config.BaseURL != "https://example.com" {
			t.Errorf("Expected BaseURL %q, got %q", "https://example.com", config.BaseURL)
		}
		if config.FileStoragePath != "/tmp/storage.json" {
			t.Errorf("Expected FileStoragePath %q, got %q", "/tmp/storage.json", config.FileStoragePath)
		}
		if config.AuditFile != "/var/log/audit.log" {
			t.Errorf("Expected AuditFile %q, got %q", "/var/log/audit.log", config.AuditFile)
		}
		if config.AuditURL != "http://audit.example.com/log" {
			t.Errorf("Expected AuditURL %q, got %q", "http://audit.example.com/log", config.AuditURL)
		}
	})

	t.Run("FromEnvironment", func(t *testing.T) {
		resetFlags()
		os.Args = []string{"test"}

		// Use t.Setenv for automatic cleanup
		t.Setenv("SERVER_ADDRESS", "env:9090")
		t.Setenv("BASE_URL", "https://env.example.com")
		t.Setenv("DATABASE_DSN", "env_dsn")
		t.Setenv("FILE_STORAGE_PATH", "/env/storage.json")
		t.Setenv("AUDIT_FILE", "/env/audit.log")
		t.Setenv("AUDIT_URL", "http://env.audit.com/log")

		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, "config.yaml")

		yamlContent := `
server_addr: ":8080"
base_url: "http://yaml.example.com"
database_dsn: "yaml_dsn"
file_storage_path: "/yaml/storage.json"
audit_file: "/yaml/audit.log"
audit_url: "http://yaml.audit.com/log"
`
		if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
			t.Fatalf("Failed to create config file: %v", err)
		}

		config, err := LoadConfig(configPath)
		if err != nil {
			t.Fatalf("LoadConfig failed: %v", err)
		}

		if config.ServerAddr != "env:9090" {
			t.Errorf("Expected ServerAddr from env %q, got %q", "env:9090", config.ServerAddr)
		}
		if config.BaseURL != "https://env.example.com" {
			t.Errorf("Expected BaseURL from env %q, got %q", "https://env.example.com", config.BaseURL)
		}
		if config.DatabaseDSN != "env_dsn" {
			t.Errorf("Expected DatabaseDSN from env %q, got %q", "env_dsn", config.DatabaseDSN)
		}
		if config.FileStoragePath != "/env/storage.json" {
			t.Errorf("Expected FileStoragePath from env %q, got %q", "/env/storage.json", config.FileStoragePath)
		}
		if config.AuditFile != "/env/audit.log" {
			t.Errorf("Expected AuditFile from env %q, got %q", "/env/audit.log", config.AuditFile)
		}
		if config.AuditURL != "http://env.audit.com/log" {
			t.Errorf("Expected AuditURL from env %q, got %q", "http://env.audit.com/log", config.AuditURL)
		}
	})

	t.Run("InvalidYAML", func(t *testing.T) {
		resetFlags()
		os.Args = []string{"test"}

		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, "config.yaml")

		// Write invalid YAML (unclosed quote)
		invalidYAML := `server_addr: ":8080
base_url: "http://example.com"`
		if err := os.WriteFile(configPath, []byte(invalidYAML), 0644); err != nil {
			t.Fatalf("Failed to create config file: %v", err)
		}

		_, err := LoadConfig(configPath)
		if err == nil {
			t.Error("Expected error for invalid YAML, got nil")
		} else if !strings.Contains(err.Error(), "YAML parsing error") {
			t.Errorf("Expected 'YAML parsing error' in error message, got: %v", err)
		}
	})

	t.Run("PartialYAML", func(t *testing.T) {
		resetFlags()
		os.Args = []string{"test"}

		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, "config.yaml")

		// YAML with only some fields
		yamlContent := `
server_addr: ":9999"
base_url: "https://partial.example.com"
`
		if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
			t.Fatalf("Failed to create config file: %v", err)
		}

		config, err := LoadConfig(configPath)
		if err != nil {
			t.Fatalf("LoadConfig failed: %v", err)
		}

		// Check that specified fields are taken from YAML
		if config.ServerAddr != ":9999" {
			t.Errorf("Expected ServerAddr %q, got %q", ":9999", config.ServerAddr)
		}
		if config.BaseURL != "https://partial.example.com" {
			t.Errorf("Expected BaseURL %q, got %q", "https://partial.example.com", config.BaseURL)
		}

		// Check that remaining fields have default values
		expectedDSN := "user=postgres password=postgres host=localhost port=5432 database=pgx_test sslmode=disable"
		if config.DatabaseDSN != expectedDSN {
			t.Errorf("Expected default DatabaseDSN %q, got %q", expectedDSN, config.DatabaseDSN)
		}
		if config.FileStoragePath != "data.json" {
			t.Errorf("Expected default FileStoragePath %q, got %q", "data.json", config.FileStoragePath)
		}
	})

	t.Run("EmptyConfigPath", func(t *testing.T) {
		resetFlags()
		os.Args = []string{"test"}

		// Empty string as config path
		config, err := LoadConfig("")
		if err != nil {
			t.Fatalf("LoadConfig failed: %v", err)
		}

		// Check default values
		if config.ServerAddr != "localhost:8080" {
			t.Errorf("Expected default ServerAddr, got %q", config.ServerAddr)
		}
	})

	t.Run("BaseURLFromServerAddr", func(t *testing.T) {
		resetFlags()
		os.Args = []string{"test"}

		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, "config.yaml")

		// Specify only server_addr
		yamlContent := `
server_addr: "example.com:3000"
`
		if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
			t.Fatalf("Failed to create config file: %v", err)
		}

		config, err := LoadConfig(configPath)
		if err != nil {
			t.Fatalf("LoadConfig failed: %v", err)
		}

		// Check that BaseURL is generated from server_addr
		expectedBaseURL := "http://example.com:3000"
		if config.BaseURL != expectedBaseURL {
			t.Errorf("Expected BaseURL %q, got %q", expectedBaseURL, config.BaseURL)
		}
	})

	t.Run("OnlyCommandLineFlags", func(t *testing.T) {
		resetFlags()

		// Only command line flags, without YAML and environment variables
		os.Args = []string{
			"test",
			"-a", "flag-only:7070",
			"-b", "https://flag-only.example.com",
		}

		// Non-existent config
		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, "nonexistent.yaml")

		config, err := LoadConfig(configPath)
		if err != nil {
			t.Fatalf("LoadConfig failed: %v", err)
		}

		// Check that values are taken from command line flags
		if config.ServerAddr != "flag-only:7070" {
			t.Errorf("Expected ServerAddr from flag %q, got %q", "flag-only:7070", config.ServerAddr)
		}
		if config.BaseURL != "https://flag-only.example.com" {
			t.Errorf("Expected BaseURL from flag %q, got %q", "https://flag-only.example.com", config.BaseURL)
		}
	})
}
