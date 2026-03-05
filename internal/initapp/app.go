// Package initapp provides application initialization and lifecycle management.
package initapp

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	pb "github.com/skiphead/practicum/pkg/api/v1/gen"
	"go.uber.org/zap"

	"github.com/skiphead/practicum/internal/audit"
	"github.com/skiphead/practicum/internal/delivery"
	"github.com/skiphead/practicum/internal/delivery/handler"
	"github.com/skiphead/practicum/internal/domain/entity"
	"github.com/skiphead/practicum/internal/domain/repository"
	"github.com/skiphead/practicum/internal/infra/client/postgresql"
	config2 "github.com/skiphead/practicum/internal/infra/config"
	"github.com/skiphead/practicum/internal/usecase"
)

// App holds all initialized application components.
type App struct {
	logger     *zap.Logger
	cfg        *config2.Config
	httpServer *delivery.Server
	grpcServer *delivery.ServerGRPC
	audit      *audit.Adapter
	closers    []func() error
}

// NewApp initializes the application with all its dependencies.
func NewApp(logger *zap.Logger, cfg *config2.Config) (*App, error) {
	app := &App{
		logger:  logger,
		cfg:     cfg,
		closers: make([]func() error, 0),
	}

	if err := app.initComponents(); err != nil {
		return nil, fmt.Errorf("failed to initialize components: %w", err)
	}

	return app, nil
}

// initComponents initializes all application components in proper order.
func (a *App) initComponents() error {
	// Initialize audit
	a.audit = initAudit(a.cfg, a.logger)

	// Initialize storage
	store := initFileStorage(a.cfg, a.logger)
	a.registerCloser(func() error {
		// Add cleanup logic for file storage if needed
		return nil
	})

	// Initialize database
	storageRepo, dbClose := initDatabase(a.cfg, a.logger)
	if dbClose != nil {
		a.registerCloser(dbClose)
	}

	// Initialize IP checker repository and use case
	subnet, err := entity.NewSubnetEntity(a.cfg.TrustedSubnet)
	if err != nil {
		return fmt.Errorf("failed to parse trusted_subnet: %w", err)
	}
	ipCheckerRepo := repository.NewIPCheckerRepository(subnet)
	ipCheckerUseCase := usecase.NewIPCheckerUseCase(ipCheckerRepo)

	storageUseCase := usecase.NewStorageUseCase(a.cfg.BaseURL, *store, *storageRepo)

	// Initialize handler
	h := handler.NewURLHandler(
		storageUseCase,
		ipCheckerUseCase,
		a.cfg.ServerAddr,
		a.cfg.BaseURL,
		a.cfg.SessionKey,
		a.audit,
		a.logger,
	)

	// Initialize httpServer
	srv, err := delivery.NewServerChi(a.cfg, h.ChiMux())
	if err != nil {
		return fmt.Errorf("httpServer creation failed: %w", err)
	}
	a.httpServer = srv

	if err := a.initGRPCServer(storageUseCase); err != nil {
		return fmt.Errorf("gRPC httpServer initialization failed: %w", err)
	}
	return nil
}

// registerCloser adds a cleanup function to be called on shutdown.
func (a *App) registerCloser(fn func() error) {
	a.closers = append(a.closers, fn)
}

// Run starts both HTTP and gRPC servers and handles graceful shutdown.
func (a *App) Run() {
	// Канал для ошибок HTTP сервера
	httpErrChan := a.httpServer.Start()

	// Канал для ошибок gRPC сервера
	var grpcErrChan <-chan error
	if a.grpcServer != nil {
		grpcErrChan = a.startGRPCServer()
	}

	// Объединяем каналы ошибок
	errChan := mergeErrorChannels(httpErrChan, grpcErrChan)

	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt,    // SIGINT
		syscall.SIGTERM, // SIGTERM
		syscall.SIGQUIT) // SIGQUIT
	defer stop()

	select {
	case <-ctx.Done():
		a.logger.Info("Received shutdown signal")
	case err := <-errChan:
		if err != nil {
			a.logger.Error("Server error", zap.Error(err))
		}
	}

	a.shutdown()
	a.grpcServer.GracefulStop()
}

// mergeErrorChannels объединяет несколько каналов ошибок в один
func mergeErrorChannels(channels ...<-chan error) <-chan error {
	out := make(chan error, len(channels))

	for _, ch := range channels {
		if ch == nil {
			continue
		}

		go func(c <-chan error) {
			if err := <-c; err != nil {
				out <- err
			}
		}(ch)
	}

	return out
}

// shutdown performs graceful shutdown of all components.
func (a *App) shutdown() {
	// Shutdown httpServer
	if err := a.httpServer.Shutdown(10 * time.Second); err != nil {
		a.logger.Error("Server shutdown error", zap.Error(err))
	} else {
		a.logger.Info("Server shutdown completed")
	}

	// Call all registered closers
	for i := len(a.closers) - 1; i >= 0; i-- {
		if err := a.closers[i](); err != nil {
			a.logger.Error("Cleanup error", zap.Error(err))
		}
	}

}

// initAudit sets up the audit logging system.
func initAudit(cfg *config2.Config, logger *zap.Logger) *audit.Adapter {
	auditCfg := audit.Config{
		FilePath:     cfg.AuditFile,
		HTTPEndpoint: cfg.AuditURL,
		Enabled:      true,
		MaxBatchSize: 10000,
		QueueSize:    10,
	}

	adapter, err := audit.NewAdapter(auditCfg)
	if err != nil {
		logger.Fatal("Failed to create audit adapter", zap.Error(err))
	}

	logger.Info("Audit system initialized",
		zap.String("file_path", cfg.AuditFile),
		zap.String("http_endpoint", cfg.AuditURL),
	)

	return adapter
}

// initFileStorage creates file-based URL storage.
func initFileStorage(cfg *config2.Config, logger *zap.Logger) *repository.FileStorage {
	store, err := repository.NewCachedFileStorage(cfg.FileStoragePath)
	if err != nil {
		logger.Fatal("File storage initialization failed", zap.Error(err))
	}
	return &store
}

// initDatabase establishes PostgreSQL connection and runs migrations.
// Returns repository and optional closer function.
func initDatabase(cfg *config2.Config, logger *zap.Logger) (*repository.URLRepository, func() error) {
	pool, connErr := pgxpool.New(context.Background(), cfg.DatabaseDSN)
	if connErr != nil {
		logger.Error("pgxpool initialization failed", zap.Error(connErr))
		return nil, nil
	}

	if pool.Ping(context.Background()) == nil {
		db := stdlib.OpenDBFromPool(pool)
		if err := postgresql.Migrations(db, "migrations"); err != nil {
			logger.Error("postgresql migration failed", zap.Error(err))
		}
	}

	repo := repository.NewStorageRepository(pool)

	closer := func() error {
		pool.Close()
		return nil
	}

	return &repo, closer
}

// initGRPCServer initializes and configures the gRPC server
func (a *App) initGRPCServer(storageUseCase usecase.URLUseCase) error {
	// Создаем конфигурацию gRPC сервера
	grpcCfg := &delivery.ServerConfig{
		Port:       a.cfg.GRPCPort,
		SessionKey: a.cfg.SessionKey,
		TLS: &delivery.TLSConfig{
			Enabled: a.cfg.GRPCTLSEnabled,
			Cert:    a.cfg.GRPCCertFile,
			Key:     a.cfg.GRPCKeyFile,
		},
	}

	// Создаем gRPC сервер
	grpcSrv, err := delivery.NewServer(grpcCfg, a.logger)
	if err != nil {
		return fmt.Errorf("failed to create gRPC server: %w", err)
	}

	// Создаем gRPC хендлер для shortener сервиса
	shortenerHandler := handler.NewShortenerHandler(
		storageUseCase,
		a.cfg.BaseURL,
		a.audit,
		a.logger,
	)

	// Регистрируем сервис
	pb.RegisterShortenerServiceServer(grpcSrv.GetGRPCServer(), shortenerHandler)
	a.logger.Info("gRPC shortener service registered",
		zap.String("service", "ShortenerService"),
	)

	authHandler := handler.NewAuthHandler(a.audit, a.logger)

	pb.RegisterAuthServiceServer(grpcSrv.GetGRPCServer(), authHandler)
	a.logger.Info("gRPC auth service registered", zap.String("service", "AuthService"))

	// Сохраняем gRPC сервер в приложении
	a.grpcServer = grpcSrv
	a.registerCloser(func() error {
		grpcSrv.GracefulStop()
		return nil
	})

	return nil
}

// startGRPCServer запускает gRPC сервер и возвращает канал ошибок
func (a *App) startGRPCServer() <-chan error {
	errChan := make(chan error, 1)

	go func() {
		defer close(errChan)

		addr, err := a.grpcServer.Run()
		if err != nil {
			errChan <- fmt.Errorf("gRPC httpServer failed to start: %w", err)
			return
		}

		a.logger.Info("gRPC server started successfully",
			zap.String("address", addr.String()),
		)
	}()

	return errChan
}
