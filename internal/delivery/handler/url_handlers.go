package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/render"
	"github.com/skiphead/practicum/internal/audit"
	"github.com/skiphead/practicum/internal/domain/entity"
	"go.uber.org/zap"
)

func (h *URLHandler) createShortAPIURL(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return
	}

	body, err := h.readRequestBody(r)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var original entity.ShortenRequest
	if err := json.Unmarshal(body, &original); err != nil {
		zap.L().Error("unmarshal error", zap.Error(err))
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	shortURL, isConflict, err := h.processAndSaveURL(original.URL, w, r)
	if err != nil {
		return
	}

	if isConflict {
		render.Status(r, http.StatusConflict)
		render.JSON(w, r, map[string]string{"result": shortURL})
		return
	}

	errAuditClient := h.auditClient.LogEvent(context.Background(), &audit.Event{
		Timestamp: time.Now().Unix(),
		Action:    "shorten",
		UserID:    "",
		URL:       original.URL,
	})
	if errAuditClient != nil {
		render.Status(r, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	render.JSON(w, r, map[string]string{"result": shortURL})
}

func (h *URLHandler) createShortURL(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	// Проверяем метод
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := h.readRequestBody(r)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	originalURL := string(body)

	// Важно: проверяем, что URL не пустой
	if originalURL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	// Простая проверка - есть ли протокол
	if !strings.Contains(originalURL, "://") && !strings.HasPrefix(originalURL, "http") {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	shortURL, isConflict, err := h.processAndSaveURL(originalURL, w, r)
	if err != nil {
		return
	}

	if isConflict {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusConflict)
		_, err = w.Write([]byte(shortURL))
		if err != nil {
			zap.L().Error("write error", zap.Error(err))
		}
		return
	}

	errAuditClient := h.auditClient.LogEvent(context.Background(), &audit.Event{
		Timestamp: time.Now().Unix(),
		Action:    "shorten",
		UserID:    "",
		URL:       originalURL,
	})
	if errAuditClient != nil {
		render.Status(r, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	_, err = w.Write([]byte(shortURL))
	if err != nil {
		zap.L().Error("write error", zap.Error(err))
		return
	}
}

func (h *URLHandler) redirectURL(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	key := r.URL.Path[1:]
	data, err := h.storage.Get(ctx, key)

	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if !data.IsActive {
		w.WriteHeader(http.StatusGone)
		return
	}

	errAuditClient := h.auditClient.LogEvent(context.Background(), &audit.Event{
		Timestamp: time.Now().Unix(),
		Action:    "follow",
		UserID:    data.UserID,
		URL:       data.OriginalURL,
	})
	if errAuditClient != nil {
		render.Status(r, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Location", data.OriginalURL)
	w.WriteHeader(http.StatusTemporaryRedirect)
}
