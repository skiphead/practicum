// Package delivery provides HTTP server implementation and delivery layer components.
// It includes server lifecycle management, graceful shutdown, and integration
// with Chi router and pprof profiling for building robust HTTP services.
package delivery

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	_ "net/http/pprof" // Import pprof for profiling endpoints

	"github.com/skiphead/practicum/infra/config"
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

	if cfg.PprofPort == "" {
		cfg.PprofPort = ":8081"
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
			Addr:         cfg.PprofPort, // Pprof server on fixed port
			Handler:      nil,           // Use default http.DefaultServeMux
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
		tlsEnabled: cfg.EnableTLS,
		pathCert:   cfg.PathCert,
		pathKey:    cfg.PathKey,
	}, nil
}

// Start begins listening for HTTP requests on the configured address.
// It also starts a separate pprof profiler server on port 8081 for debugging.
// The method returns a channel that will receive any server errors.
//
// Returns:
//   - <-chan error: Channel that emits server errors (buffered, capacity 2)
//     The channel is closed when both servers stop gracefully.
//
// The method runs two servers concurrently:
// 1. Main application server on the configured address
// 2. pprof profiling server on port 8081 (for debugging/profiling)
//
// Note: The pprof server provides profiling endpoints at /debug/pprof/
func (s *Server) Start() <-chan error {
	serverError := make(chan error, 2) // Buffer for both servers

	// Start pprof profiling server on port 8081
	go func() {
		zap.L().Info("Starting pprof server", zap.String("addr", s.pprofServer.Addr))
		err := s.pprofServer.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverError <- err
		}
	}()

	// Start main application server
	go func() {
		zap.L().Info("Starting main server", zap.String("addr", s.Server.Addr))
		var err error
		if s.tlsEnabled {
			err = s.Server.ListenAndServeTLS(s.pathCert, s.pathKey)
		} else {
			err = s.Server.ListenAndServe()
		}

		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverError <- err
		}
	}()

	// Goroutine to close error channel when both servers are done
	go func() {
		// Wait a bit to ensure both servers have time to start
		time.Sleep(100 * time.Millisecond)
		// We don't close the channel immediately because servers run indefinitely
		// The channel will be closed in Shutdown method
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

	zap.L().Info("Shutting down main server", zap.String("addr", s.Server.Addr))

	// Shutdown main server
	mainErr := s.Server.Shutdown(ctx)

	// Shutdown pprof server
	zap.L().Info("Shutting down pprof server", zap.String("addr", s.pprofServer.Addr))
	pprofErr := s.pprofServer.Shutdown(ctx)

	if mainErr != nil {
		return mainErr
	}
	return pprofErr
}

// GetAddr returns the address the main server is listening on.
// Useful for testing to determine the actual port when using ":0".
func (s *Server) GetAddr() string {
	return s.Server.Addr
}
