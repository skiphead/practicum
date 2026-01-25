package handler

import (
	"encoding/json"
	"github.com/skiphead/practicum/internal/domain"
	"net/http"
	"time"
)

// AuditHandler - обработчик аудита
type AuditHandler struct {
	publisher domain.Publisher
}

func NewAuditHandler(publisher domain.Publisher) *AuditHandler {
	return &AuditHandler{publisher: publisher}
}

// HandleShorten - обработчик события создания короткой ссылки
func (ah *AuditHandler) HandleShorten(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	url := r.URL.Query().Get("url")

	if url == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	event := domain.AuditEvent{
		Timestamp: time.Now().Unix(),
		Action:    "shorten",
		UserID:    userID,
		URL:       url,
	}

	ah.publisher.Notify(event)

	// Здесь можно вернуть короткую ссылку
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ok",
		"short_url": generateShortURL(url),
		"original":  url,
		"timestamp": event.Timestamp,
	})
}

// HandleFollow - обработчик события перехода по короткой ссылке
func (ah *AuditHandler) HandleFollow(w http.ResponseWriter, r *http.Request) {
	shortURL := r.URL.Path[len("/follow/"):]
	userID := getUserID(r)

	// Здесь можно получить оригинальный URL по короткой ссылке
	originalURL := resolveShortURL(shortURL)
	if originalURL == "" {
		http.Error(w, "URL not found", http.StatusNotFound)
		return
	}

	event := domain.AuditEvent{
		Timestamp: time.Now().Unix(),
		Action:    "follow",
		UserID:    userID,
		URL:       originalURL,
	}

	ah.publisher.Notify(event)

	// Перенаправляем на оригинальный URL
	http.Redirect(w, r, originalURL, http.StatusFound)
}

// Вспомогательные функции
func getUserID(r *http.Request) string {
	// Здесь можно получить user_id из:
	// 1. Токена авторизации
	// 2. Сессии
	// 3. Параметра запроса
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		// Из заголовка Authorization
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			// Парсим токен
			return extractUserIDFromToken(authHeader)
		}
	}
	return userID
}

func extractUserIDFromToken(token string) string {
	// Реализация извлечения user_id из токена
	return ""
}

func generateShortURL(originalURL string) string {
	// Генерация короткой ссылки
	return "https://short.ly/abc123"
}

func resolveShortURL(shortURL string) string {
	// Разрешение короткой ссылки в оригинальную
	// Здесь должна быть логика поиска в БД
	return "https://original-long-domain.com/path/to/page"
}
