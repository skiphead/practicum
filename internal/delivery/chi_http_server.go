package delivery

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/skiphead/practicum/infra/config"
	handlers "github.com/skiphead/practicum/internal/delivery/handler"
)

type Server struct {
	*http.Server
}

func NewServer(cfg *config.Config, handler *handlers.URLHandler) (*Server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	r := chi.NewRouter()

	r.Get("/{key}", handler.RedirectURL)
	r.Post("/", handler.CreateShortURL)

	return &Server{
		&http.Server{
			Addr:         cfg.ServerAddr,
			Handler:      r,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
	}, nil
}

func (s *Server) Start() <-chan error {
	serverError := make(chan error, 1)
	go func() {
		log.Printf("Server is running on http://%s", s.Addr)
		if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverError <- err
		}
		close(serverError)
	}()
	return serverError
}

func (s *Server) Shutdown(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	log.Println("Server is shutting down...")
	return s.Server.Shutdown(ctx)
}
