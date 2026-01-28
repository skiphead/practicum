package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"go.uber.org/zap"
)

func (h *URLHandler) processAndSaveURL(originalURL string, w http.ResponseWriter, r *http.Request) (string, bool, error) {
	if err := h.validateURL(originalURL, w); err != nil {
		return "", false, err
	}

	userID := h.getUserIDFromContext(r.Context())
	if userID == "" {
		http.Error(w, "User not found", http.StatusUnauthorized)
		return "", false, fmt.Errorf("user not found")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	resp, err := h.storage.Save(ctx, originalURL, userID)
	if err != nil {
		if h.storage.IsDuplicateError(err) {
			return h.buildShortURL(resp.ShortCode), true, nil
		}
		zap.L().Error("save error", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return "", false, err
	}

	return h.buildShortURL(resp.ShortCode), false, nil
}

func (h *URLHandler) readRequestBody(r *http.Request) ([]byte, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	defer h.closeBody(r.Body)
	return body, nil
}

func (h *URLHandler) validateURL(originalURL string, w http.ResponseWriter) error {
	if originalURL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return fmt.Errorf("URL is required")
	}

	// Парсим URL для более детальной проверки
	u, err := url.Parse(originalURL)
	if err != nil {
		http.Error(w, "Invalid URL format", http.StatusBadRequest)
		return fmt.Errorf("invalid URL format: %v", err)
	}

	// Проверяем наличие схемы (протокола)
	if u.Scheme == "" {
		http.Error(w, "URL scheme (protocol) is required", http.StatusBadRequest)
		return fmt.Errorf("URL scheme is required")
	}

	// Проверяем наличие хоста
	if u.Host == "" {
		http.Error(w, "URL host is required", http.StatusBadRequest)
		return fmt.Errorf("URL host is required")
	}

	// Проверяем допустимые протоколы (только HTTP и HTTPS)
	if u.Scheme != "http" && u.Scheme != "https" {
		http.Error(w, "Only HTTP and HTTPS protocols are allowed", http.StatusBadRequest)
		return fmt.Errorf("invalid URL scheme: %s (only http and https are allowed)", u.Scheme)
	}

	return nil
}

func (h *URLHandler) buildShortURL(key string) string {
	return fmt.Sprintf("%s/%s", h.baseURL, key)
}

func (h *URLHandler) closeBody(body io.ReadCloser) {
	if err := body.Close(); err != nil {
		zap.L().Error("error close Body", zap.Error(err))
	}
}

func (h *URLHandler) handleConflictError(w http.ResponseWriter, err error) bool {
	if h.storage.IsDuplicateError(err) {
		http.Error(w, err.Error(), http.StatusConflict)
		return true
	}
	return false
}
