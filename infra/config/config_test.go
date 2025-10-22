package config

import (
	"os"
	"testing"
)

func TestNewDefaultConfig(t *testing.T) {
	cfg := NewDefaultConfig()

	if cfg.ServerAddr != "localhost:8080" {
		t.Errorf("Expected ServerAddr 'localhost:8080', got '%s'", cfg.ServerAddr)
	}
}
func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
		errText string
	}{
		{
			name:    "valid address with host and port",
			cfg:     &Config{ServerAddr: "localhost:8080"},
			wantErr: false,
		},
		{
			name:    "valid IPv4 address with port",
			cfg:     &Config{ServerAddr: "192.168.1.1:8080"},
			wantErr: false,
		},
		{
			name:    "valid IPv6 address with port",
			cfg:     &Config{ServerAddr: "[::1]:8080"},
			wantErr: false,
		},
		{
			name:    "missing host (only port)",
			cfg:     &Config{ServerAddr: ":8080"},
			wantErr: true,
			errText: "missing host in address \":8080\"",
		},
		{
			name:    "empty host and port",
			cfg:     &Config{ServerAddr: ":"},
			wantErr: true,
			errText: "missing host in address \":\"",
		},
		{
			name:    "malformed address (no colon)",
			cfg:     &Config{ServerAddr: "localhost8080"},
			wantErr: true,
			errText: "address localhost8080: missing port in address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && err.Error() != tt.errText {
				t.Errorf("Validate() error = %v, want err text %v", err.Error(), tt.errText)
			}
		})
	}
}

func TestGetEnvLoadConfig(t *testing.T) {

	key := "SERVER_ADDRESS"
	value := "localhost:8082"
	os.Setenv(key, value)
	defer os.Unsetenv(key)

	cfg, err := LoadConfig("configs/config.yaml")
	if err != nil {
		t.Errorf("Error loading config: %v", err)
	}

	//Тест переменная установлена
	if cfg.ServerAddr != value {
		t.Errorf("Expected '%s', got '%s'", value, cfg.ServerAddr)
	}
	//Тест переменная не установлена
	resultEmpty := os.Getenv("Какая-то-переменная")
	if cfg.ServerAddr == resultEmpty {
		t.Errorf("Expected empty string, got '%s'", resultEmpty)
	}
}
