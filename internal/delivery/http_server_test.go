package delivery

import (
	"github.com/skiphead/practicum/infra/config"
	handlers "github.com/skiphead/practicum/internal/delivery/handler"
	"github.com/skiphead/practicum/pkg/storage"
	"testing"
)

const serverAddr = `127.0.0.1:8080`
const invalidAddr = `invalid-address`

func TestNewServer(t *testing.T) {
	cfg := &config.Config{ServerAddr: serverAddr}
	memoryStorage := storage.NewMemoryStorage()
	handler := handlers.NewURLHandler(memoryStorage, cfg.ServerAddr, "")

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
	memoryStorage := storage.NewMemoryStorage()
	handler := handlers.NewURLHandler(memoryStorage, cfg.ServerAddr, "")

	_, err := NewServerChi(cfg, handler.ChiMux())
	if err == nil {
		t.Error("Expected error for invalid config, got nil")
	}
}
