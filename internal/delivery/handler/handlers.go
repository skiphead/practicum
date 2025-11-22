package handlers

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/skiphead/practicum/internal/domain/entity"
	"github.com/skiphead/practicum/internal/usecase"
	"github.com/skiphead/practicum/pkg/utils"
	"go.uber.org/zap"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Константы для JWT
const (
	sessionCookieName = "session_token"
	//sessionSecret     = "your-secret-key" // В реальном приложении вынесите в конфиг
	sessionDuration = 24 * time.Hour
)

// SessionClaims структура для хранения данных в JWT токене
type SessionClaims struct {
	jwt.RegisteredClaims
	UserID string `json:"user_id"`
}

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

func (h *URLHandler) ChiMux() *chi.Mux {
	r := chi.NewRouter()
	r.Use(compressionMiddleware)
	r.Use(loggingMiddleware)
	r.Use(h.sessionMiddleware) // Добавляем сессионный middleware
	r.Get("/{key}", h.redirectURL)
	r.Get("/api/user/urls", h.getApiUserUrls)
	r.Post("/", h.createShortURL)
	r.Post("/api/shorten", h.createShortAPIURL)
	r.Post("/api/shorten/batch", h.createBatchShortAPIURL)
	r.Get("/ping", h.pingDB)

	return r
}

// sessionMiddleware проверяет JWT токен сессии
func (h *URLHandler) sessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var userID string

		// Получаем куку с токеном
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			// Если куки нет, создаем новую сессию
			userID = h.createNewSession(w)
			ctx := context.WithValue(r.Context(), "user_id", userID)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Парсим и валидируем токен с проверкой алгоритма
		token, err := jwt.ParseWithClaims(cookie.Value, &SessionClaims{}, func(token *jwt.Token) (interface{}, error) {
			// Проверяем, что используется ожидаемый алгоритм подписи
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(os.Getenv("SESSION_KEY")), nil
		})

		if err != nil || !token.Valid {
			// Если токен невалидный, создаем новую сессию
			userID = h.createNewSession(w)
			ctx := context.WithValue(r.Context(), "user_id", userID)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Извлекаем claims
		if claims, ok := token.Claims.(*SessionClaims); ok {
			if claims.UserID == "" {
				// Если user_id отсутствует в токене, возвращаем 401
				http.Error(w, "Invalid session token", http.StatusUnauthorized)
				return
			}
			userID = claims.UserID
		} else {
			// Если не удалось извлечь claims, создаем новую сессию
			userID = h.createNewSession(w)
		}

		// Сохраняем user_id в контекст
		ctx := context.WithValue(r.Context(), "user_id", userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// createNewSession создает новую сессию с новым user_id
func (h *URLHandler) createNewSession(w http.ResponseWriter) string {
	userID := uuid.New().String()

	// Создаем JWT токен
	claims := SessionClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(sessionDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		UserID: userID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(os.Getenv("SESSION_KEY")))
	if err != nil {
		zap.L().Error("Failed to create JWT token", zap.Error(err))
		return userID
	}

	// Устанавливаем куку
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    tokenString,
		Path:     "/",
		Expires:  time.Now().Add(sessionDuration),
		HttpOnly: true,
		Secure:   false, // В продакшене установите true для HTTPS
		SameSite: http.SameSiteLaxMode,
	})

	return userID
}

// getUserIDFromContext извлекает user_id из контекста
func (h *URLHandler) getUserIDFromContext(ctx context.Context) string {
	if userID, ok := ctx.Value("user_id").(string); ok {
		return userID
	}
	return ""
}

// Обновляем метод getApiUserUrls для использования user_id из контекста
func (h *URLHandler) getApiUserUrls(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID := h.getUserIDFromContext(r.Context())
	if userID == "" {
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*5)
	defer cancel()

	urls, err := h.storage.GetByUserID(ctx, userID)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		return
	}

	if len(urls) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var list []entity.ListByUserIDResponse

	for _, url := range urls {
		list = append(list, entity.ListByUserIDResponse{
			OriginalUrl: url.OriginalURL,
			ShortUrl:    fmt.Sprintf("%s/%s", h.baseURL, url.ShortCode),
		})
	}

	render.JSON(w, r, list)
}

// Обновляем методы сохранения URL для использования user_id
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

	resp, err := h.storage.Save(ctx, originalURL, userID) // Предполагаем, что метод Save теперь принимает userID
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

// Обновляем вызовы processAndSaveURL, передавая r *http.Request
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

	shortURL, isConflict, err := h.processAndSaveURL(original.URL, w, r) // Передаем r
	if err != nil {
		return
	}

	if isConflict {
		render.Status(r, http.StatusConflict)
		render.JSON(w, r, map[string]string{"result": shortURL})
		return
	}

	w.WriteHeader(http.StatusCreated)
	render.JSON(w, r, map[string]string{"result": shortURL})
}

func (h *URLHandler) createShortURL(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	body, err := h.readRequestBody(r)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	originalURL := string(body)

	shortURL, isConflict, err := h.processAndSaveURL(originalURL, w, r) // Передаем r
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

	w.WriteHeader(http.StatusCreated)
	_, err = w.Write([]byte(shortURL))
	if err != nil {
		zap.L().Error("write error", zap.Error(err))
		return
	}
}

// Остальной код остается без изменений...
func (h *URLHandler) pingDB(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second*5)
	defer cancel()
	err := h.storage.Ping(ctx)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, err = w.Write([]byte(err.Error()))
		if err != nil {
			zap.L().Error("write error", zap.Error(err))
			return
		}
		return
	}

	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte("ok"))
	if err != nil {
		zap.L().Error("write error", zap.Error(err))
		return
	}
}

func (h *URLHandler) createBatchShortAPIURL(w http.ResponseWriter, r *http.Request) {
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

	var original []entity.BatchShortenRequest
	if err := json.Unmarshal(body, &original); err != nil {
		zap.L().Error("unmarshal error", zap.Error(err))
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	userID := h.getUserIDFromContext(r.Context())
	if userID == "" {
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*5)
	defer cancel()

	duplicates, errDuplicates := h.storage.FindDuplicateURLs(ctx, original)
	if errDuplicates != nil {
		render.Status(r, http.StatusInternalServerError)
	}

	if len(duplicates) > 0 {
		w.WriteHeader(http.StatusConflict)
		render.JSON(w, r, duplicates)
		return
	}

	shortURLs, err := h.storage.BatchSave(ctx, original, userID)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	render.JSON(w, r, shortURLs)
}

// handleConflictError обрабатывает ошибки конфликтов для всех методов
func (h *URLHandler) handleConflictError(w http.ResponseWriter, err error) bool {
	if h.storage.IsDuplicateError(err) {
		http.Error(w, err.Error(), http.StatusConflict)
		return true
	}
	return false
}

// readRequestBody унифицированное чтение тела запроса
func (h *URLHandler) readRequestBody(r *http.Request) ([]byte, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	defer h.closeBody(r.Body)
	return body, nil
}

// validateURL унифицированная валидация URL
func (h *URLHandler) validateURL(originalURL string, w http.ResponseWriter) error {
	if originalURL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return fmt.Errorf("URL is required")
	}

	if !utils.IsValidURL(originalURL) {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return fmt.Errorf("invalid URL scheme or host")
	}

	u, err := url.Parse(originalURL)
	if err != nil || u.Scheme == "" || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return fmt.Errorf("invalid URL scheme or host")
	}

	return nil
}

// buildShortURL создает короткий URL на основе ключа
func (h *URLHandler) buildShortURL(key string) string {
	return fmt.Sprintf("%s/%s", h.baseURL, key)
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

	w.Header().Set("Location", data.OriginalURL)
	w.WriteHeader(http.StatusTemporaryRedirect)
}

// closeBody унифицированное закрытие тела запроса
func (h *URLHandler) closeBody(body io.ReadCloser) {
	if err := body.Close(); err != nil {
		zap.L().Error("error close Body", zap.Error(err))
	}
}

func compressionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ow := w

		// Обработка сжатия ответа
		if shouldCompressResponse(r) {
			cw := newCompressWriter(w)
			ow = cw
			defer func(cw *compressWriter) {
				errCw := cw.Close()
				if errCw != nil {
					zap.L().Error("error closing compress writer", zap.Error(errCw))
				}
			}(cw)
		}

		// Обработка распаковки
		if shouldDecompressRequest(r) {
			cr, err := newCompressReader(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			r.Body = cr
			defer func(cr *compressReader) {
				errCr := cr.Close()
				if errCr != nil {
					zap.L().Error("error closing compress reader", zap.Error(errCr))
				}
			}(cr)
		}

		next.ServeHTTP(ow, r)
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Используем WrapResponseWriter для получения статуса и размера ответа
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)

		duration := time.Since(start)

		// Логирование запроса
		logRequest(r, duration)
		// Логирование ответа
		logResponse(ww)
	})
}

func logRequest(r *http.Request, duration time.Duration) {
	zap.L().Info("request",
		zap.String("path", r.URL.Path),
		zap.String("method", r.Method),
		zap.Duration("duration", duration))
}

func logResponse(ww middleware.WrapResponseWriter) {
	zap.L().Info("response",
		zap.Int("status", ww.Status()),
		zap.Int("bytes", ww.BytesWritten()))
}

// shouldCompressResponse проверяет, нужно ли сжимать ответ
func shouldCompressResponse(r *http.Request) bool {
	// Быстрая проверка выхода - если клиент не поддерживает gzip
	if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		return false
	}

	contentType := r.Header.Get("Content-Type")

	// Проверяем, является ли тип контента одним из поддерживаемых
	return strings.HasPrefix(contentType, "text/plain") ||
		contentType == "application/json"
}

// shouldDecompressRequest проверяет, нужно ли распаковывать запрос
func shouldDecompressRequest(r *http.Request) bool {
	contentType := r.Header.Get("Content-Type")
	supportedContentType := contentType == "text/plain" || contentType == "application/json"
	contentEncoding := r.Header.Get("Content-Encoding")
	sendsGzip := strings.Contains(contentEncoding, "gzip")

	return supportedContentType && sendsGzip
}

// compressWriter реализует интерфейс http.ResponseWriter и позволяет прозрачно для сервера
// сжимать передаваемые данные и выставлять правильные HTTP-заголовки
type compressWriter struct {
	w  http.ResponseWriter
	zw *gzip.Writer
}

func newCompressWriter(w http.ResponseWriter) *compressWriter {
	return &compressWriter{
		w:  w,
		zw: gzip.NewWriter(w),
	}
}

func (c *compressWriter) Header() http.Header {
	return c.w.Header()
}

func (c *compressWriter) Write(p []byte) (int, error) {
	return c.zw.Write(p)
}

func (c *compressWriter) WriteHeader(statusCode int) {
	// Убираем проверку статуса - устанавливаем заголовок для всех статусов
	c.w.Header().Set("Content-Encoding", "gzip")
	c.w.WriteHeader(statusCode)
}

// Close закрывает gzip.Writer и досылает все данные из буфера.
func (c *compressWriter) Close() error {
	return c.zw.Close()
}

// compressReader реализует интерфейс io.ReadCloser и позволяет прозрачно для сервера
// декомпрессировать получаемые от клиента данные
type compressReader struct {
	r  io.ReadCloser
	zr *gzip.Reader
}

func newCompressReader(r io.ReadCloser) (*compressReader, error) {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}

	return &compressReader{
		r:  r,
		zr: zr,
	}, nil
}

// Изменено на pointer receiver
func (c *compressReader) Read(p []byte) (n int, err error) {
	return c.zr.Read(p)
}

func (c *compressReader) Close() error {
	if err := c.r.Close(); err != nil {
		return err
	}
	return c.zr.Close()
}
