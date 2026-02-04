// main.go
// Main package for the URL shortening application.
// The application provides an HTTP server for processing URL shortening requests,
// using file storage or PostgreSQL database for data persistence.
package main

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/skiphead/practicum/infra/client/postgresql"
	"github.com/skiphead/practicum/infra/config"
	"github.com/skiphead/practicum/internal/audit"
	"github.com/skiphead/practicum/internal/delivery"
	"github.com/skiphead/practicum/internal/delivery/handler"

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

// main - entry point of the application.
// Initializes application components and starts the server.
// Initialization sequence:
// 1. Logger
// 2. Configuration
// 3. Storage (file and database)
// 4. HTTP request handlers
// 5. HTTP server
func main() {
	// Initialize logger
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

	// Load configuration
	cfg := loadConfig()

	auditClient := initAudit(cfg)

	// Initialize storage
	store := initFileStorage(cfg)
	storageRepo := initDatabase(cfg)

	// Create URL handler
	h := handler.NewURLHandler(
		usecase.NewStorageUseCase(cfg.BaseURL, *store, *storageRepo),
		cfg.ServerAddr,
		cfg.BaseURL,
		cfg.SessionKey,
		auditClient)

	// Initialize server
	server := initServer(cfg, h)

	// Start server
	runServer(server)
}

// runServer manages the HTTP server lifecycle.
// Starts the server in a separate goroutine and handles shutdown signals.
// Parameters:
// - server: HTTP server instance to manage
// Algorithm:
// - Starts server in a separate channel for error handling
// - Waits for OS Interrupt or SIGTERM signals
// - Performs graceful shutdown with 10-second timeout
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
		os.Exit(1)
	} else {
		zap.L().Info("Server shutdown completed")
		os.Exit(0)
	}
}

// initFileStorage initializes file storage for URLs.
// Parameters:
//   - cfg: application configuration containing file storage path
//
// Returns:
//   - pointer to initialized file storage
//   - terminates application on initialization error
func initFileStorage(cfg *config.Config) *repository.FileStorage {
	store, err := repository.NewCachedFileStorage(cfg.FileStoragePath)
	if err != nil {
		zap.L().Fatal("File storage initialization failed", zap.Error(err))
	}
	return &store
}

// initDatabase initializes PostgreSQL connection and applies migrations.
// Parameters:
//   - cfg: application configuration with database DSN connection string
//
// Returns:
//   - pointer to URL repository or nil on error
//
// Actions:
//  1. Establishes connection with database connection pool
//  2. Checks connection via ping
//  3. Applies migrations through standard database/sql library
//  4. Creates repository for URL operations
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

// initServer creates an HTTP server instance using the Chi framework.
// Parameters:
//   - cfg: server configuration
//   - handler: HTTP request handler
//
// Returns:
//   - configured server instance
//   - terminates application on server creation error
func initServer(cfg *config.Config, handler *handler.URLHandler) *delivery.Server {
	srv, err := delivery.NewServerChi(cfg, handler.ChiMux())
	if err != nil {
		zap.L().Fatal("Server creation failed", zap.Error(err))
	}
	return srv
}

// loadConfig loads application configuration from YAML file.
// Returns:
//   - pointer to loaded configuration
//
// Logic:
//   - Attempts to load configuration from configs/config.yaml file
//   - On error uses default configuration
//   - Validates required parameters
//   - Terminates application on validation error
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

// initAudit initializes the audit system for logging URL operations.
// Parameters:
//   - cfg: application configuration with audit settings
//
// Returns:
//   - pointer to initialized audit adapter
//   - terminates application on initialization error
//
// Creates audit adapter with:
//   - File logging to specified audit file
//   - HTTP endpoint for remote audit logging
//   - Batch processing for efficient event delivery
//   - Asynchronous queue for non-blocking operations
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
