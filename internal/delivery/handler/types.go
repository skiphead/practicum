package handlers

import "github.com/skiphead/practicum/internal/usecase"

type URLHandler struct {
	storage    usecase.URLUseCase
	serverAddr string
	baseURL    string
	sessionKey string
}

func NewURLHandler(storage usecase.URLUseCase, serverAddr, baseURL, sessionKey string) *URLHandler {
	return &URLHandler{
		storage:    storage,
		serverAddr: serverAddr,
		baseURL:    baseURL,
		sessionKey: sessionKey,
	}
}
