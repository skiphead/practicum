package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCachedFileStorage(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_db.json")

	t.Run("NewCachedFileStorage", func(t *testing.T) {
		storage, err := NewCachedFileStorage(dbPath)
		if err != nil {
			t.Fatalf("Failed to create storage: %v", err)
		}
		defer cleanupStorage(storage, dbPath)

		if storage == nil {
			t.Fatal("Storage should not be nil")
		}
	})

	t.Run("Save and Get", func(t *testing.T) {
		storage, err := NewCachedFileStorage(dbPath)
		if err != nil {
			t.Fatalf("Failed to create storage: %v", err)
		}
		defer cleanupStorage(storage, dbPath)

		// Сохраняем запись
		err = storage.Save("abc123", "https://example.com")
		if err != nil {
			t.Fatalf("Save failed: %v", err)
		}

		// Получаем запись
		record, found, err := storage.Get("abc123")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if !found {
			t.Fatal("Record should be found")
		}
		if record.ShortURL != "abc123" {
			t.Errorf("Expected ShortURL 'abc123', got '%s'", record.ShortURL)
		}
		if record.OriginalURL != "https://example.com" {
			t.Errorf("Expected OriginalURL 'https://example.com', got '%s'", record.OriginalURL)
		}
		if record.UUID == "" {
			t.Error("UUID should not be empty")
		}
	})

	t.Run("Get non-existent", func(t *testing.T) {
		storage, err := NewCachedFileStorage(dbPath)
		if err != nil {
			t.Fatalf("Failed to create storage: %v", err)
		}
		defer cleanupStorage(storage, dbPath)

		record, found, err := storage.Get("nonexistent")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if found {
			t.Fatal("Record should not be found")
		}
		if record != nil {
			t.Error("Record should be nil for non-existent key")
		}
	})

	t.Run("GetByID", func(t *testing.T) {
		storage, err := NewCachedFileStorage(dbPath)
		if err != nil {
			t.Fatalf("Failed to create storage: %v", err)
		}
		defer cleanupStorage(storage, dbPath)

		// Сохраняем запись
		err = storage.Save("test123", "https://test.com")
		if err != nil {
			t.Fatalf("Save failed: %v", err)
		}

		// Получаем запись по shortURL чтобы узнать ID
		record, found, err := storage.Get("test123")
		if err != nil || !found {
			t.Fatalf("Failed to get record: %v", err)
		}

		// Получаем по ID
		recordByID, err := storage.GetByID(record.UUID)
		if err != nil {
			t.Fatalf("GetByID failed: %v", err)
		}
		if recordByID.ShortURL != "test123" {
			t.Errorf("Expected ShortURL 'test123', got '%s'", recordByID.ShortURL)
		}
	})

	t.Run("GetByID non-existent", func(t *testing.T) {
		storage, err := NewCachedFileStorage(dbPath)
		if err != nil {
			t.Fatalf("Failed to create storage: %v", err)
		}
		defer cleanupStorage(storage, dbPath)

		_, err = storage.GetByID("nonexistent-id")
		if err == nil {
			t.Error("Expected error for non-existent ID")
		}
	})

	t.Run("FindByOriginalURL", func(t *testing.T) {
		storage, err := NewCachedFileStorage(dbPath)
		if err != nil {
			t.Fatalf("Failed to create storage: %v", err)
		}
		defer cleanupStorage(storage, dbPath)

		originalURL := "https://find-me.com"
		err = storage.Save("find456", originalURL)
		if err != nil {
			t.Fatalf("Save failed: %v", err)
		}

		record, err := storage.FindByOriginalURL(originalURL)
		if err != nil {
			t.Fatalf("FindByOriginalURL failed: %v", err)
		}
		if record.OriginalURL != originalURL {
			t.Errorf("Expected OriginalURL '%s', got '%s'", originalURL, record.OriginalURL)
		}
		if record.ShortURL != "find456" {
			t.Errorf("Expected ShortURL 'find456', got '%s'", record.ShortURL)
		}
	})

	t.Run("FindByOriginalURL non-existent", func(t *testing.T) {
		storage, err := NewCachedFileStorage(dbPath)
		if err != nil {
			t.Fatalf("Failed to create storage: %v", err)
		}
		defer cleanupStorage(storage, dbPath)

		_, err = storage.FindByOriginalURL("https://not-exist.com")
		if err == nil {
			t.Error("Expected error for non-existent original URL")
		}
	})

	t.Run("Delete by shortURL", func(t *testing.T) {
		storage, err := NewCachedFileStorage(dbPath)
		if err != nil {
			t.Fatalf("Failed to create storage: %v", err)
		}
		defer cleanupStorage(storage, dbPath)

		// Сохраняем запись
		err = storage.Save("todelete", "https://delete.com")
		if err != nil {
			t.Fatalf("Save failed: %v", err)
		}

		// Проверяем что запись существует
		_, found, _ := storage.Get("todelete")
		if !found {
			t.Fatal("Record should exist before deletion")
		}

		// Удаляем
		err = storage.Delete("todelete")
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		// Проверяем что записи больше нет
		_, found, _ = storage.Get("todelete")
		if found {
			t.Fatal("Record should not exist after deletion")
		}
	})

	t.Run("Delete non-existent shortURL", func(t *testing.T) {
		storage, err := NewCachedFileStorage(dbPath)
		if err != nil {
			t.Fatalf("Failed to create storage: %v", err)
		}
		defer cleanupStorage(storage, dbPath)

		err = storage.Delete("nonexistent")
		if err == nil {
			t.Error("Expected error when deleting non-existent shortURL")
		}
	})

	t.Run("DeleteByID", func(t *testing.T) {
		storage, err := NewCachedFileStorage(dbPath)
		if err != nil {
			t.Fatalf("Failed to create storage: %v", err)
		}
		defer cleanupStorage(storage, dbPath)

		// Сохраняем запись
		err = storage.Save("deleteid", "https://delete-id.com")
		if err != nil {
			t.Fatalf("Save failed: %v", err)
		}

		// Получаем ID
		record, found, _ := storage.Get("deleteid")
		if !found {
			t.Fatal("Record should exist")
		}

		// Удаляем по ID
		err = storage.DeleteByID(record.UUID)
		if err != nil {
			t.Fatalf("DeleteByID failed: %v", err)
		}

		// Проверяем что записи больше нет
		_, found, _ = storage.Get("deleteid")
		if found {
			t.Fatal("Record should not exist after deletion")
		}
	})

	t.Run("DeleteByID non-existent", func(t *testing.T) {
		storage, err := NewCachedFileStorage(dbPath)
		if err != nil {
			t.Fatalf("Failed to create storage: %v", err)
		}
		defer cleanupStorage(storage, dbPath)

		err = storage.DeleteByID("nonexistent-id")
		if err == nil {
			t.Error("Expected error when deleting non-existent ID")
		}
	})

	t.Run("Stats", func(t *testing.T) {
		storage, err := NewCachedFileStorage(dbPath)
		if err != nil {
			t.Fatalf("Failed to create storage: %v", err)
		}
		defer cleanupStorage(storage, dbPath)

		stats := storage.Stats()

		// Проверяем основные поля статистики
		if stats["storage_type"] != "cached_file" {
			t.Errorf("Expected storage_type 'cached_file', got '%s'", stats["storage_type"])
		}
		if stats["file_path"] != dbPath {
			t.Errorf("Expected file_path '%s', got '%s'", dbPath, stats["file_path"])
		}

		// total_records должно быть числом
		if total, ok := stats["total_records"].(int); !ok {
			t.Error("total_records should be an integer")
		} else if total < 0 {
			t.Error("total_records should be non-negative")
		}
	})

	t.Run("Persistence", func(t *testing.T) {
		// Тестируем сохранение данных между сессиями
		storage1, err := NewCachedFileStorage(dbPath)
		if err != nil {
			t.Fatalf("Failed to create storage: %v", err)
		}

		// Сохраняем данные в первом хранилище
		err = storage1.Save("persist1", "https://persist1.com")
		if err != nil {
			t.Fatalf("Save failed: %v", err)
		}
		err = storage1.Save("persist2", "https://persist2.com")
		if err != nil {
			t.Fatalf("Save failed: %v", err)
		}

		// Закрываем первое хранилище
		if closer, ok := storage1.(interface{ Close() error }); ok {
			closer.Close()
		}

		// Создаем второе хранилище с тем же файлом
		storage2, err := NewCachedFileStorage(dbPath)
		if err != nil {
			t.Fatalf("Failed to create second storage: %v", err)
		}
		defer cleanupStorage(storage2, dbPath)

		// Проверяем что данные сохранились
		record1, found, err := storage2.Get("persist1")
		if err != nil || !found {
			t.Fatalf("Failed to get persisted record1: %v", err)
		}
		if record1.OriginalURL != "https://persist1.com" {
			t.Errorf("Expected OriginalURL 'https://persist1.com', got '%s'", record1.OriginalURL)
		}

		record2, found, err := storage2.Get("persist2")
		if err != nil || !found {
			t.Fatalf("Failed to get persisted record2: %v", err)
		}
		if record2.OriginalURL != "https://persist2.com" {
			t.Errorf("Expected OriginalURL 'https://persist2.com', got '%s'", record2.OriginalURL)
		}
	})

	t.Run("Update existing", func(t *testing.T) {
		storage, err := NewCachedFileStorage(dbPath)
		if err != nil {
			t.Fatalf("Failed to create storage: %v", err)
		}
		defer cleanupStorage(storage, dbPath)

		// Сохраняем первую версию
		err = storage.Save("update", "https://first.com")
		if err != nil {
			t.Fatalf("First save failed: %v", err)
		}

		// Получаем первую версию
		record1, found, _ := storage.Get("update")
		if !found {
			t.Fatal("First record should exist")
		}
		firstUUID := record1.UUID

		// Сохраняем вторую версию с тем же shortURL
		err = storage.Save("update", "https://second.com")
		if err != nil {
			t.Fatalf("Second save failed: %v", err)
		}

		// Получаем вторую версию
		record2, found, _ := storage.Get("update")
		if !found {
			t.Fatal("Second record should exist")
		}

		// UUID должен остаться тем же (или измениться? зависит от логики)
		// В текущей реализации генерируется новый UUID при каждом Save
		if record2.UUID == firstUUID {
			t.Log("UUID remained the same after update")
		}

		if record2.OriginalURL != "https://second.com" {
			t.Errorf("Expected updated OriginalURL 'https://second.com', got '%s'", record2.OriginalURL)
		}
	})

	t.Run("Concurrent access", func(t *testing.T) {
		storage, err := NewCachedFileStorage(dbPath)
		if err != nil {
			t.Fatalf("Failed to create storage: %v", err)
		}
		defer cleanupStorage(storage, dbPath)

		// Запускаем несколько горутин для конкурентного доступа
		done := make(chan bool)
		errors := make(chan error, 10)

		// Горутины для записи
		for i := 0; i < 5; i++ {
			go func(index int) {
				key := string(rune('a' + index))
				err := storage.Save(key, "https://concurrent.com")
				if err != nil {
					errors <- err
				}
				done <- true
			}(i)
		}

		// Горутины для чтения
		for i := 0; i < 5; i++ {
			go func(index int) {
				key := string(rune('a' + index))
				storage.Get(key)
				done <- true
			}(i)
		}

		// Ожидаем завершения всех горутин
		for i := 0; i < 10; i++ {
			<-done
		}

		// Проверяем ошибки
		close(errors)
		for err := range errors {
			t.Errorf("Concurrent access error: %v", err)
		}

		// Проверяем что все записи сохранились
		stats := storage.Stats()
		if total, ok := stats["total_records"].(int); !ok || total < 5 {
			t.Errorf("Expected at least 5 records, got %d", total)
		}
	})
}

func TestURLRecord(t *testing.T) {
	// Тестируем структуру URLRecord
	now := time.Now()
	record := URLRecord{
		UUID:        "test-uuid",
		ShortURL:    "abc123",
		OriginalURL: "https://example.com",
		CreatedAt:   now,
	}

	if record.UUID != "test-uuid" {
		t.Errorf("Expected UUID 'test-uuid', got '%s'", record.UUID)
	}
	if record.ShortURL != "abc123" {
		t.Errorf("Expected ShortURL 'abc123', got '%s'", record.ShortURL)
	}
	if record.OriginalURL != "https://example.com" {
		t.Errorf("Expected OriginalURL 'https://example.com', got '%s'", record.OriginalURL)
	}
	if !record.CreatedAt.Equal(now) {
		t.Error("CreatedAt time mismatch")
	}
}

// Вспомогательная функция для очистки
func cleanupStorage(storage Storage, dbPath string) {
	// Если storage реализует Close, вызываем его
	if closer, ok := storage.(interface{ Close() error }); ok {
		closer.Close()
	}

	// Удаляем тестовый файл если он существует
	if dbPath != "" {
		os.Remove(dbPath)
	}
}

// Benchmark тесты для измерения производительности
func BenchmarkStorageOperations(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "benchmark_db.json")

	storage, err := NewCachedFileStorage(dbPath)
	if err != nil {
		b.Fatalf("Failed to create storage: %v", err)
	}
	defer cleanupStorage(storage, dbPath)

	b.Run("Save", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			key := string(rune('a' + i%26))
			storage.Save(key, "https://benchmark.com")
		}
	})

	b.Run("Get", func(b *testing.B) {
		// Сначала сохраним несколько записей
		for i := 0; i < 100; i++ {
			key := string(rune('a' + i%26))
			storage.Save(key, "https://benchmark.com")
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := string(rune('a' + i%26))
			storage.Get(key)
		}
	})
}
