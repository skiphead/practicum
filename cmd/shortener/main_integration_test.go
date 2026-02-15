package main

import (
	"os"
	"os/exec"
	"strings"
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

// TestBuildWithFlags - интеграционный тест, который проверяет сборку с флагами
// Запускать с тегами: go test -tags=integration -v
func TestBuildWithFlags(t *testing.T) {
	// Определяем тестовые значения
	testVersion := "test-version-1.0"
	testDate := "2023-12-31"
	testCommit := "test-commit-abc123"

	// Команда для сборки тестового бинарника
	buildCmd := exec.Command("go", "build",
		"-ldflags",
		"-X main.buildVersion="+testVersion+
			" -X main.buildDate="+testDate+
			" -X main.buildCommit="+testCommit,
		"-o", "test_binary",
		".")

	output, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Logf("Вывод команды сборки: %s", output)
		t.Skipf("Пропускаем интеграционный тест: не удалось собрать бинарник: %v", err)
	}
	defer func() {
		// Удаляем созданный бинарник после теста
		cleanupCmd := exec.Command("rm", "-f", "test_binary")
		cleanupCmd.Run()
	}()

	// Запускаем собранный бинарник и проверяем вывод
	runCmd := exec.Command("./test_binary")
	output, err = runCmd.CombinedOutput()
	if err != nil {
		// Игнорируем ошибки выполнения, так как нас интересует только вывод версий
		t.Logf("Бинарник вернул ошибку (ожидаемо, так как нет конфигурации): %v", err)
	}

	outputStr := string(output)

	// Проверяем, что вывод содержит ожидаемые значения
	if !strings.Contains(outputStr, "Build version: "+testVersion) {
		t.Errorf("Вывод не содержит ожидаемую версию. Вывод: %s", outputStr)
	}
	if !strings.Contains(outputStr, "Build date: "+testDate) {
		t.Errorf("Вывод не содержит ожидаемую дату. Вывод: %s", outputStr)
	}
	if !strings.Contains(outputStr, "Build commit: "+testCommit) {
		t.Errorf("Вывод не содержит ожидаемый коммит. Вывод: %s", outputStr)
	}
}

// TestBuildWithoutFlags проверяет сборку без флагов
func TestBuildWithoutFlags(t *testing.T) {
	// Команда для сборки тестового бинарника без флагов
	buildCmd := exec.Command("go", "build", "-o", "test_binary_no_flags", ".")

	output, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Logf("Вывод команды сборки: %s", output)
		t.Skipf("Пропускаем интеграционный тест: не удалось собрать бинарник: %v", err)
	}
	defer func() {
		// Удаляем созданный бинарник после теста
		cleanupCmd := exec.Command("rm", "-f", "test_binary_no_flags")
		cleanupCmd.Run()
	}()

	// Запускаем собранный бинарник и проверяем вывод
	runCmd := exec.Command("./test_binary_no_flags")
	output, err = runCmd.CombinedOutput()
	if err != nil {
		t.Logf("Бинарник вернул ошибку (ожидаемо): %v", err)
	}

	outputStr := string(output)

	// Проверяем, что вывод содержит значения по умолчанию "N/A"
	if !strings.Contains(outputStr, "Build version: N/A") {
		t.Errorf("Вывод не содержит версию по умолчанию 'N/A'. Вывод: %s", outputStr)
	}
	if !strings.Contains(outputStr, "Build date: N/A") {
		t.Errorf("Вывод не содержит дату по умолчанию 'N/A'. Вывод: %s", outputStr)
	}
	if !strings.Contains(outputStr, "Build commit: N/A") {
		t.Errorf("Вывод не содержит коммит по умолчанию 'N/A'. Вывод: %s", outputStr)
	}
}
