package delivery

import (
	"context"
	"github.com/skiphead/practicum/internal/config"
	handlers "github.com/skiphead/practicum/internal/delivery/handler"
	"log"
	"net/http"
	"time"
)

type Server struct {
	*http.Server
}

func NewServer(cfg *config.Config, handler *handlers.URLHandler) (*Server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", handler.HandleRequest)

	return &Server{
		&http.Server{
			Addr:         cfg.ServerAddr,
			Handler:      mux,
			ReadTimeout:  15 * time.Second, // Good practice to set timeouts
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
	}, nil
}

// Start runs the server in a goroutine and returns a channel to listen for errors.
func (s *Server) Start() <-chan error {
	serverError := make(chan error, 1)
	go func() {
		log.Printf("Server is running on http://%s", s.Addr)
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverError <- err
		}
		close(serverError)
	}()
	return serverError
}

// Shutdown gracefully stops the server with a context timeout.
func (s *Server) Shutdown(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	log.Println("Server is shutting down...")
	return s.Server.Shutdown(ctx)
}
