package delivery

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	_ "net/http/pprof"

	"github.com/skiphead/practicum/infra/config"
)

type Server struct {
	*http.Server
}

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
	}, nil
}

func (s *Server) Start() <-chan error {
	go http.ListenAndServe(":8081", nil)

	serverError := make(chan error, 1)

	go func() {
		zap.L().Info("Starting server", zap.String("addr", s.Server.Addr))
		if err := s.Server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverError <- err
		}
		close(serverError)
	}()
	return serverError
}

func (s *Server) Shutdown(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	zap.L().Info("Shutting down server", zap.String("addr", s.Server.Addr))
	return s.Server.Shutdown(ctx)
}
