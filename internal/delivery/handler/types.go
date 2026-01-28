package handler

import (
	"github.com/skiphead/practicum/internal/audit"
	"github.com/skiphead/practicum/internal/usecase"
)

type URLHandler struct {
	storage     usecase.URLUseCase
	auditClient *audit.Adapter
	serverAddr  string
	baseURL     string
	sessionKey  string
}

func NewURLHandler(storage usecase.URLUseCase, serverAddr, baseURL, sessionKey string, auditClient *audit.Adapter) *URLHandler {
	return &URLHandler{
		storage:     storage,
		serverAddr:  serverAddr,
		baseURL:     baseURL,
		sessionKey:  sessionKey,
		auditClient: auditClient,
	}
}
