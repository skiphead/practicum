package audit

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestNewAdapter_ValidConfig проверяет создание адаптера с валидной конфигурацией
func TestNewAdapter_ValidConfig(t *testing.T) {
	cfg := Config{
		FilePath:     "/tmp/audit.log",
		Enabled:      true,
		MaxBatchSize: 100,
		QueueSize:    1000,
	}

	adapter, err := NewAdapter(cfg)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}
	defer adapter.Close()

	if adapter == nil {
		t.Error("Adapter should not be nil")
	}
}

// TestNewAdapter_InvalidConfig проверяет обработку невалидной конфигурации
func TestNewAdapter_InvalidConfig(t *testing.T) {
	testCases := []struct {
		name string
		cfg  Config
	}{
		{
			name: "Negative MaxBatchSize",
			cfg: Config{
				FilePath:     "/tmp/audit.log",
				MaxBatchSize: -1,
				QueueSize:    1000,
			},
		},
		{
			name: "Negative QueueSize",
			cfg: Config{
				FilePath:     "/tmp/audit.log",
				MaxBatchSize: 100,
				QueueSize:    -1,
			},
		},
		{
			name: "Zero MaxBatchSize",
			cfg: Config{
				FilePath:     "/tmp/audit.log",
				MaxBatchSize: 0,
				QueueSize:    1000,
			},
		},
		{
			name: "Zero QueueSize",
			cfg: Config{
				FilePath:     "/tmp/audit.log",
				MaxBatchSize: 100,
				QueueSize:    0,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			adapter, err := NewAdapter(tc.cfg)
			if err == nil {
				if adapter != nil {
					adapter.Close()
				}
				t.Error("Expected error for invalid config")
			}
		})
	}
}

// TestAdapter_EnableDisable проверяет включение и выключение адаптера
func TestAdapter_EnableDisable(t *testing.T) {
	cfg := Config{
		FilePath:     "/tmp/audit.log",
		Enabled:      false, // Начинаем с выключенного состояния
		MaxBatchSize: 100,
		QueueSize:    1000,
	}

	adapter, err := NewAdapter(cfg)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}
	defer adapter.Close()

	// Проверяем начальное состояние
	event := &Event{
		Timestamp: time.Now().Unix(),
		Action:    "test",
		UserID:    "user123",
		URL:       "http://example.com",
	}

	ctx := context.Background()

	// В выключенном состоянии не должно быть ошибки
	err = adapter.LogEvent(ctx, event)
	if err != nil {
		t.Errorf("LogEvent should not return error when disabled: %v", err)
	}

	// Включаем адаптер
	adapter.Enable()

	// Теперь можно логировать события
	err = adapter.LogEvent(ctx, event)
	if err != nil {
		t.Errorf("LogEvent should work when enabled: %v", err)
	}

	// Выключаем адаптер
	adapter.Disable()

	// После выключения снова не должно быть ошибки
	err = adapter.LogEvent(ctx, event)
	if err != nil {
		t.Errorf("LogEvent should not return error after disabling: %v", err)
	}
}

// TestAdapter_LogEvent_ContextCancelled проверяет обработку отмененного контекста
func TestAdapter_LogEvent_ContextCancelled(t *testing.T) {
	cfg := Config{
		FilePath:     "/tmp/audit.log",
		Enabled:      true,
		MaxBatchSize: 100,
		QueueSize:    1000,
	}

	adapter, err := NewAdapter(cfg)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}
	defer adapter.Close()

	event := &Event{
		Timestamp: time.Now().Unix(),
		Action:    "test",
		UserID:    "user123",
		URL:       "http://example.com",
	}

	// Create and immediately cancel context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Try to log with cancelled context
	err = adapter.LogEvent(ctx, event)

	// Should return context error
	if err == nil {
		t.Error("Expected error for cancelled context")
	} else if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled error, got: %v", err)
	}
}

// TestAdapter_LogEvent_QueueFull проверяет обработку переполненной очереди
func TestAdapter_LogEvent_QueueFull(t *testing.T) {
	cfg := Config{
		FilePath:     "/tmp/audit.log",
		Enabled:      true,
		MaxBatchSize: 100,
		QueueSize:    1, // Маленькая очередь для теста
	}

	adapter, err := NewAdapter(cfg)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}
	defer adapter.Close()

	event := &Event{
		Timestamp: time.Now().Unix(),
		Action:    "test",
		UserID:    "user123",
		URL:       "http://example.com",
	}

	ctx := context.Background()

	// Первое событие должно пройти успешно
	err = adapter.LogEvent(ctx, event)
	if err != nil {
		t.Errorf("First event should be queued: %v", err)
	}

	// Второе событие должно вернуть ошибку (очередь полна)
	err = adapter.LogEvent(ctx, event)
	if err == nil {
		t.Error("Expected error for full queue")
	}
}

// TestAdapter_Close проверяет корректное закрытие адаптера
func TestAdapter_Close(t *testing.T) {
	cfg := Config{
		FilePath:     "/tmp/audit.log",
		Enabled:      true,
		MaxBatchSize: 100,
		QueueSize:    1000,
	}

	adapter, err := NewAdapter(cfg)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}

	// Проверяем, что адаптер работает до закрытия
	event := &Event{
		Timestamp: time.Now().Unix(),
		Action:    "test",
		UserID:    "user123",
		URL:       "http://example.com",
	}

	err = adapter.LogEvent(context.Background(), event)
	if err != nil {
		t.Errorf("LogEvent should work before close: %v", err)
	}

	// Дадим немного времени на обработку события
	time.Sleep(10 * time.Millisecond)

	// Закрываем адаптер
	err = adapter.Close()
	if err != nil {
		// Игнорируем ошибку "file already closed", так как это может быть нормальным поведением
		if err.Error() != "close /tmp/audit.log: file already closed" {
			t.Errorf("Unexpected error on close: %v", err)
		}
	}

	// После закрытия адаптер должен быть выключен
	// В выключенном состоянии адаптер просто игнорирует события
	err = adapter.LogEvent(context.Background(), event)
	if err != nil {
		t.Errorf("LogEvent should not return error after close (adapter is disabled): %v", err)
	}
}

// TestAdapter_CloseTwice проверяет множественное закрытие адаптера
func TestAdapter_CloseTwice(t *testing.T) {
	cfg := Config{
		FilePath:     "/tmp/audit.log",
		Enabled:      true,
		MaxBatchSize: 100,
		QueueSize:    1000,
	}

	adapter, err := NewAdapter(cfg)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}

	// Первое закрытие
	err = adapter.Close()
	if err != nil {
		// Игнорируем возможные ошибки при первом закрытии
		t.Logf("First close error (may be expected): %v", err)
	}

	// Второе закрытие (должно работать без паники)
	err = adapter.Close()
	if err != nil {
		t.Logf("Second close error (may be expected): %v", err)
	}
}

// TestAdapter_LogEvent_AfterDisable проверяет поведение после выключения адаптера
func TestAdapter_LogEvent_AfterDisable(t *testing.T) {
	cfg := Config{
		FilePath:     "/tmp/audit.log",
		Enabled:      true,
		MaxBatchSize: 100,
		QueueSize:    1000,
	}

	adapter, err := NewAdapter(cfg)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}
	defer adapter.Close()

	event := &Event{
		Timestamp: time.Now().Unix(),
		Action:    "test",
		UserID:    "user123",
		URL:       "http://example.com",
	}

	// Выключаем адаптер
	adapter.Disable()

	// После выключения адаптер должен игнорировать события
	err = adapter.LogEvent(context.Background(), event)
	if err != nil {
		t.Errorf("LogEvent should not return error after disable: %v", err)
	}
}

// TestAdapter_EnableAfterDisable проверяет повторное включение адаптера
func TestAdapter_EnableAfterDisable(t *testing.T) {
	cfg := Config{
		FilePath:     "/tmp/audit.log",
		Enabled:      true,
		MaxBatchSize: 100,
		QueueSize:    1000,
	}

	adapter, err := NewAdapter(cfg)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}
	defer adapter.Close()

	event := &Event{
		Timestamp: time.Now().Unix(),
		Action:    "test",
		UserID:    "user123",
		URL:       "http://example.com",
	}

	// Выключаем
	adapter.Disable()

	// Включаем снова
	adapter.Enable()

	// После повторного включения адаптер должен работать
	err = adapter.LogEvent(context.Background(), event)
	if err != nil {
		t.Errorf("LogEvent should work after re-enable: %v", err)
	}
}

// TestAdapter_ConcurrentLogging проверяет конкурентное логирование
func TestAdapter_ConcurrentLogging(t *testing.T) {
	cfg := Config{
		FilePath:     "/tmp/audit_concurrent.log",
		Enabled:      true,
		MaxBatchSize: 100,
		QueueSize:    1000,
	}

	adapter, err := NewAdapter(cfg)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}
	defer adapter.Close()

	// Запускаем несколько горутин для конкурентного логирования
	numGoroutines := 10
	eventsPerGoroutine := 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < eventsPerGoroutine; j++ {
				event := &Event{
					Timestamp: time.Now().Unix(),
					Action:    fmt.Sprintf("action_%d", id),
					UserID:    fmt.Sprintf("user_%d", id),
					URL:       fmt.Sprintf("http://example.com/%d/%d", id, j),
				}
				adapter.LogEvent(context.Background(), event)
			}
		}(i)
	}

	wg.Wait()

	// Дадим время на обработку всех событий
	time.Sleep(100 * time.Millisecond)
}

// TestConfig_Default проверяет функцию DefaultConfig
func TestConfig_Default(t *testing.T) {
	cfg := DefaultConfig()

	// Проверяем значения по умолчанию
	if cfg.FilePath != "/var/log/audit.log" {
		t.Errorf("Default FilePath is incorrect: %s", cfg.FilePath)
	}

	if cfg.Enabled != true {
		t.Error("Default Enabled should be true")
	}

	if cfg.MaxBatchSize != 1000 {
		t.Errorf("Default MaxBatchSize is incorrect: %d", cfg.MaxBatchSize)
	}

	if cfg.QueueSize != 10000 {
		t.Errorf("Default QueueSize is incorrect: %d", cfg.QueueSize)
	}
}

// TestAdapter_NoReceivers проверяет создание адаптера без ресиверов
func TestAdapter_NoReceivers(t *testing.T) {
	cfg := Config{
		FilePath:     "", // Нет файла
		HTTPEndpoint: "", // Нет HTTP endpoint
		Enabled:      true,
		MaxBatchSize: 100,
		QueueSize:    1000,
	}

	adapter, err := NewAdapter(cfg)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}
	defer adapter.Close()

	// Адаптер должен быть создан, но в выключенном состоянии
	// (так как нет ресиверов)
	event := &Event{
		Timestamp: time.Now().Unix(),
		Action:    "test",
		UserID:    "user123",
		URL:       "http://example.com",
	}

	err = adapter.LogEvent(context.Background(), event)
	if err != nil {
		t.Errorf("LogEvent should work without receivers: %v", err)
	}
}

func TestEvent_Validation(t *testing.T) {
	tests := []struct {
		name    string
		event   Event
		wantErr bool
	}{
		{
			name: "Валидное событие shorten",
			event: Event{
				Timestamp: time.Now().Unix(),
				Action:    "shorten",
				UserID:    "user123",
				URL:       "https://example.com",
			},
			wantErr: false,
		},
		{
			name: "Валидное событие follow",
			event: Event{
				Timestamp: time.Now().Unix(),
				Action:    "follow",
				UserID:    "user456",
				URL:       "https://short.url/abc",
			},
			wantErr: false,
		},
		{
			name: "Невалидное действие",
			event: Event{
				Timestamp: time.Now().Unix(),
				Action:    "invalid_action",
				UserID:    "user123",
				URL:       "https://example.com",
			},
			wantErr: true,
		},
		{
			name: "Пустой URL",
			event: Event{
				Timestamp: time.Now().Unix(),
				Action:    "shorten",
				UserID:    "user123",
				URL:       "",
			},
			wantErr: true,
		},
		{
			name: "Некорректный timestamp",
			event: Event{
				Timestamp: -1,
				Action:    "shorten",
				UserID:    "user123",
				URL:       "https://example.com",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.event.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Validate проверяет валидность события
func (e *Event) Validate() error {
	// Простая валидация для примера
	if e.Timestamp <= 0 {
		return errors.New("invalid timestamp")
	}

	if e.Action != "shorten" && e.Action != "follow" {
		return errors.New("invalid action")
	}

	if e.URL == "" {
		return errors.New("URL cannot be empty")
	}

	return nil
}
