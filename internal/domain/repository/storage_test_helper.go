package repository

import (
	"path/filepath"
	"testing"
)

// TestInterfaceCompliance проверяет что cachedFileStorage реализует весь интерфейс FileStorage
func TestInterfaceCompliance(t *testing.T) {
	var _ FileStorage = &cachedFileStorage{}
}

// TestStorageConstructor проверяет конструктор и базовую функциональность
func TestStorageConstructor(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "constructor_test.json")

	// Тестируем создание нового хранилища
	storage, err := NewCachedFileStorage(dbPath)
	if err != nil {
		t.Fatalf("NewCachedFileStorage failed: %v", err)
	}

	// Проверяем что возвращается правильный тип
	if storage == nil {
		t.Fatal("NewCachedFileStorage returned nil")
	}

	// Проверяем что можем вызывать все методы интерфейса
	err = storage.Save("user_id", "", "test", "https://test.com")
	if err != nil {
		t.Errorf("Save method failed: %v", err)
	}

	_, found, err := storage.Get("test")
	if err != nil {
		t.Errorf("Get method failed: %v", err)
	}
	if !found {
		t.Error("Get should find saved record")
	}

	stats := storage.Stats()
	if stats == nil {
		t.Error("Stats should not return nil")
	}

	// Cleanup
	if closer, ok := storage.(interface{ Close() error }); ok {
		closer.Close()
	}
}
