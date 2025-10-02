package main

import (
	"context"
	"github.com/skiphead/practicum/internal/config"
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
	cfg := config.NewDefaultConfig()
	store := storage.NewMemoryStorage()
	handler := handlers.NewURLHandler(store, cfg.ServerAddr)

	srv, err := delivery.NewServer(cfg, handler)
	if err != nil {
		log.Fatal("Error creating server:", err)
	}

	// Start the server and listen for errors
	serverErr := srv.Start()

	// Listen for termination signals
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Block until a signal is received or the server has an error
	select {
	case <-ctx.Done(): // A signal was received
		log.Println("Received shutdown signal")
	case err = <-serverErr: // The server encountered an error on startup
		log.Fatalf("Server error: %v", err)
	}

	if err = srv.Shutdown(10 * time.Second); err != nil {
		log.Printf("Server shutdown error: %v", err)
		// If shutdown fails, you might want to force close
	}

	log.Println("Server exited properly")
}
