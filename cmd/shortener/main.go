// main.go
// Основной пакет приложения для сокращения URL-адресов.
// Приложение предоставляет HTTP-сервер для обработки запросов на сокращение URL-адресов,
// используя файловое хранилище или базу данных PostgreSQL для сохранения данных.
package main

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/skiphead/practicum/infra/client/postgresql"
	"github.com/skiphead/practicum/infra/config"
	"github.com/skiphead/practicum/internal/audit"
	"github.com/skiphead/practicum/internal/delivery"
	handler "github.com/skiphead/practicum/internal/delivery/handler"

	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/skiphead/practicum/internal/domain/repository"
	"github.com/skiphead/practicum/internal/usecase"
	"go.uber.org/zap"

	_ "net/http/pprof"
)

// main - точка входа в приложение.
// Инициализирует компоненты приложения и запускает сервер.
// Последовательность инициализации:
// 1. Логгер
// 2. Конфигурация
// 3. Хранилища (файловое и база данных)
// 4. Обработчики HTTP-запросов
// 5. HTTP-сервер
func main() {
	// Инициализация логгера
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}
	defer func() {
		if syncErr := logger.Sync(); syncErr != nil {
			log.Printf("Error syncing logger: %v\n", syncErr)
		}
	}()
	zap.ReplaceGlobals(logger)

	// Загрузка конфигурации
	cfg := loadConfig()

	auditClient := initAudit(cfg)

	// Инициализация хранилищ
	store := initFileStorage(cfg)
	storageRepo := initDatabase(cfg)

	// Создание обработчика URL
	h := handler.NewURLHandler(
		usecase.NewStorageUseCase(cfg.BaseURL, *store, *storageRepo),
		cfg.ServerAddr,
		cfg.BaseURL,
		cfg.SessionKey,
		auditClient)

	// Инициализация сервера
	server := initServer(cfg, h)

	// Запуск сервера
	runServer(server)
}

// runServer управляет жизненным циклом HTTP-сервера.
// Запускает сервер в отдельной горутине и обрабатывает сигналы завершения работы.
// Параметры:
// - server: экземпляр HTTP-сервера для управления
// Алгоритм:
// - Запускает сервер в отдельном канале для обработки ошибок
// - Ожидает сигналов OS Interrupt или SIGTERM
// - Выполняет graceful shutdown с таймаутом 10 секунд
func runServer(server *delivery.Server) {
	serverErrChan := server.Start()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case <-ctx.Done():
		zap.L().Info("Received shutdown signal")
	case err := <-serverErrChan:
		if err != nil {
			zap.L().Error("Server error", zap.Error(err))
		}
	}

	if err := server.Shutdown(10 * time.Second); err != nil {
		zap.L().Error("Server shutdown error", zap.Error(err))
	} else {
		zap.L().Info("Server shutdown completed")
	}
}

// initFileStorage инициализирует файловое хранилище для URL.
// Параметры:
//   - cfg: конфигурация приложения, содержащая путь к файлу хранилища
//
// Возвращает:
//   - указатель на инициализированное файловое хранилище
//   - завершает приложение при ошибке инициализации
func initFileStorage(cfg *config.Config) *repository.FileStorage {
	store, err := repository.NewCachedFileStorage(cfg.FileStoragePath)
	if err != nil {
		zap.L().Fatal("File storage initialization failed", zap.Error(err))
	}
	return &store
}

// initDatabase инициализирует подключение к PostgreSQL и применяет миграции.
// Параметры:
//   - cfg: конфигурация приложения с DSN строкой подключения
//
// Возвращает:
//   - указатель на репозиторий URL или nil при ошибке
//
// Действия:
//  1. Устанавливает соединение с пулом подключений БД
//  2. Проверяет подключение через ping
//  3. Применяет миграции через стандартную библиотеку database/sql
//  4. Создает репозиторий для работы с URL
func initDatabase(cfg *config.Config) *repository.URLRepository {
	pool, connErr := pgxpool.New(context.Background(), cfg.DatabaseDSN)
	if connErr != nil {
		zap.L().Error("pgxpool initialization failed", zap.Error(connErr))
	}
	if pool.Ping(context.Background()) == nil {
		db := stdlib.OpenDBFromPool(pool)
		if err := postgresql.Migrations(db, "migrations"); err != nil {
			zap.L().Error("postgresql migration failed", zap.Error(err))
		}
	}

	repo := repository.NewStorageRepository(pool)

	return &repo
}

// initServer создает экземпляр HTTP-сервера с использованием фреймворка Chi.
// Параметры:
//   - cfg: конфигурация сервера
//   - handler: обработчик HTTP-запросов
//
// Возвращает:
//   - сконфигурированный экземпляр сервера
//   - завершает приложение при ошибке создания сервера
func initServer(cfg *config.Config, handler *handler.URLHandler) *delivery.Server {
	srv, err := delivery.NewServerChi(cfg, handler.ChiMux())
	if err != nil {
		zap.L().Fatal("Server creation failed", zap.Error(err))
	}
	return srv
}

// loadConfig загружает конфигурацию приложения из YAML-файла.
// Возвращает:
//   - указатель на загруженную конфигурацию
//
// Логика:
//   - Пытается загрузить конфигурацию из файла configs/config.yaml
//   - При ошибке использует конфигурацию по умолчанию
//   - Выполняет валидацию обязательных параметров
//   - Завершает приложение при ошибке валидации
func loadConfig() *config.Config {
	cfg, err := config.LoadConfig("configs/config.yaml")
	if err != nil {
		cfg = config.NewDefaultConfig()
		zap.L().Info("Using default configuration after failed config load",
			zap.Error(err),
			zap.String("config_path", "configs/config.yaml"))
	}

	if err = cfg.Validate(); err != nil {
		zap.L().Fatal("Configuration validation failed", zap.Error(err))
	}
	return cfg
}

func initAudit(cfg *config.Config) *audit.Adapter {

	auditCfg := audit.Config{
		FilePath:     cfg.AuditFile,
		HTTPEndpoint: cfg.AuditURL,
		Enabled:      true,
		MaxBatchSize: 10000,
		QueueSize:    10,
	}

	adapter, err := audit.NewAdapter(auditCfg)
	if err != nil {
		zap.L().Fatal("Failed to create audit adapter", zap.Error(err))
	}

	zap.L().Info("Audit system initialized",
		zap.String("file_path", cfg.AuditFile),
		zap.String("http_endpoint", cfg.AuditURL),
	)

	return adapter
}
