package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/skiphead/practicum/infra/config"
	"github.com/skiphead/practicum/internal/delivery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// TestRunServerSignalHandling тестирует функцию runServer с сигналами
func TestRunServerSignalHandling(t *testing.T) {
	logger := zaptest.NewLogger(t, zaptest.Level(zap.DebugLevel))
	zap.ReplaceGlobals(logger)

	// Создаем тестовый сервер
	router := chi.NewRouter()
	router.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	cfg := &config.Config{
		ServerAddr: "localhost:8082",
	}

	server, err := delivery.NewServerChi(cfg, router)
	require.NoError(t, err)

	// Канал для отслеживания завершения сервера
	done := make(chan bool, 1)

	// Запускаем runServer в горутине
	go func() {
		// Эмулируем функцию runServer из main.go
		func() {
			serverErrChan := server.Start()

			ctx, stop := signal.NotifyContext(context.Background(),
				os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
			defer stop()

			zap.L().Info("Server started, waiting for signals...")

			select {
			case <-ctx.Done():
				zap.L().Info("Received shutdown signal")
			case err := <-serverErrChan:
				if err != nil {
					zap.L().Error("Server error", zap.Error(err))
				}
			}

			zap.L().Info("Initiating graceful shutdown...")
			if err := server.Shutdown(5 * time.Second); err != nil {
				zap.L().Error("Server shutdown error", zap.Error(err))
			} else {
				zap.L().Info("Server shutdown completed")
			}
			done <- true
		}()
	}()

	// Даем серверу время запуститься
	time.Sleep(500 * time.Millisecond)

	// Проверяем, что сервер отвечает
	resp, err := http.Get("http://localhost:8082/ping")
	if err == nil {
		resp.Body.Close()
		t.Log("Server is responding")
	}

	// Отправляем сигнал SIGTERM
	t.Logf("Sending SIGTERM to process %d", os.Getpid())
	proc, err := os.FindProcess(os.Getpid())
	require.NoError(t, err)
	err = proc.Signal(syscall.SIGTERM)
	require.NoError(t, err)

	// Ждем завершения сервера с таймаутом
	select {
	case <-done:
		t.Log("Server shutdown completed successfully")
	case <-time.After(10 * time.Second):
		t.Fatal("Server shutdown timeout - server did not stop")
	}

	// Проверяем, что сервер действительно остановлен
	_, err = http.Get("http://localhost:8082/ping")
	assert.Error(t, err, "Server should not be responding after shutdown")
}

// TestDirectSignalHandling тестирует прямую обработку сигналов
func TestDirectSignalHandling(t *testing.T) {
	// Создаем канал для сигналов
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

	// Канал для получения сигнала в тесте
	received := make(chan os.Signal, 1)

	go func() {
		sig := <-sigChan
		t.Logf("Signal handler received: %v", sig)
		received <- sig
	}()

	// Даем горутине время запуститься
	time.Sleep(100 * time.Millisecond)

	// Отправляем сигнал
	t.Log("Sending SIGTERM")
	proc, err := os.FindProcess(os.Getpid())
	require.NoError(t, err)
	err = proc.Signal(syscall.SIGTERM)
	require.NoError(t, err)

	// Ждем получения сигнала
	select {
	case sig := <-received:
		t.Logf("Test received signal: %v", sig)
		assert.Equal(t, syscall.SIGTERM, sig)
	case <-time.After(3 * time.Second):
		t.Fatal("Signal not received in direct handler")
	}
}

// TestSignalWithContext тестирует signal.NotifyContext
func TestSignalWithContext(t *testing.T) {
	ctx, cancel := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	defer cancel()

	received := make(chan bool, 1)

	go func() {
		<-ctx.Done()
		t.Log("Context done signal received")
		received <- true
	}()

	time.Sleep(100 * time.Millisecond)

	t.Log("Sending SIGTERM")
	proc, err := os.FindProcess(os.Getpid())
	require.NoError(t, err)
	err = proc.Signal(syscall.SIGTERM)
	require.NoError(t, err)

	select {
	case <-received:
		t.Log("Signal received via context")
	case <-time.After(3 * time.Second):
		t.Fatal("Signal not received via context")
	}
}

// TestMultipleSignalsToContext тест отправки нескольких сигналов
func TestMultipleSignalsToContext(t *testing.T) {
	ctx, cancel := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	defer cancel()

	received := make(chan bool, 1)

	go func() {
		<-ctx.Done()
		t.Log("First signal received")
		received <- true

		// Проверяем, что второй сигнал не вызывает панику
		time.Sleep(500 * time.Millisecond)
		t.Log("Context still works after first signal")
	}()

	time.Sleep(100 * time.Millisecond)

	proc, err := os.FindProcess(os.Getpid())
	require.NoError(t, err)

	// Отправляем первый сигнал
	t.Log("Sending first SIGTERM")
	err = proc.Signal(syscall.SIGTERM)
	require.NoError(t, err)

	select {
	case <-received:
		t.Log("First signal handled")
	case <-time.After(3 * time.Second):
		t.Fatal("First signal not received")
	}

	// Отправляем второй сигнал (должен игнорироваться или не вызывать ошибок)
	t.Log("Sending second SIGTERM")
	err = proc.Signal(syscall.SIGTERM)
	require.NoError(t, err)

	// Даем время на возможную обработку
	time.Sleep(500 * time.Millisecond)
	t.Log("Test completed")
}
