package handler

import (
	"context"
	"net"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	mw "github.com/skiphead/practicum/internal/middleware"
	"go.uber.org/zap"
)

// Типизированные ключи контекста для избежания коллизий
type contextKey string

const (
	requestIDKey contextKey = "request_id"
	clientIPKey  contextKey = "client_ip"
	ipSourceKey  contextKey = "ip_source"
)

// ChiMux создает и настраивает Chi роутер с middleware и маршрутами для сервиса сокращения URL.
//
// Порядок middleware (применяются последовательно):
//  1. requestIDMiddleware - добавляет request ID в контекст для трассировки
//  2. LoggingMiddleware - логирует детали запроса (применяется ко всем маршрутам)
//  3. sessionMiddleware - управляет пользовательскими сессиями и аутентификацией через cookies
//  4. CompressionMiddleware - обрабатывает gzip сжатие для запросов/ответов
//
// Группы маршрутов:
//
//	Публичные маршруты (доступны всем):
//	  GET    /ping                       - проверка здоровья базы данных
//	  GET    /{key}                      - редирект на оригинальный URL по короткому ключу
//
//	Защищенные маршруты (с сессией и сжатием):
//	  GET    /api/user/urls               - получение всех URL текущего пользователя
//	  DELETE /api/user/urls                - удаление URL пользователя (пакетная операция)
//	  POST   /                             - создание короткого URL через форму
//	  POST   /api/shorten                   - создание короткого URL через JSON API
//	  POST   /api/shorten/batch              - создание нескольких коротких URL через batch API
//
//	Защищенные внутренние маршруты (требуют валидации IP):
//	  GET    /api/internal/stats            - получение статистики сервиса (требуется доверенный IP)
func (h *URLHandler) ChiMux() *chi.Mux {
	r := chi.NewRouter()

	// Глобальные middleware - применяются ко всем маршрутам
	r.Use(h.requestIDMiddleware) // Добавляет request ID для трассировки
	r.Use(mw.LoggingMiddleware)  // Логирует все запросы (включая /ping и /{key})

	// Маршруты без middleware сессии и сжатия
	r.Get("/ping", h.pingDB)       // GET /ping - проверка БД
	r.Get("/{key}", h.RedirectURL) // GET /{key} - редирект

	// Защищенные маршруты - с middleware сессии и сжатия
	r.Group(func(r chi.Router) {
		r.Use(h.sessionMiddleware)
		r.Use(mw.CompressionMiddleware)

		// Эндпоинты для создания URL
		r.Post("/", h.createShortURL)                          // POST / - создать короткий URL через форму
		r.Post("/api/shorten", h.CreateShortAPIURL)            // POST /api/shorten - создать через JSON API
		r.Post("/api/shorten/batch", h.createBatchShortAPIURL) // POST /api/shorten/batch - пакетное создание
	})

	r.Group(func(r chi.Router) {
		r.Use(h.sessionMiddleware)
		// Эндпоинты для работы с URL пользователя
		r.Get("/api/user/urls", h.getAPIUserUrls)       // GET /api/user/urls - получить URL пользователя
		r.Delete("/api/user/urls", h.deleteAPIUserUrls) // DELETE /api/user/urls - удалить URL пользователя
	})

	// Защищенные внутренние маршруты - требуют валидации IP
	r.Group(func(r chi.Router) {
		r.Use(h.IPCheckMiddleware) // Применяем валидацию IP только к этой группе
		r.Use(h.sessionMiddleware) // Сессия всё ещё нужна для контекста пользователя
		r.Use(mw.CompressionMiddleware)

		// Эндпоинт статистики, требующий валидации IP
		r.Get("/api/internal/stats", h.statsHandler)
	})

	return r
}

// requestIDMiddleware добавляет уникальный ID запроса в контекст для трассировки
func (h *URLHandler) requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.New().String()
		ctx := context.WithValue(r.Context(), requestIDKey, requestID)
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// getRequestID извлекает request ID из контекста с использованием типизированного ключа
func getRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return "unknown"
}

// parseRemoteAddr парсит RemoteAddr для извлечения IP-адреса с поддержкой IPv6
func parseRemoteAddr(remoteAddr string) string {
	// Пытаемся распарсить host:port
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		return host
	}

	// Если не получилось распарсить, возвращаем как есть (может быть уже IP без порта)
	// Это может случиться, если сервер настроен иначе или это IPv6-адрес в квадратных скобках
	if strings.HasPrefix(remoteAddr, "[") && strings.Contains(remoteAddr, "]") {
		// Пробуем извлечь IPv6-адрес из скобок
		end := strings.Index(remoteAddr, "]")
		if end > 1 {
			return remoteAddr[1:end]
		}
	}

	return remoteAddr
}

// IPCheckMiddleware проверяет, что IP клиента принадлежит доверенной подсети
// перед разрешением доступа к защищенным внутренним эндпоинтам.
//
// Middleware извлекает IP клиента из заголовка X-Real-IP, который должен
// устанавливаться обратным прокси или балансировщиком нагрузки. Если X-Real-IP
// отсутствует, используется RemoteAddr с правильным парсингом через net.SplitHostPort.
//
// Процесс:
//  1. Извлечение IP из заголовка X-Real-IP или парсинг RemoteAddr через net.SplitHostPort
//  2. Проверка формата IP и проверка по доверенной подсети через IPCheckerUseCase
//  3. Разрешение доступа если IP доверенный, иначе возврат 403 Forbidden
//
// Заголовки:
//   - X-Real-IP: Опциональный заголовок с реальным IP клиента (устанавливается прокси)
//
// Коды статуса:
//   - 200 OK: Запрос передается следующему обработчику (IP доверенный)
//   - 403 Forbidden: IP не в доверенной подсети
//   - 500 Internal Server Error: Ошибка при проверке IP
//
// Логирование:
//   - Использует zap логгер с контекстом запроса и структурированным логированием
//   - Логирует попытки доступа с IP клиента, результатом проверки и ID запроса
//   - X-Request-ID автоматически включается во все логи через контекст
func (h *URLHandler) IPCheckMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Получаем ID запроса из контекста для трассировки
		requestID := getRequestID(r.Context())

		// Получаем логгер с контекстом
		logger := h.logger.With(
			zap.String("request_id", requestID),
			zap.String("middleware", "ip_check"),
			zap.String("path", r.URL.Path),
		)

		// Пытаемся получить IP из заголовка X-Real-IP сначала (устанавливается прокси)
		clientIP := r.Header.Get("X-Real-IP")
		clientIP = strings.TrimSpace(clientIP)
		ipSource := "X-Real-IP"

		// Используем RemoteAddr если X-Real-IP отсутствует
		if clientIP == "" {
			// Используем net.SplitHostPort для правильного парсинга RemoteAddr (поддерживает IPv6)
			clientIP = parseRemoteAddr(r.RemoteAddr)
			ipSource = "RemoteAddr"
			logger.Debug("Заголовок X-Real-IP отсутствует, используем RemoteAddr",
				zap.String("remote_addr", r.RemoteAddr),
				zap.String("parsed_ip", clientIP))
		}

		// Проверяем IP через use case
		isTrusted, err := h.ipCheckerUseCase.CheckIP(clientIP)

		// Раздельная обработка ошибок для лучшей наблюдаемости
		if err != nil {
			logger.Error("Ошибка при проверке IP",
				zap.String("client_ip", clientIP),
				zap.String("ip_source", ipSource),
				zap.Error(err))
			http.Error(w, "Внутренняя ошибка сервера при проверке IP", http.StatusInternalServerError)
			return
		}

		if !isTrusted {
			logger.Warn("Доступ запрещен для недоверенного IP",
				zap.String("client_ip", clientIP),
				zap.String("ip_source", ipSource))
			http.Error(w, "Доступ запрещен: IP не в доверенной подсети", http.StatusForbidden)
			return
		}

		// Логируем успешную проверку
		logger.Info("Проверка IP пройдена",
			zap.String("client_ip", clientIP),
			zap.String("ip_source", ipSource))

		// Добавляем IP клиента в контекст для downstream обработчиков (с типизированными ключами)
		ctx := context.WithValue(r.Context(), clientIPKey, clientIP)
		ctx = context.WithValue(ctx, ipSourceKey, ipSource)

		// Передаем управление следующему обработчику
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
