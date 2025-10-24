package main

import (
	"testing"

	"github.com/skiphead/practicum/infra/config"
	"github.com/stretchr/testify/assert"
)

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.Config
		shouldError bool
	}{
		{
			name: "Valid config",
			config: &config.Config{
				ServerAddr: "localhost:8080",
				BaseURL:    "http://localhost:8080",
			},
			shouldError: false,
		},
		{
			name: "Empty server address",
			config: &config.Config{
				ServerAddr: "",
				BaseURL:    "http://localhost:8080",
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
