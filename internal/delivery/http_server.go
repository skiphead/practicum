// Package delivery provides HTTP server implementation and delivery layer components.
// It includes server lifecycle management, graceful shutdown, and integration
// with Chi router and pprof profiling for building robust HTTP services.
package delivery

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/skiphead/practicum/internal/infra/config"
	"go.uber.org/zap"

	_ "net/http/pprof" // Import pprof for profiling endpoints
)

// Server represents an HTTP server with Chi router and graceful shutdown capabilities.
// It encapsulates the standard http.Server with additional configuration
// and lifecycle management features.
type Server struct {
	*http.Server              // Embedded standard HTTP server
	pprofServer  *http.Server // Pprof server for profiling
	tlsEnabled   bool
	pathCert     string
	pathKey      string
}

// NewServerChi creates and configures a new HTTP server with Chi router.
// It validates the configuration and sets up server timeouts for reliability.
//
// Parameters:
//   - cfg: Server configuration including address, timeouts, and other settings
//   - mux: Chi router instance with configured routes and middleware
//
// Returns:
//   - *Server: Configured server instance ready to start
//   - error: If configuration validation fails
//
// Server timeouts are set to:
//   - ReadTimeout: 15 seconds - maximum duration for reading the entire request
//   - WriteTimeout: 15 seconds - maximum duration for writing the response
//   - IdleTimeout: 60 seconds - maximum idle connection keep-alive duration
func NewServerChi(cfg *config.Config, mux *chi.Mux) (*Server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	pprofPort := cfg.PprofPort
	if pprofPort == "" {
		pprofPort = ":8081"
	}

	return &Server{
		Server: &http.Server{
			Addr:         cfg.ServerAddr,
			Handler:      mux,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		pprofServer: &http.Server{
			Addr:         pprofPort, // Pprof server port from config with fallback
			Handler:      nil,       // Use default http.DefaultServeMux
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
		tlsEnabled: cfg.EnableTLS,
		pathCert:   cfg.PathCert,
		pathKey:    cfg.PathKey,
	}, nil
}

// Start begins listening for HTTP requests on the configured address.
// It also starts a separate pprof profiler server for debugging.
// The method returns a channel that will receive any server errors.
//
// Returns:
//   - <-chan error: Channel that emits server errors (buffered, capacity 1)
//     The channel is closed when both servers stop, either by error or shutdown.
func (s *Server) Start() <-chan error {
	serverError := make(chan error, 1) // Емкость 1 достаточна для первой критической ошибки
	var wg sync.WaitGroup
	wg.Add(2)

	// Pprof server
	go func() {
		defer wg.Done()
		err := s.pprofServer.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			select {
			case serverError <- err: // Неблокирующая отправка
			default: // Канал уже содержит ошибку — игнорируем
			}
		}
	}()

	// Main server
	go func() {
		defer wg.Done()
		var err error
		if s.tlsEnabled {
			err = s.Server.ListenAndServeTLS(s.pathCert, s.pathKey)
		} else {
			err = s.Server.ListenAndServe()
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			select {
			case serverError <- err:
			default:
			}
		}
	}()

	// Coordinator: закрываем канал после завершения обоих серверов
	go func() {
		wg.Wait()
		close(serverError)
	}()

	return serverError
}

// Shutdown gracefully stops the HTTP server with the specified timeout.
// It allows in-flight requests to complete while preventing new connections.
// Also stops the pprof profiling server.
//
// Parameters:
//   - timeout: Maximum duration to wait for graceful shutdown
//
// Returns:
//   - error: If shutdown exceeds timeout or encounters other errors
//
// The method:
// 1. Creates a context with the specified timeout
// 2. Calls Shutdown on both main and pprof servers
// 3. Logs the shutdown event for monitoring
//
// Note: After shutdown is initiated, both servers will stop accepting new connections
// but will allow existing requests to complete within the timeout period.
func (s *Server) Shutdown(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var mainErr, pprofErr error

	if err := s.Server.Shutdown(ctx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		mainErr = fmt.Errorf("main server shutdown: %w", err)
	}

	if err := s.pprofServer.Shutdown(ctx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		pprofErr = fmt.Errorf("pprof server shutdown: %w", err)
	}

	if mainErr == nil && pprofErr == nil {
		zap.L().Info("Servers shut down gracefully")
		return nil
	}

	return errors.Join(mainErr, pprofErr)
}

// GetAddr returns the address the main server is listening on.
// Useful for testing to determine the actual port when using ":0".
func (s *Server) GetAddr() string {
	return s.Server.Addr
}
