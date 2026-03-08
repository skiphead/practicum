package initapp

import (
	"path/filepath"
	"testing"

	"github.com/skiphead/practicum/internal/infra/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewApp_MinimalConfig(t *testing.T) {
	tmpDir := t.TempDir()
	logger := zap.NewNop() // No-op logger for tests

	cfg := createTestConfig(tmpDir)

	app, err := NewApp(logger, cfg)

	// Если PostgreSQL недоступен, initDatabase может вернуть ошибку
	// Поэтому проверяем либо успех, либо ожидаемую ошибку подключения
	if err != nil {
		assert.Contains(t, err.Error(), "failed to initialize components")
	} else {
		require.NotNil(t, app)
		assert.NotNil(t, app.httpServer)
		assert.NotNil(t, app.audit)
	}
}

func TestInitAudit_ValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	logger := zap.NewNop()

	cfg := createTestConfig(tmpDir)
	cfg.AuditFile = filepath.Join(tmpDir, "audit_test.log")
	cfg.AuditURL = "" // File-only mode

	adapter := initAudit(cfg, logger)
	assert.NotNil(t, adapter)
}

// Helper: создаёт валидный тестовый конфиг
func createTestConfig(tmpDir string) *config.Config {
	return &config.Config{
		ServerAddr:      ":0", // Любая свободная порта
		BaseURL:         "http://localhost",
		FileStoragePath: filepath.Join(tmpDir, "storage.db"),
		DatabaseDSN:     "postgres://test:test@localhost:5432/test?sslmode=disable",
		TrustedSubnet:   "127.0.0.1/32",
		SessionKey:      "test-key-32-bytes-long!!!!!!",
		AuditFile:       filepath.Join(tmpDir, "audit.log"),
		AuditURL:        "",
	}
}
