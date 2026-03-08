package audit

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// setupTest - подготовка к тесту
func setupTest(t *testing.T, testFile string) (*Logger, func()) {
	ResetInstances() // Сбрасываем все экземпляры перед тестом

	logger, err := GetInstance(testFile)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	cleanup := func() {
		logger.Close()
		CloseInstance(testFile)
		os.Remove(testFile)
	}

	return logger, cleanup
}

// TestSingletonPattern - тест на единственность экземпляра
func TestSingletonPattern(t *testing.T) {
	testFile1 := "test_singleton1.log"
	testFile2 := "test_singleton2.log"

	// Удаляем возможные предыдущие файлы
	os.Remove(testFile1)
	os.Remove(testFile2)
	defer os.Remove(testFile1)
	defer os.Remove(testFile2)

	// Первое создание экземпляра
	logger1, err := GetInstance(testFile1)
	if err != nil {
		t.Fatalf("Failed to create first instance: %v", err)
	}
	defer CloseInstance(testFile1)

	// Второе создание с тем же путем - должен вернуть тот же экземпляр
	logger2, err := GetInstance(testFile1)
	if err != nil {
		t.Fatalf("Failed to create second instance: %v", err)
	}

	// Проверяем, что это один и тот же объект
	if logger1 != logger2 {
		t.Error("Singleton pattern failed: different instances returned for same file")
	}

	// Создаем логгер с другим путем файла
	logger3, err := GetInstance(testFile2)
	if err != nil {
		t.Fatalf("Failed to create third instance: %v", err)
	}
	defer CloseInstance(testFile2)

	if logger1 == logger3 {
		t.Error("Singleton pattern failed: same instance for different file paths")
	}
}

// TestLogEntryStructure - тест структуры записи лога
func TestLogEntryStructure(t *testing.T) {
	testFile := "test_structure.log"

	logger, cleanup := setupTest(t, testFile)
	defer cleanup()

	// Логируем тестовую запись
	userID := "test_user_123"
	url := "https://example.com/test/path"
	beforeLog := time.Now().Unix()

	err := logger.LogShorten(userID, url)
	if err != nil {
		t.Fatalf("Failed to log: %v", err)
	}

	// Ждем немного, чтобы timestamp был точно больше
	time.Sleep(10 * time.Millisecond)
	afterLog := time.Now().Unix()

	// Закрываем логгер для чтения файла
	logger.Close()

	// Читаем записанную строку
	file, err := os.Open(testFile)
	if err != nil {
		t.Fatalf("Failed to open log file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		t.Fatal("No log entry found")
	}

	line := scanner.Text()
	var entry LogEntry
	err = json.Unmarshal([]byte(line), &entry)
	if err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Проверяем структуру записи
	if entry.Action != ActionShorten {
		t.Errorf("Expected action 'shorten', got '%s'", entry.Action)
	}

	if entry.UserID != userID {
		t.Errorf("Expected user_id '%s', got '%s'", userID, entry.UserID)
	}

	if entry.URL != url {
		t.Errorf("Expected url '%s', got '%s'", url, entry.URL)
	}

	// Проверяем timestamp
	if entry.TS < beforeLog || entry.TS > afterLog {
		t.Errorf("Timestamp %d not in expected range [%d, %d]", entry.TS, beforeLog, afterLog)
	}
}

// TestLogMethods - тест методов LogShorten и LogFollow
func TestLogMethods(t *testing.T) {
	testFile := "test_methods.log"

	logger, cleanup := setupTest(t, testFile)
	defer cleanup()

	// Тестируем LogShorten
	err := logger.LogShorten("user1", "https://example.com/url1")
	if err != nil {
		t.Errorf("LogShorten failed: %v", err)
	}

	// Тестируем LogFollow
	err = logger.LogFollow("user2", "https://example.com/url2")
	if err != nil {
		t.Errorf("LogFollow failed: %v", err)
	}

	// Тестируем общий метод log
	err = logger.Log(ActionShorten, "user3", "https://example.com/url3")
	if err != nil {
		t.Errorf("log method failed: %v", err)
	}

	// Закрываем для чтения
	logger.Close()

	// Проверяем количество записей
	file, err := os.Open(testFile)
	if err != nil {
		t.Fatalf("Failed to open log file: %v", err)
	}
	defer file.Close()

	lineCount := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		var entry LogEntry
		err = json.Unmarshal([]byte(line), &entry)
		if err != nil {
			t.Errorf("Failed to parse line %d: %v", lineCount+1, err)
		}

		// Проверяем валидность JSON
		if entry.URL == "" {
			t.Errorf("Empty URL in line %d", lineCount+1)
		}

		lineCount++
	}

	if lineCount != 3 {
		t.Errorf("Expected 3 log entries, got %d", lineCount)
	}
}

// TestConcurrentLogging - тест конкурентной записи
func TestConcurrentLogging(t *testing.T) {
	testFile := "test_concurrent.log"

	logger, cleanup := setupTest(t, testFile)
	defer cleanup()

	// Количество горутин для тестирования
	const goroutineCount = 50
	done := make(chan bool, goroutineCount)
	errors := make(chan error, goroutineCount)

	// Запускаем горутины
	for i := 0; i < goroutineCount; i++ {
		go func(id int) {
			userID := string(rune('A' + (id % 26))) // A, B, C, ...
			url := "https://example.com/test/" + userID

			var err error
			if id%2 == 0 {
				err = logger.LogShorten(userID, url)
			} else {
				err = logger.LogFollow(userID, url)
			}

			if err != nil {
				errors <- err
			}
			done <- true
		}(i)
	}

	// Ждем завершения всех горутин
	for i := 0; i < goroutineCount; i++ {
		<-done
	}

	// Проверяем ошибки
	select {
	case err := <-errors:
		t.Errorf("Error in concurrent logging: %v", err)
	default:
		// Нет ошибок
	}

	// Закрываем для чтения
	logger.Close()

	// Проверяем, что все записи сохранены
	file, err := os.Open(testFile)
	if err != nil {
		t.Fatalf("Failed to open log file: %v", err)
	}
	defer file.Close()

	lineCount := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lineCount++
		line := scanner.Text()

		// Проверяем валидность JSON
		if !strings.HasPrefix(line, "{") || !strings.HasSuffix(line, "}") {
			t.Errorf("Invalid JSON format in line %d: %s", lineCount, line)
		}

		var entry LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Errorf("Invalid JSON in line %d: %v", lineCount, err)
		}
	}

	if lineCount != goroutineCount {
		t.Errorf("Expected %d log entries, got %d", goroutineCount, lineCount)
	}
}

// TestFileCreation - тест создания файла
func TestFileCreation(t *testing.T) {
	testDir := "test_logs"
	testFile := filepath.Join(testDir, "nested", "path", "test.log")

	// Убедимся, что директории нет
	os.RemoveAll(testDir)
	defer os.RemoveAll(testDir)

	logger, err := GetInstance(testFile)
	if err != nil {
		t.Fatalf("Failed to create logger with nested path: %v", err)
	}
	defer CloseInstance(testFile)

	// Логируем что-нибудь
	err = logger.LogShorten("test", "https://example.com")
	if err != nil {
		t.Errorf("Failed to log: %v", err)
	}

	// Закрываем для проверки
	logger.Close()

	// Проверяем, что файл создан
	if _, err = os.Stat(testFile); os.IsNotExist(err) {
		t.Errorf("log file was not created: %s", testFile)
	}

	// Проверяем содержимое
	file, err := os.Open(testFile)
	if err != nil {
		t.Fatalf("Failed to open created file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		var entry LogEntry
		if err := json.Unmarshal([]byte(scanner.Text()), &entry); err != nil {
			t.Errorf("Failed to parse JSON from created file: %v", err)
		}
	} else {
		t.Error("No content in created file")
	}
}

// TestReopen - тест переоткрытия файла
func TestReopen(t *testing.T) {
	testFile := "test_reopen.log"

	logger, cleanup := setupTest(t, testFile)
	defer cleanup()

	// Записываем первую запись
	err := logger.LogShorten("user1", "https://example.com/1")
	if err != nil {
		t.Errorf("Failed to log first entry: %v", err)
	}

	// Закрываем файл
	logger.Close()

	// Переименовываем файл (симуляция ротации)
	renamedFile := testFile + ".old"
	os.Rename(testFile, renamedFile)
	defer os.Remove(renamedFile)

	// Переоткрываем логгер
	err = logger.Reopen()
	if err != nil {
		t.Errorf("Failed to reopen logger: %v", err)
	}

	// Записываем вторую запись
	err = logger.LogShorten("user2", "https://example.com/2")
	if err != nil {
		t.Errorf("Failed to log second entry: %v", err)
	}

	// Закрываем для проверки
	logger.Close()

	// Проверяем, что новый файл создан и содержит запись
	if _, err = os.Stat(testFile); os.IsNotExist(err) {
		t.Errorf("New log file was not created after reopen")
	}

	// Проверяем содержимое нового файла
	file, err := os.Open(testFile)
	if err != nil {
		t.Fatalf("Failed to open new log file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	hasEntry := false
	for scanner.Scan() {
		hasEntry = true
		line := scanner.Text()
		var entry LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Errorf("Failed to parse JSON: %v", err)
		}
		if entry.UserID != "user2" {
			t.Errorf("Expected user2, got %s", entry.UserID)
		}
	}

	if !hasEntry {
		t.Error("No entries found in new log file")
	}
}

// TestInvalidPath - тест с некорректным путем (упрощенный кроссплатформенный)
func TestInvalidPath(t *testing.T) {
	// Тест 1: Путь с нулевым символом - должен всегда падать
	t.Run("null character path", func(t *testing.T) {
		logger, err := GetInstance("test\x00file.log")

		// Путь с нулевым символом должен вызывать ошибку на всех системах
		if err == nil {
			t.Error("Expected error for path with null character, got nil")
			if logger != nil {
				CloseInstance("test\x00file.log")
			}
		} else {
			t.Logf("Got expected error for null character path: %v", err)
		}
	})

	// Тест 2: Пустой путь
	t.Run("empty path", func(t *testing.T) {
		logger, err := GetInstance("")

		if err == nil {
			t.Error("Expected error for empty path, got nil")
			if logger != nil {
				CloseInstance("")
			}
		} else {
			t.Logf("Got expected error for empty path: %v", err)
		}
	})

	// Тест 3: Слишком длинный путь
	t.Run("very long path", func(t *testing.T) {
		// Создаем очень длинное имя файла
		longName := string(make([]byte, 10000))
		logger, err := GetInstance(longName)

		if err == nil {
			// На некоторых системах длинные пути могут работать
			t.Log("Very long path succeeded (this may be system dependent)")
			if logger != nil {
				CloseInstance(longName)
			}
		} else {
			t.Logf("Got error for very long path: %v", err)
		}
	})

	// Тест 4: Попытка создать файл в системном расположении
	// Этот тест пропускается в CI или при запуске от root
	t.Run("system location", func(t *testing.T) {
		// Выбираем системный путь в зависимости от ОС
		var sysPath string
		switch runtime.GOOS {
		case "windows":
			sysPath = `C:\Windows\System32\config\test.log`
		case "darwin":
			sysPath = "/System/Library/test.log"
		case "linux":
			sysPath = "/proc/cpuinfo"
		default:
			sysPath = "/dev/null/test.log"
		}

		// Пропускаем тест, если мы root (может сработать)
		if os.Geteuid() == 0 {
			t.Skip("Skipping system location test when running as root")
		}

		logger, err := GetInstance(sysPath)

		if err == nil {
			t.Errorf("Expected error for system path %s, got nil", sysPath)
			if logger != nil {
				CloseInstance(sysPath)
			}
		} else {
			t.Logf("Got expected error for system path: %v", err)
		}
	})
}

// BenchmarkLogging - бенчмарк производительности
func BenchmarkLogging(b *testing.B) {
	testFile := "benchmark.log"

	// Настройка перед бенчмарком
	ResetInstances()
	logger, err := GetInstance(testFile)
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}
	defer func() {
		logger.Close()
		CloseInstance(testFile)
		os.Remove(testFile)
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := logger.LogShorten("bench_user", "https://example.com/benchmark")
		if err != nil {
			b.Fatalf("Failed to log: %v", err)
		}
	}
	b.StopTimer()
}
