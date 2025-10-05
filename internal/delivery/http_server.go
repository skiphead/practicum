package delivery

import (
	"context"
	"errors"
	"github.com/go-chi/chi/v5"
	"log"
	"net/http"
	"time"

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

func NewNativeServer(cfg *config.Config, mux *http.ServeMux) (*Server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &Server{
		&http.Server{
			Addr:         cfg.ServerAddr,   // Адрес сервера из конфигурации
			Handler:      mux,              // HTTP-роутер
			ReadTimeout:  15 * time.Second, // Таймаут на чтение запроса
			WriteTimeout: 15 * time.Second, // Таймаут на запись ответа
			IdleTimeout:  60 * time.Second, // Таймаут для keep-alive соединений
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
