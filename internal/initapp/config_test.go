package initapp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_Defaults(t *testing.T) {
	// Сохраняем оригинальные аргументы и env
	origArgs := os.Args
	origEnv := os.Getenv("CONFIG")
	defer func() {
		os.Args = origArgs
		if origEnv == "" {
			os.Unsetenv("CONFIG")
		} else {
			os.Setenv("CONFIG", origEnv)
		}
	}()

	// Очищаем флаги для теста
	os.Args = []string{"cmd"}

	cfg := LoadConfig()

	require.NotNil(t, cfg)
	assert.NotEmpty(t, cfg.ServerAddr)
	assert.NotEmpty(t, cfg.BaseURL)
}

func TestLoadConfig_ValidationFails(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.json")

	// Конфиг без обязательных полей
	err := os.WriteFile(configPath, []byte(`{"server_addr": ""}`), 0644)
	require.NoError(t, err)

	os.Args = []string{"cmd", "-config", configPath}

	// Ожидаем panic от Fatal
	assert.Panics(t, func() {
		LoadConfig()
	})
}
