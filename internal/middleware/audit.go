package middleware

import (
	"context"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/skiphead/practicum/internal/audit"
	"go.uber.org/zap"
	"net/http"
	"strings"
	"time"
)

// AuditMiddleware middleware для аудита запросов
type AuditMiddleware struct {
	auditAdapter *audit.Adapter
	logger       *zap.Logger
}

// NewAuditMiddleware создает новый middleware аудита
func NewAuditMiddleware(adapter *audit.Adapter, logger *zap.Logger) *AuditMiddleware {
	return &AuditMiddleware{
		auditAdapter: adapter,
		logger:       logger,
	}
}

// Wrap оборачивает хендлер с аудитом
func (am *AuditMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Используем WrapResponseWriter
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		// Засекаем время начала
		start := time.Now()

		// Выполняем хендлер
		next.ServeHTTP(ww, r)

		// Вычисляем длительность
		duration := time.Since(start)

		// Логируем успешные запросы (2xx, 3xx)
		if ww.Status() >= 200 && ww.Status() < 400 {
			am.logAuditEvent(r, ww.Status(), start, duration)
		}

		// Логируем через zap
		am.logRequest(r, ww.Status(), duration)
	})
}

// logAuditEvent создает и отправляет событие аудита
func (am *AuditMiddleware) logAuditEvent(r *http.Request, statusCode int, start time.Time, duration time.Duration) {
	// Определяем действие
	action := am.determineAction(r, statusCode)
	if action == "" {
		return
	}

	// Извлекаем данные
	userID := am.extractUserID(r)
	url := am.extractURL(r, action)

	// Создаем событие
	event := audit.NewEvent(action, userID, url)

	// Отправляем асинхронно
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if err := am.auditAdapter.LogEvent(ctx, event); err != nil {
		am.logger.Warn("Failed to send audit event",
			zap.String("action", action),
			zap.Error(err))
	}
}

// determineAction определяет тип действия
func (am *AuditMiddleware) determineAction(r *http.Request, statusCode int) string {
	path := r.URL.Path
	method := r.Method

	switch {
	case path == "/" && method == "POST":
		return "shorten"
	case path == "/api/shorten" && method == "POST":
		return "shorten"
	case strings.HasPrefix(path, "/api/shorten/batch") && method == "POST":
		return "shorten_batch"
	case len(path) > 1 && path[0] == '/' && !strings.Contains(path[1:], "/") && method == "GET":
		return "follow"
	default:
		return ""
	}
}

// extractUserID извлекает идентификатор пользователя
func (am *AuditMiddleware) extractUserID(r *http.Request) string {
	// Из заголовков
	if userID := r.Header.Get("X-User-ID"); userID != "" {
		return userID
	}

	// Из query параметров
	if userID := r.URL.Query().Get("user_id"); userID != "" {
		return userID
	}

	// Из Authorization заголовка
	if authHeader := r.Header.Get("Authorization"); authHeader != "" {
		if strings.HasPrefix(authHeader, "Bearer ") {
			// В реальной реализации здесь был бы парсинг JWT
			return "authenticated_user"
		}
	}

	// По умолчанию - IP адрес
	return am.extractClientIP(r)
}

// extractURL извлекает URL
func (am *AuditMiddleware) extractURL(r *http.Request, action string) string {
	// Для создания коротких ссылок пытаемся получить оригинальный URL
	if action == "shorten" {
		// Для JSON запросов
		if r.Header.Get("Content-Type") == "application/json" {
			// В реальной реализации нужно прочитать и распарсить тело
			// Для примера возвращаем заглушку
			return r.URL.String() + " [from JSON body]"
		}

		// Для form-data
		if url := r.FormValue("url"); url != "" {
			return url
		}
	}

	// Для переходов по ссылкам и других действий
	return r.URL.String()
}

// extractClientIP извлекает IP клиента
func (am *AuditMiddleware) extractClientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		ips := strings.Split(ip, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}

	return strings.Split(r.RemoteAddr, ":")[0]
}

// logRequest логирует запрос через zap
func (am *AuditMiddleware) logRequest(r *http.Request, statusCode int, duration time.Duration) {
	am.logger.Info("request",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
		zap.Int("status", statusCode),
		zap.Duration("duration", duration),
		zap.String("user_agent", r.UserAgent()))
}
