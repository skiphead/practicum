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
	*http.Server // Embedded standard HTTP server
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

	return &Server{
		&http.Server{
			Addr:         cfg.ServerAddr,
			Handler:      mux,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		cfg.EnableTLS,
		cfg.PathCert,
		cfg.PathKey,
	}, nil
}

// Start begins listening for HTTP requests on the configured address.
// It also starts a separate pprof profiler server on port 8081 for debugging.
// The method returns a channel that will receive any server errors.
//
// Returns:
//   - <-chan error: Channel that emits server errors (buffered, capacity 1)
//     The channel is closed when the server stops gracefully.
//
// The method runs two servers concurrently:
// 1. Main application server on the configured address
// 2. pprof profiling server on port 8081 (for debugging/profiling)
//
// Note: The pprof server provides profiling endpoints at /debug/pprof/
func (s *Server) Start() <-chan error {

	serverError := make(chan error, 1)

	// Start pprof profiling server on port 8081
	go func() {
		err := http.ListenAndServe(":8081", nil)
		if err != nil {
			serverError <- err
		}
	}()

	go func() {
		zap.L().Info("Starting server", zap.String("addr", s.Server.Addr))
		var err error
		if s.tlsEnabled {
			err = s.Server.ListenAndServeTLS(s.pathCert, s.pathKey)
		} else {
			err = s.Server.ListenAndServe()
		}

		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverError <- err
		}
		close(serverError)
	}()
	return serverError
}

// Shutdown gracefully stops the HTTP server with the specified timeout.
// It allows in-flight requests to complete while preventing new connections.
//
// Parameters:
//   - timeout: Maximum duration to wait for graceful shutdown
//
// Returns:
//   - error: If shutdown exceeds timeout or encounters other errors
//
// The method:
// 1. Creates a context with the specified timeout
// 2. Calls the standard http.Server.Shutdown() method
// 3. Logs the shutdown event for monitoring
//
// Note: After shutdown is initiated, the server will stop accepting new connections
// but will allow existing requests to complete within the timeout period.
func (s *Server) Shutdown(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	zap.L().Info("Shutting down server", zap.String("addr", s.Server.Addr))
	return s.Server.Shutdown(ctx)
}
