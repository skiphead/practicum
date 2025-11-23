package handlers

import "github.com/skiphead/practicum/internal/usecase"

type URLHandler struct {
	storage    usecase.URLUseCase
	serverAddr string
	baseURL    string
}

func NewURLHandler(storage usecase.URLUseCase, serverAddr, baseURL string) *URLHandler {
	return &URLHandler{
		storage:    storage,
		serverAddr: serverAddr,
		baseURL:    baseURL,
	}
}
