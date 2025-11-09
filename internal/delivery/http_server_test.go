package delivery

import (
	"github.com/skiphead/practicum/infra/client/postgresql"
	"github.com/skiphead/practicum/infra/config"
	handlers "github.com/skiphead/practicum/internal/delivery/handler"
	"github.com/skiphead/practicum/internal/domain/repository"
	"github.com/skiphead/practicum/internal/usecase"
	"testing"
)

const serverAddr = `127.0.0.1:8080`
const invalidAddr = `invalid-address`

func TestNewServer(t *testing.T) {
	cfg := &config.Config{ServerAddr: serverAddr}
	memoryStorage, _ := repository.NewCachedFileStorage("tests/test.json")
	pool := postgresql.SafeConn(cfg.DatabaseDSN)

	repoStorage := repository.NewStorageRepository(pool)
	handler := handlers.NewURLHandler(*usecase.NewStorageUseCase("http://localhost", memoryStorage, repoStorage), cfg.ServerAddr, cfg.BaseURL)

	server, err := NewServerChi(cfg, handler.ChiMux())
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
	pool := postgresql.SafeConn(cfg.DatabaseDSN)

	repoStorage := repository.NewStorageRepository(pool)
	handler := handlers.NewURLHandler(*usecase.NewStorageUseCase("http://localhost", memoryStorage, repoStorage), cfg.ServerAddr, cfg.BaseURL)

	_, err := NewServerChi(cfg, handler.ChiMux())
	if err == nil {
		t.Error("Expected error for invalid config, got nil")
	}
}
