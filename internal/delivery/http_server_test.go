package delivery

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/skiphead/practicum/internal/audit"
	"github.com/skiphead/practicum/internal/delivery/handler"
	"github.com/skiphead/practicum/internal/domain/entity"
	"github.com/skiphead/practicum/internal/domain/repository"
	"github.com/skiphead/practicum/internal/infra/config"
	"github.com/skiphead/practicum/internal/usecase"
)

const serverAddr = `127.0.0.1:9090`
const invalidAddr = `invalid-address`

func TestNewServer(t *testing.T) {
	cfg := &config.Config{ServerAddr: serverAddr}
	memoryStorage, _ := repository.NewCachedFileStorage("test.json")

	repoStorage := repository.NewStorageRepository(&pgxpool.Pool{})

	subnet, err := entity.NewSubnetEntity(cfg.TrustedSubnet)
	if err != nil {
		t.Fatal(err)
	}

	ipCheckerRepo := repository.NewIPCheckerRepository(subnet)

	// Инициализируем use case (Application)
	ipCheckerUseCase := usecase.NewIPCheckerUseCase(ipCheckerRepo)

	storage := usecase.NewStorageUseCase("http://localhost", memoryStorage,
		repoStorage)

	urlHandler := handler.NewURLHandler(storage, ipCheckerUseCase, cfg.ServerAddr, cfg.BaseURL, "", &audit.Adapter{})

	server, err := NewServerChi(cfg, urlHandler.ChiMux())
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	if server.Addr != cfg.ServerAddr {
		t.Errorf("Expected server address '%s', got '%s'", cfg.ServerAddr, server.Addr)
	}
}

func TestNewServer_InvalidConfig(t *testing.T) {
	cfg := &config.Config{ServerAddr: invalidAddr}
	memoryStorage, _ := repository.NewCachedFileStorage("test.json")

	repoStorage := repository.NewStorageRepository(&pgxpool.Pool{})
	urlHandler := handler.NewURLHandler(usecase.NewStorageUseCase("http://localhost", memoryStorage,
		repoStorage), nil, cfg.ServerAddr, cfg.BaseURL, "", &audit.Adapter{})

	_, err := NewServerChi(cfg, urlHandler.ChiMux())
	if err == nil {
		t.Error("Expected error for invalid config, got nil")
	}
}
