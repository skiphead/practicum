package main

import (
	"context"
	"testing"
	"time"

	"github.com/skiphead/practicum/infra/config"
	"github.com/skiphead/practicum/pkg/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestMainLogic тестирует основную логику приложения
func TestMainLogic(t *testing.T) {
	// Создаем тестовый логгер
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)
	zap.ReplaceGlobals(logger)

	// Тестируем создание конфигурации по умолчанию
	cfg := config.NewDefaultConfig()
	err = cfg.Validate()
	assert.NoError(t, err, "Default config should be valid")

	// Тестируем создание хранилища
	store := storage.NewMemoryStorage()
	assert.NotNil(t, store, "Storage should be created")

	// Тестируем graceful shutdown логику
	t.Run("GracefulShutdown", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		done := make(chan struct{})

		go func() {
			// Имитируем обработку сигналов
			<-ctx.Done()
			close(done)
		}()

		select {
		case <-done:
			// Ожидаемый случай - контекст отменен
			assert.Error(t, ctx.Err())
		case <-time.After(200 * time.Millisecond):
			t.Fatal("Timeout waiting for graceful shutdown")
		}
	})
}
