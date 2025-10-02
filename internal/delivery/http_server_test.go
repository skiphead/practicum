package delivery

import (
	"bytes"
	"github.com/skiphead/practicum/internal/config"
	handlers "github.com/skiphead/practicum/internal/delivery/handler"
	"github.com/skiphead/practicum/pkg/storage"
	"net/http"
	"net/http/httptest"

	"testing"
)

func TestNewServer(t *testing.T) {
	cfg := &config.Config{ServerAddr: "127.0.0.1:8080"}
	storage := storage.NewMemoryStorage()
	handler := handlers.NewURLHandler(storage, cfg.ServerAddr)

	server, err := NewServer(cfg, handler)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	if server.Addr != cfg.ServerAddr {
		t.Errorf("Expected server address '%s', got '%s'", cfg.ServerAddr, server.Addr)
	}
}

func TestNewServer_InvalidConfig(t *testing.T) {
	cfg := &config.Config{ServerAddr: "invalid-address"}
	storage := storage.NewMemoryStorage()
	handler := handlers.NewURLHandler(storage, cfg.ServerAddr)

	_, err := NewServer(cfg, handler)
	if err == nil {
		t.Error("Expected error for invalid config, got nil")
	}
}

func TestServer_Routing(t *testing.T) {
	cfg := &config.Config{ServerAddr: "127.0.0.1:8080"}
	storage := storage.NewMemoryStorage()
	handler := handlers.NewURLHandler(storage, cfg.ServerAddr)

	server, err := NewServer(cfg, handler)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Test server routing with a test request
	testServer := httptest.NewServer(server.Handler)
	defer testServer.Close()

	// Test POST request
	resp, err := http.Post(testServer.URL, "text/plain", bytes.NewBufferString("https://example.com"))
	if err != nil {
		t.Fatalf("Failed to make POST request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}
