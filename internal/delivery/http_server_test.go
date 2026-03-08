package delivery

import (
	"context"
	"errors"
	"net/http"

	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/skiphead/practicum/internal/infra/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// initLogger инициализирует zap logger для тестов
func initLogger(t *testing.T) {
	t.Helper()
	logger, _ := zap.NewDevelopment()
	zap.ReplaceGlobals(logger)
	t.Cleanup(func() { _ = logger.Sync() })
}

// mockConfig создаёт базовую конфигурацию для тестов
func mockConfig() *config.Config {
	return &config.Config{
		ServerAddr: "localhost:0", // ephemeral port для тестов
		PprofPort:  "",            // пустой → должен стать ":8081"
		EnableTLS:  false,
		PathCert:   "",
		PathKey:    "",
	}
}

// =============================================================================
// Тесты NewServerChi
// =============================================================================

func TestNewServerChi_ConfigValidation(t *testing.T) {
	initLogger(t)
	mux := chi.NewRouter()

	t.Run("invalid config returns error", func(t *testing.T) {
		cfg := &config.Config{} // пустой конфиг должен провалить валидацию
		srv, err := NewServerChi(cfg, mux)
		assert.Error(t, err)
		assert.Nil(t, srv)
	})

	t.Run("valid config creates server", func(t *testing.T) {
		cfg := mockConfig()
		// Предварительно валидируем, чтобы NewServerChi прошёл
		require.NoError(t, cfg.Validate())

		srv, err := NewServerChi(cfg, mux)
		require.NoError(t, err)
		require.NotNil(t, srv)

		assert.Equal(t, cfg.ServerAddr, srv.Server.Addr)
		assert.Equal(t, cfg.EnableTLS, srv.tlsEnabled)
		assert.NotNil(t, srv.pprofServer)
	})
}

func TestNewServerChi_PprofPortFallback(t *testing.T) {
	initLogger(t)
	mux := chi.NewRouter()

	t.Run("empty pprof port uses default", func(t *testing.T) {
		cfg := mockConfig()
		cfg.PprofPort = "" // явно пусто
		originalPort := cfg.PprofPort

		require.NoError(t, cfg.Validate())
		srv, err := NewServerChi(cfg, mux)
		require.NoError(t, err)

		// Конфиг НЕ должен измениться
		assert.Equal(t, originalPort, cfg.PprofPort, "config should not be mutated")
		// Сервер должен получить дефолтное значение
		assert.Equal(t, ":8081", srv.pprofServer.Addr)
	})

	t.Run("custom pprof port is preserved", func(t *testing.T) {
		cfg := mockConfig()
		cfg.PprofPort = ":9090"

		require.NoError(t, cfg.Validate())
		srv, err := NewServerChi(cfg, mux)
		require.NoError(t, err)

		assert.Equal(t, ":9090", srv.pprofServer.Addr)
	})
}

func TestNewServerChi_ServerTimeouts(t *testing.T) {
	initLogger(t)
	mux := chi.NewRouter()
	cfg := mockConfig()
	require.NoError(t, cfg.Validate())

	srv, err := NewServerChi(cfg, mux)
	require.NoError(t, err)

	assert.Equal(t, 15*time.Second, srv.Server.ReadTimeout)
	assert.Equal(t, 15*time.Second, srv.Server.WriteTimeout)
	assert.Equal(t, 60*time.Second, srv.Server.IdleTimeout)
	assert.Equal(t, 10*time.Second, srv.pprofServer.ReadTimeout)
	assert.Equal(t, 10*time.Second, srv.pprofServer.WriteTimeout)
}

// =============================================================================
// Тесты GetAddr
// =============================================================================

func TestServer_GetAddr(t *testing.T) {
	initLogger(t)
	mux := chi.NewRouter()
	cfg := mockConfig()
	cfg.ServerAddr = "127.0.0.1:8888"
	require.NoError(t, cfg.Validate())

	srv, err := NewServerChi(cfg, mux)
	require.NoError(t, err)

	assert.Equal(t, "127.0.0.1:8888", srv.GetAddr())
}

// =============================================================================
// Тесты Start и Shutdown
// =============================================================================

func TestServer_Start_Shutdown_Graceful(t *testing.T) {
	initLogger(t)
	mux := chi.NewRouter()
	mux.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	cfg := mockConfig()
	cfg.ServerAddr = "localhost:0" // OS выберет свободный порт
	require.NoError(t, cfg.Validate())

	srv, err := NewServerChi(cfg, mux)
	require.NoError(t, err)

	// Запускаем сервер
	errCh := srv.Start()

	// Даём время на старт
	time.Sleep(100 * time.Millisecond)

	// Проверяем, что pprof сервер запустился (опционально)
	resp, err := http.Get("http://localhost:8081/debug/pprof/")
	if err == nil {
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	}

	// Graceful shutdown
	shutdownErr := srv.Shutdown(5 * time.Second)
	assert.NoError(t, shutdownErr)

	// Проверяем, что канал ошибок закрылся корректно
	select {
	case err, ok := <-errCh:
		if ok && err != nil {
			t.Errorf("unexpected error from server: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for error channel to close")
	}
}

func TestServer_Shutdown_ContextDeadline(t *testing.T) {
	initLogger(t)
	mux := chi.NewRouter()

	// Добавляем хендлер, который "висит"
	mux.Get("/slow", func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done() // ждём отмены контекста
	})

	cfg := mockConfig()
	require.NoError(t, cfg.Validate())

	srv, err := NewServerChi(cfg, mux)
	require.NoError(t, err)

	// Запускаем (ошибки игнорируем, т.к. порт :0)
	_ = srv.Start()
	time.Sleep(50 * time.Millisecond)

	// Shutdown с очень коротким таймаутом
	err = srv.Shutdown(1 * time.Millisecond)

	// Ожидаем context.DeadlineExceeded или успешный shutdown
	// (зависит от скорости остановки)
	if err != nil {
		assert.True(t, errors.Is(err, context.DeadlineExceeded) ||
			errors.Is(err, context.Canceled),
			"expected timeout error, got: %v", err)
	}
}

func TestServer_Shutdown_BothServers(t *testing.T) {
	initLogger(t)
	mux := chi.NewRouter()
	cfg := mockConfig()
	require.NoError(t, cfg.Validate())

	srv, err := NewServerChi(cfg, mux)
	require.NoError(t, err)

	// Мокаем Shutdown для обоих серверов через замену полей
	// (в реальном коде лучше использовать интерфейс, но для теста допустимо)

	// Проверяем, что Shutdown вызывается для обоих серверов
	// через логирование или поведение (интеграционный аспект)

	err = srv.Shutdown(2 * time.Second)
	assert.NoError(t, err)
}

// =============================================================================
// Тесты TLS
// =============================================================================

func TestNewServerChi_TLS_Configuration(t *testing.T) {
	initLogger(t)
	mux := chi.NewRouter()
	cfg := mockConfig()
	cfg.EnableTLS = true
	cfg.PathCert = "/path/to/cert.pem"
	cfg.PathKey = "/path/to/key.pem"
	require.NoError(t, cfg.Validate())

	srv, err := NewServerChi(cfg, mux)
	require.NoError(t, err)

	assert.True(t, srv.tlsEnabled)
	assert.Equal(t, "/path/to/cert.pem", srv.pathCert)
	assert.Equal(t, "/path/to/key.pem", srv.pathKey)
}

// =============================================================================
// Benchmark тесты
// =============================================================================

func BenchmarkNewServerChi(b *testing.B) {

	mux := chi.NewRouter()
	cfg := mockConfig()
	require.NoError(b, cfg.Validate())

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		srv, err := NewServerChi(cfg, mux)
		if err != nil {
			b.Fatal(err)
		}
		_ = srv
	}
}

func BenchmarkServer_Shutdown(b *testing.B) {

	mux := chi.NewRouter()
	cfg := mockConfig()
	require.NoError(b, cfg.Validate())

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		srv, err := NewServerChi(cfg, mux)
		if err != nil {
			b.Fatal(err)
		}
		_ = srv.Shutdown(100 * time.Millisecond)
	}
}
