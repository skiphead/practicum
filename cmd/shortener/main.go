// Main package for the URL shortening application.
// The application provides an HTTP server for processing URL shortening requests,
// using file storage or PostgreSQL database for data persistence.
package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/skiphead/practicum/infra/client/postgresql"
	"github.com/skiphead/practicum/infra/config"
	"github.com/skiphead/practicum/internal/audit"
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

	_ "net/http/pprof"
)

var buildVersion, buildDate, buildCommit string

// main is the entry point. Initializes components and starts the HTTP server.
func main() {
	printBuildInfo()

	logger := initLogger()
	defer func(logger *zap.Logger) {
		err := logger.Sync()
		if err != nil {
			logger.Fatal("failed to sync logger", zap.Error(err))
		}
	}(logger)

	cfg := loadConfig()
	auditClient := initAudit(cfg)

	store := initFileStorage(cfg)
	storageRepo := initDatabase(cfg)

	h := handler.NewURLHandler(
		usecase.NewStorageUseCase(cfg.BaseURL, *store, *storageRepo),
		cfg.ServerAddr,
		cfg.BaseURL,
		cfg.SessionKey,
		auditClient)

	server := initServer(cfg, h)
	runServer(server)
}

// printBuildInfo displays version, date, and commit information.
func printBuildInfo() {
	if buildVersion == "" {
		buildVersion = "N/A"
	}
	if buildCommit == "" {
		buildCommit = "N/A"
	}
	if buildDate == "" {
		buildDate = "N/A"
	}

	fmt.Printf("Build version: %s\n", buildVersion)
	fmt.Printf("Build date: %s\n", buildDate)
	fmt.Printf("Build commit: %s\n", buildCommit)
}

// initLogger configures the global zap logger.
func initLogger() *zap.Logger {
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}
	zap.ReplaceGlobals(logger)
	return logger
}

// runServer starts the HTTP server and handles graceful shutdown.
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

// initFileStorage creates file-based URL storage.
func initFileStorage(cfg *config.Config) *repository.FileStorage {
	store, err := repository.NewCachedFileStorage(cfg.FileStoragePath)
	if err != nil {
		zap.L().Fatal("File storage initialization failed", zap.Error(err))
	}
	return &store
}

// initDatabase establishes PostgreSQL connection and runs migrations.
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

// initServer creates an HTTP server with Chi router.
func initServer(cfg *config.Config, handler *handler.URLHandler) *delivery.Server {
	srv, err := delivery.NewServerChi(cfg, handler.ChiMux())
	if err != nil {
		zap.L().Fatal("Server creation failed", zap.Error(err))
	}
	return srv
}

// loadConfig reads configuration from YAML file or uses defaults.
func loadConfig() *config.Config {
	pathConfig := "configs/config.json"

	var flagConfig string
	flag.StringVar(&flagConfig, "config", "", "Path to config file")
	flag.Parse()

	if flagConfig != "" {
		pathConfig = flagConfig
	}
	if env := os.Getenv("CONFIG"); env != "" {
		pathConfig = flagConfig
	}

	cfg, err := config.LoadConfig(pathConfig)
	if err != nil {
		cfg = config.NewDefaultConfig()
		zap.L().Info("Using default configuration after failed config load",
			zap.Error(err),
			zap.String("config_path", pathConfig))
	}

	if err = cfg.Validate(); err != nil {
		zap.L().Fatal("Configuration validation failed", zap.Error(err))
	}
	return cfg
}

// initAudit sets up the audit logging system.
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
