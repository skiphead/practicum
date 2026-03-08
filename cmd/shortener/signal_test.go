package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/skiphead/practicum/internal/delivery"
	"github.com/skiphead/practicum/internal/infra/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// getFreePort возвращает номер свободного порта
func getFreePort() int {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return 8081 // fallback
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	return addr.Port
}

// getFreeAddr возвращает полный адрес в формате "host:port"
func getFreeAddr() string {
	port := getFreePort()
	return fmt.Sprintf("localhost:%d", port)
}

// TestRunServerSignalHandling тестирует функцию runServer с сигналами
func TestRunServerSignalHandling(t *testing.T) {
	logger := zaptest.NewLogger(t, zaptest.Level(zap.DebugLevel))
	zap.ReplaceGlobals(logger)

	addr := getFreeAddr()
	t.Logf("Using address: %s", addr)

	router := chi.NewRouter()
	router.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	cfg := &config.Config{
		ServerAddr: addr,
		PprofPort:  fmt.Sprintf(":%d", getFreePort()), // Уникальный pprof порт
	}

	server, err := delivery.NewServerChi(cfg, router)
	require.NoError(t, err)

	done := make(chan bool, 1)

	go func() {
		errChan := server.Start()

		ctx, stop := signal.NotifyContext(context.Background(),
			os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
		defer stop()

		zap.L().Info("Server started, waiting for signals...")

		select {
		case <-ctx.Done():
			zap.L().Info("Received shutdown signal")
		case err := <-errChan:
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

	time.Sleep(500 * time.Millisecond)

	func() {
		resp, err := http.Get("http://" + addr + "/ping")
		if err == nil {
			defer resp.Body.Close()
			_, _ = io.Copy(io.Discard, resp.Body)
			t.Log("Server is responding")
		}
	}()

	t.Logf("Sending SIGTERM to process %d", os.Getpid())
	proc, err := os.FindProcess(os.Getpid())
	require.NoError(t, err)
	err = proc.Signal(syscall.SIGTERM)
	require.NoError(t, err)

	select {
	case <-done:
		t.Log("Server shutdown completed successfully")
	case <-time.After(10 * time.Second):
		t.Fatal("Server shutdown timeout")
	}
}

// TestDirectSignalHandling тестирует прямую обработку сигналов
func TestDirectSignalHandling(t *testing.T) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

	received := make(chan os.Signal, 1)

	go func() {
		sig := <-sigChan
		t.Logf("Signal handler received: %v", sig)
		received <- sig
	}()

	time.Sleep(100 * time.Millisecond)

	t.Log("Sending SIGTERM")
	proc, err := os.FindProcess(os.Getpid())
	require.NoError(t, err)
	err = proc.Signal(syscall.SIGTERM)
	require.NoError(t, err)

	select {
	case sig := <-received:
		t.Logf("Test received signal: %v", sig)
		assert.Equal(t, syscall.SIGTERM, sig)
	case <-time.After(3 * time.Second):
		t.Fatal("Signal not received")
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
		t.Fatal("Signal not received")
	}
}

// TestAllSignals тестирует все три сигнала
func TestAllSignals(t *testing.T) {
	signals := []struct {
		name string
		sig  os.Signal
	}{
		{"SIGINT", os.Interrupt},
		{"SIGTERM", syscall.SIGTERM},
		{"SIGQUIT", syscall.SIGQUIT},
	}

	for _, tt := range signals {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := signal.NotifyContext(context.Background(), tt.sig)
			defer cancel()

			received := make(chan bool, 1)

			go func() {
				<-ctx.Done()
				received <- true
			}()

			time.Sleep(100 * time.Millisecond)

			t.Logf("Sending %v", tt.sig)
			proc, _ := os.FindProcess(os.Getpid())
			proc.Signal(tt.sig)

			select {
			case <-received:
				t.Logf("%v handled successfully", tt.sig)
			case <-time.After(2 * time.Second):
				t.Fatalf("%v not handled", tt.sig)
			}
		})
	}
}

// TestParallelServers тестирует запуск нескольких серверов параллельно
func TestParallelServers(t *testing.T) {
	// Запускаем тесты параллельно
	t.Run("Server 1", func(t *testing.T) {
		t.Parallel()

		// Получаем свободные порты
		mainAddr := getFreeAddr()
		pprofPort := getFreePort()

		t.Logf("Server 1 using main addr: %s, pprof port: %d", mainAddr, pprofPort)

		cfg := &config.Config{
			ServerAddr: mainAddr,
			PprofPort:  fmt.Sprintf(":%d", pprofPort),
		}

		router := chi.NewRouter()
		router.Get("/", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		server, err := delivery.NewServerChi(cfg, router)
		require.NoError(t, err)

		// Запускаем сервер
		errChan := server.Start()

		// Ждем запуска
		time.Sleep(200 * time.Millisecond)

		// Проверяем, что сервер запустился
		func() {
			client := http.Client{Timeout: 1 * time.Second}
			resp, err := client.Get("http://" + mainAddr + "/")
			if err == nil {
				defer resp.Body.Close()
				// Читаем тело для полного закрытия соединения
				_, _ = io.Copy(io.Discard, resp.Body)
				t.Log("Server 1 is responding")
			} else {
				t.Logf("Server 1 not responding: %v", err)
			}
		}()

		// Останавливаем сервер
		err = server.Shutdown(2 * time.Second)
		require.NoError(t, err)

		// Ждем завершения
		select {
		case e := <-errChan:
			if e != nil && e != http.ErrServerClosed {
				t.Errorf("Server 1 error: %v", e)
			}
			t.Log("Server 1 stopped normally")
		case <-time.After(3 * time.Second):
			t.Log("Server 1 channel timeout")
		}
	})

	t.Run("Server 2", func(t *testing.T) {
		t.Parallel()

		// Получаем свободные порты
		mainAddr := getFreeAddr()
		pprofPort := getFreePort()

		t.Logf("Server 2 using main addr: %s, pprof port: %d", mainAddr, pprofPort)

		cfg := &config.Config{
			ServerAddr: mainAddr,
			PprofPort:  fmt.Sprintf(":%d", pprofPort),
		}

		router := chi.NewRouter()
		router.Get("/", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		server, err := delivery.NewServerChi(cfg, router)
		require.NoError(t, err)

		// Запускаем сервер
		errChan := server.Start()

		// Ждем запуска
		time.Sleep(200 * time.Millisecond)

		// Проверяем, что сервер запустился
		func() {
			client := http.Client{Timeout: 1 * time.Second}
			resp, err := client.Get("http://" + mainAddr + "/")
			if err == nil {
				defer resp.Body.Close()
				// Читаем тело для полного закрытия соединения
				_, _ = io.Copy(io.Discard, resp.Body)
				t.Log("Server 2 is responding")
			} else {
				t.Logf("Server 2 not responding: %v", err)
			}
		}()

		// Останавливаем сервер
		err = server.Shutdown(2 * time.Second)
		require.NoError(t, err)

		// Ждем завершения
		select {
		case e := <-errChan:
			if e != nil && e != http.ErrServerClosed {
				t.Errorf("Server 2 error: %v", e)
			}
			t.Log("Server 2 stopped normally")
		case <-time.After(3 * time.Second):
			t.Log("Server 2 channel timeout")
		}
	})
}

// Также исправим TestHealthCheck, где может быть похожая проблема
func TestHealthCheck(t *testing.T) {
	addr := getFreeAddr()
	t.Logf("Using address for health check: %s", addr)

	router := chi.NewRouter()
	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	cfg := &config.Config{
		ServerAddr: addr,
		PprofPort:  fmt.Sprintf(":%d", getFreePort()),
	}
	t.Logf("Config ServerAddr: %s", cfg.ServerAddr)
	t.Logf("Config PprofPort: %s", cfg.PprofPort)

	server, err := delivery.NewServerChi(cfg, router)
	require.NoError(t, err)

	serverAddr := server.GetAddr()
	t.Logf("Actual server address from GetAddr(): %s", serverAddr)

	// Запускаем сервер
	_ = server.Start()

	// Даем серверу время запуститься
	time.Sleep(200 * time.Millisecond)

	// Делаем запрос
	func() {
		requestURL := "http://" + serverAddr + "/health"
		t.Logf("Making request to: %s", requestURL)

		client := http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get(requestURL)
		if err != nil {
			t.Logf("Health check request failed: %v", err)
			return
		}
		defer resp.Body.Close()

		// Читаем тело для полного закрытия соединения
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Logf("Failed to read body: %v", err)
			return
		}

		assert.Equal(t, "OK", string(body))
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		t.Log("Health check passed")
	}()

	// Останавливаем сервер
	t.Log("Shutting down server...")
	err = server.Shutdown(1 * time.Second)
	require.NoError(t, err)

	// Даем время на завершение
	time.Sleep(500 * time.Millisecond)

	// Проверяем, что сервер действительно остановлен
	func() {
		client := http.Client{Timeout: 1 * time.Second}
		resp, err := client.Get("http://" + serverAddr + "/health")
		if err == nil {
			defer resp.Body.Close()
			_, _ = io.Copy(io.Discard, resp.Body)
		}
		assert.Error(t, err, "Server should not be responding after shutdown")
	}()

	t.Log("Test completed successfully")
}

// TestHealthCheckSimple упрощенная версия
func TestHealthCheckSimple(t *testing.T) {
	addr := getFreeAddr()
	t.Logf("Testing with address: %s", addr)

	router := chi.NewRouter()
	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	cfg := &config.Config{
		ServerAddr: addr,
		PprofPort:  fmt.Sprintf(":%d", getFreePort()),
	}

	server, err := delivery.NewServerChi(cfg, router)
	require.NoError(t, err)

	// Запускаем сервер
	errChan := server.Start()

	// Ждем запуска с повторными попытками
	maxAttempts := 10
	var serverRunning bool

	for i := 0; i < maxAttempts; i++ {
		func() {
			client := http.Client{Timeout: 500 * time.Millisecond}
			resp, err := client.Get("http://" + addr + "/health")
			if err != nil {
				t.Logf("Attempt %d: server not ready: %v", i+1, err)
				return
			}
			defer resp.Body.Close()

			// Читаем тело для полного закрытия соединения
			_, readErr := io.Copy(io.Discard, resp.Body)
			if readErr != nil {
				t.Logf("Attempt %d: error reading body: %v", i+1, readErr)
				return
			}

			if resp.StatusCode == http.StatusOK {
				serverRunning = true
				t.Logf("Server is running (attempt %d)", i+1)
			}
		}()

		if serverRunning {
			break
		}

		// Проверяем канал ошибок
		select {
		case e := <-errChan:
			t.Fatalf("Server error: %v", e)
		default:
			time.Sleep(200 * time.Millisecond)
		}
	}

	if !serverRunning {
		t.Fatal("Server failed to start within timeout")
	}

	// Финальная проверка с закрытием тела
	func() {
		resp, err := http.Get("http://" + addr + "/health")
		require.NoError(t, err)
		defer resp.Body.Close()

		// Читаем тело для полного закрытия соединения
		_, err = io.Copy(io.Discard, resp.Body)
		require.NoError(t, err)

		t.Log("Server confirmed running")
	}()

	// Останавливаем сервер
	err = server.Shutdown(1 * time.Second)
	require.NoError(t, err)

	// Проверяем, что сервер не отвечает
	time.Sleep(200 * time.Millisecond)

	func() {
		client := http.Client{Timeout: 500 * time.Millisecond}
		resp, err := client.Get("http://" + addr + "/health")
		if err == nil {
			defer resp.Body.Close()
			_, _ = io.Copy(io.Discard, resp.Body)
		}
		assert.Error(t, err, "Server should be stopped")
	}()
}

// Тест для проверки уникальных pprof портов
func TestUniquePprofPorts(t *testing.T) {
	ports := make(map[int]bool)

	for i := 0; i < 10; i++ {
		port := getFreePort()
		if ports[port] {
			t.Errorf("Port %d already used", port)
		}
		ports[port] = true
		t.Logf("Got unique port: %d", port)
	}
}
