package delivery

import (
	"context"
	"errors"
	"github.com/skiphead/practicum/infra/config"
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
	mux.HandleFunc("/", handler.HandleRequest) // Регистрирует обработчик для всех путей

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
