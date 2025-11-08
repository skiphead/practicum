package main

import (
	"context"
	"github.com/skiphead/practicum/infra/client/postgresql"
	"github.com/skiphead/practicum/infra/config"
	"github.com/skiphead/practicum/internal/delivery"
	"github.com/skiphead/practicum/internal/delivery/handler"
	"github.com/skiphead/practicum/internal/domain/repository"
	"github.com/skiphead/practicum/internal/usecase"
	"github.com/skiphead/practicum/pkg/storage"
	"go.uber.org/zap"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}
	defer logger.Sync()
	zap.ReplaceGlobals(logger)

	cfg, err := config.LoadConfig("configs/config.yaml")
	if err != nil {
		cfg = config.NewDefaultConfig()
		zap.L().Info("Не удалось загрузить config.yaml, используется конфигурация по умолчанию",
			zap.Error(err))
	}

	if err = cfg.Validate(); err != nil {
		zap.L().Fatal("Ошибка валидации конфигурации",
			zap.Error(err))
	}

	store, err := storage.NewCachedFileStorage(cfg.FileStoragePath)
	if err != nil {
		zap.L().Panic("storage initialization failed", zap.Error(err))
	}

	pool := postgresql.Conn(cfg.DatabaseDSN)

	repoHealth := repository.NewHealthRepository(pool)

	handler := handlers.NewURLHandler(store, cfg.ServerAddr, cfg.BaseURL, *usecase.NewHealthUseCase(repoHealth))

	srv, errNewServe := delivery.NewServerChi(cfg, handler.ChiMux())
	if errNewServe != nil {
		zap.L().Fatal("Ошибка создания сервера",
			zap.Error(errNewServe))
	}

	serverErr := srv.Start()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case <-ctx.Done():
		log.Println("Получен сигнал завершения работы")
		zap.L().Warn("Получен сигнал завершения работы")
	case err = <-serverErr:
		zap.L().Fatal("Ошибка сервера",
			zap.Error(err))
	}

	if err = srv.Shutdown(10 * time.Second); err != nil {
		zap.L().Warn("Ошибка завершения работы сервера", zap.Error(err))
	}
	zap.L().Info("Сервер завершил работу корректно")
}
