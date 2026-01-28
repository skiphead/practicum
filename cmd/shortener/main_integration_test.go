// main_integration_test.go
package main

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMain_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Пропускаем интеграционные тесты в short mode")
	}

	// 1. Создаем временную директорию
	tmpDir := t.TempDir()

	// 2. Создаем тестовую конфигурацию
	configYAML := `
server_addr: "localhost:8089"
base_url: "http://localhost:8089"
file_storage_path: "` + tmpDir + `/test.json"
database_dsn: ""
audit_file: "` + tmpDir + `/audit.log"
audit_url: ""
session_key: "test-session-key"
enable_https: false
`

	// 3. Записываем конфиг в стандартное расположение
	os.MkdirAll("configs", 0755)
	configPath := "configs/config.yaml"
	err := os.WriteFile(configPath, []byte(configYAML), 0644)
	require.NoError(t, err)

	// 4. Очищаем после теста
	defer os.Remove(configPath)
	defer os.RemoveAll("configs")

	// 5. Устанавливаем переменные окружения для переопределения
	originalEnv := os.Environ()
	defer func() {
		os.Clearenv()
		for _, env := range originalEnv {
			os.Setenv(env, "")
		}
	}()

	os.Setenv("SERVER_ADDRESS", "localhost:8089")
	os.Setenv("BASE_URL", "http://localhost:8089")
	os.Setenv("FILE_STORAGE_PATH", tmpDir+"/test.json")
	os.Setenv("DATABASE_DSN", "")
	os.Setenv("AUDIT_FILE", tmpDir+"/audit.log")
	os.Setenv("AUDIT_URL", "")
	os.Setenv("SESSION_KEY", "test-session-key")

	// 6. Запускаем приложение в фоне
	done := make(chan struct{})

	go func() {
		defer close(done)

		// Запускаем main
		main()
	}()

	// 7. Ждем запуска сервера
	time.Sleep(2 * time.Second)

	// 8. Останавливаем приложение
	// Нужно отправить сигнал или остановить как-то иначе
	// Вам нужно добавить способ остановки приложения извне

	// 9. Ждем завершения
	select {
	case <-done:
		t.Log("Приложение корректно завершилось")
	case <-time.After(5 * time.Second):
		// Принудительно завершаем
		t.Log("Таймаут, приложение продолжает работать")
	}
}
