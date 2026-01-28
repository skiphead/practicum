// audit/adapter_config_test.go
package audit

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "Валидная конфигурация с файлом",
			config: Config{
				FilePath:     "test.log",
				HTTPEndpoint: "",
				Enabled:      true,
				MaxBatchSize: 100,
				QueueSize:    1000,
			},
			wantErr: false,
		},
		{
			name: "Валидная конфигурация с HTTP",
			config: Config{
				FilePath:     "",
				HTTPEndpoint: "http://localhost:8080",
				Enabled:      true,
				MaxBatchSize: 100,
				QueueSize:    1000,
			},
			wantErr: false,
		},
		{
			name: "Валидная конфигурация с обоими приемниками",
			config: Config{
				FilePath:     "test.log",
				HTTPEndpoint: "http://localhost:8080",
				Enabled:      true,
				MaxBatchSize: 100,
				QueueSize:    1000,
			},
			wantErr: false,
		},
		{
			name: "Нулевой MaxBatchSize",
			config: Config{
				FilePath:     "test.log",
				MaxBatchSize: 0,
				QueueSize:    1000,
			},
			wantErr: true,
			errMsg:  "MaxBatchSize must be positive",
		},
		{
			name: "Отрицательный MaxBatchSize",
			config: Config{
				FilePath:     "test.log",
				MaxBatchSize: -1,
				QueueSize:    1000,
			},
			wantErr: true,
			errMsg:  "MaxBatchSize must be positive",
		},
		{
			name: "Нулевой QueueSize",
			config: Config{
				FilePath:     "test.log",
				MaxBatchSize: 100,
				QueueSize:    0,
			},
			wantErr: true,
			errMsg:  "QueueSize must be positive",
		},
		{
			name: "Отрицательный QueueSize",
			config: Config{
				FilePath:     "test.log",
				MaxBatchSize: 100,
				QueueSize:    -1,
			},
			wantErr: true,
			errMsg:  "QueueSize must be positive",
		},
		{
			name: "Конфигурация без приемников",
			config: Config{
				FilePath:     "",
				HTTPEndpoint: "",
				Enabled:      true,
				MaxBatchSize: 100,
				QueueSize:    1000,
			},
			wantErr: false, // Должно быть валидно, адаптер будет в no-op режиме
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, "/var/log/audit.log", cfg.FilePath)
	assert.Equal(t, "", cfg.HTTPEndpoint)
	assert.True(t, cfg.Enabled)
	assert.Equal(t, 1000, cfg.MaxBatchSize)
	assert.Equal(t, 10000, cfg.QueueSize)

	// Должна быть валидной
	err := cfg.Validate()
	assert.NoError(t, err)
}
