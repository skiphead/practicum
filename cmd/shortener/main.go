package main

import (
	"context"
	"github.com/skiphead/practicum/infra/config"
	"github.com/skiphead/practicum/internal/delivery"
	"github.com/skiphead/practicum/internal/delivery/handler"
	"github.com/skiphead/practicum/pkg/storage"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg, err := config.LoadConfig("configs/config.yaml")
	if err != nil {
		cfg = config.NewDefaultConfig()
		log.Printf("Не удалось загрузить config.yaml, используется конфигурация по умолчанию: %v", err)
	}

	if err = cfg.Validate(); err != nil {
		log.Fatalf("Ошибка валидации конфигурации: %v", err)
	}

	store := storage.NewMemoryStorage()
	handler := handlers.NewURLHandler(store, cfg.ServerAddr, cfg.BaseURL)

	srv, errNewServe := delivery.NewServerChi(cfg, handler.ChiMux())
	if errNewServe != nil {
		log.Fatal("Ошибка создания сервера:", errNewServe)
	}

	serverErr := srv.Start()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case <-ctx.Done():
		log.Println("Получен сигнал завершения работы")
	case err = <-serverErr:
		log.Fatalf("Ошибка сервера: %v", err)
	}

	if err = srv.Shutdown(10 * time.Second); err != nil {
		log.Printf("Ошибка завершения работы сервера: %v", err)
	}

	log.Println("Сервер завершил работу корректно")
}
