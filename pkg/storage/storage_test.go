package storage

import (
	"sync"
	"testing"
)

func TestMemoryStorage_SaveAndGet(t *testing.T) {
	storage := NewMemoryStorage()

	// Тест сохранения и получения
	storage.Save("test", "https://example.com")

	url, exists := storage.Get("test")
	if !exists {
		t.Error("Expected to find key 'test'")
	}
	if url != "https://example.com" {
		t.Errorf("Expected 'https://example.com', got '%s'", url)
	}
}

func TestMemoryStorage_GetNotExists(t *testing.T) {
	storage := NewMemoryStorage()

	// Тест получения несуществующего ключа
	_, exists := storage.Get("missing")
	if exists {
		t.Error("Expected not to find key 'missing'")
	}
}

func TestMemoryStorage_Overwrite(t *testing.T) {
	storage := NewMemoryStorage()

	// Тест перезаписи значения
	storage.Save("key", "first_value")
	storage.Save("key", "second_value")

	url, _ := storage.Get("key")
	if url != "second_value" {
		t.Errorf("Expected 'second_value', got '%s'", url)
	}
}

func TestMemoryStorage_ConcurrentAccess(t *testing.T) {
	storage := NewMemoryStorage()
	var wg sync.WaitGroup

	// Конкурентные записи
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			storage.Save(string(rune(n)), "url")
		}(i)
	}

	// Конкурентные чтения
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			storage.Get(string(rune(n)))
		}(i)
	}

	wg.Wait()
}

func TestMemoryStorage_RaceCondition(t *testing.T) {
	storage := NewMemoryStorage()
	var wg sync.WaitGroup

	// Параллельные операции записи и чтения
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			storage.Save("key", "url1")
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			storage.Get("key")
		}
	}()

	wg.Wait()
}
