package config

import (
	"testing"
)

func TestNewDefaultConfig(t *testing.T) {
	cfg := NewDefaultConfig()

	if cfg.ServerAddr != "127.0.0.1:8080" {
		t.Errorf("Expected ServerAddr '127.0.0.1:8080', got '%s'", cfg.ServerAddr)
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		wantErr bool
	}{
		{"valid address", "127.0.0.1:8080", false},
		{"valid address with different port", "localhost:3000", false},
		{"invalid address - no port", "127.0.0.1", true},
		{"invalid address - bad format", "invalid-address", true},
		{"empty address", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{ServerAddr: tt.addr}
			err := cfg.Validate()

			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
