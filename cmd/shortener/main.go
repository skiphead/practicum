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
	"github.com/skiphead/practicum/internal/delivery"
	"github.com/skiphead/practicum/internal/delivery/handler"
	"github.com/skiphead/practicum/internal/domain/repository"
	"github.com/skiphead/practicum/internal/usecase"
	"go.uber.org/zap"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
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

	// Инициализация хранилищ
	store := initFileStorage(cfg)

	pool, connErr := pgxpool.New(context.Background(), cfg.DatabaseDSN)
	if connErr != nil {
		zap.L().Error("pgxpool initialization failed", zap.Error(connErr))
	}
	if pool.Ping(context.Background()) == nil {
		db := stdlib.OpenDBFromPool(pool)
		if err = postgresql.Migrations(db, "migrations"); err != nil {
			zap.L().Error("postgresql migration failed", zap.Error(err))
		}
	}
	storageRepo := repository.NewStorageRepository(pool)

	// Создание обработчика URL
	handler := handlers.NewURLHandler(
		usecase.NewStorageUseCase(cfg.BaseURL, *store, storageRepo),
		cfg.ServerAddr,
		cfg.BaseURL)

	// Инициализация сервера
	server := initServer(cfg, handler)

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

// initServer создает экземпляр HTTP-сервера с использованием фреймворка Chi.
// Параметры:
//   - cfg: конфигурация сервера
//   - handler: обработчик HTTP-запросов
//
// Возвращает:
//   - сконфигурированный экземпляр сервера
//   - завершает приложение при ошибке создания сервера
func initServer(cfg *config.Config, handler *handlers.URLHandler) *delivery.Server {
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
