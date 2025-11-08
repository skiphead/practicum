package handlers

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/skiphead/practicum/internal/domain/entity"
	"github.com/skiphead/practicum/internal/usecase"
	"github.com/skiphead/practicum/pkg/storage"
	"github.com/skiphead/practicum/pkg/utils"
	"go.uber.org/zap"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	keyLength     = 8
	randomCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	schema        = `http`
)

type URLHandler struct {
	storage    storage.Storage
	health     *usecase.HealthUseCase
	serverAddr string
	baseURL    string
}

func NewURLHandler(storage storage.Storage, serverAddr, baseURL string, health usecase.HealthUseCase) *URLHandler {
	return &URLHandler{
		storage:    storage,
		serverAddr: serverAddr,
		baseURL:    baseURL,
		health:     &health,
	}
}

func (h *URLHandler) ChiMux() *chi.Mux {

	r := chi.NewRouter()
	r.Use(CompressionMiddleware)
	r.Use(LoggingMiddleware)
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Get("/{key}", h.redirectURL)
	r.Get("/stats", h.stats)
	r.Post("/", h.createShortURL)
	r.Post("/api/shorten", h.createShortAPIURL)
	r.Get("/ping", h.pingDB)

	return r
}

func (h *URLHandler) pingDB(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second*5)
	defer cancel()
	if !h.health.HealthRepo.Ping(ctx) {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("ok"))
	if err != nil {
		zap.L().Error("write error", zap.Error(err))
		return
	}
}

func (h *URLHandler) stats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return
	}

	w.WriteHeader(http.StatusCreated)
	render.JSON(w, r, h.storage.Stats())
}

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

	shortURL, err := h.processAndSaveURL(original.URL, w)
	if err != nil {
		return
	}

	w.WriteHeader(http.StatusCreated)
	render.JSON(w, r, map[string]string{"result": shortURL})
}

func (h *URLHandler) createShortURL(w http.ResponseWriter, r *http.Request) {
	body, err := h.readRequestBody(r)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	originalURL := string(body)
	shortURL, err := h.processAndSaveURL(originalURL, w)
	if err != nil {
		zap.L().Error("process error", zap.Error(err))
		return
	}

	w.WriteHeader(http.StatusCreated)
	_, err = w.Write([]byte(shortURL))
	if err != nil {
		zap.L().Error("write error", zap.Error(err))
		return
	}
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

// processAndSaveURL общая логика обработки и сохранения URL
func (h *URLHandler) processAndSaveURL(originalURL string, w http.ResponseWriter) (string, error) {
	if err := h.validateURL(originalURL, w); err != nil {
		return "", err
	}

	key := h.generateUniqueKey()
	h.storage.Save(key, originalURL)
	return h.buildShortURL(key), nil
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
	if h.baseURL != "" {
		return fmt.Sprintf("%s/%s", h.baseURL, key)
	}

	return fmt.Sprintf("%s://%s/%s", schema, h.serverAddr, key)
}

func (h *URLHandler) redirectURL(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	key := r.URL.Path[1:]
	data, exists, _ := h.storage.Get(key)

	if !exists {
		http.Error(w, "Short URL not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Location", data.OriginalURL)
	w.WriteHeader(http.StatusTemporaryRedirect)
}

func (h *URLHandler) generateUniqueKey() string {
	for {
		key := h.generateRandomKey()
		if _, exists, _ := h.storage.Get(key); !exists {
			return key
		}
	}
}

func (h *URLHandler) generateRandomKey() string {
	buf := make([]byte, keyLength)
	for i := range buf {
		buf[i] = randomCharset[rand.Intn(len(randomCharset))]
	}
	return string(buf)
}

// closeBody унифицированное закрытие тела запроса
func (h *URLHandler) closeBody(body io.ReadCloser) {
	if err := body.Close(); err != nil {
		log.Printf("error close Body: %v", err)
	}
}

/*
func xMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ow := w

		// Унифицированная обработка сжатия
		supportsGzip := strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")

		contentType := r.Header.Get("Content-Type")
		supportedContentType := contentType == "text/plain" || contentType == "application/json"

		if supportedContentType {
			if supportsGzip {
				cw := newCompressWriter(w)
				ow = cw
				defer func(cw *compressWriter) {
					errCw := cw.Close()
					if errCw != nil {
						zap.L().Error("error closing compress writer", zap.Error(errCw))
					}
				}(cw)
			}

			contentEncoding := r.Header.Get("Content-Encoding")
			sendsGzip := strings.Contains(contentEncoding, "gzip")
			if sendsGzip {
				cr, err := newCompressReader(r.Body)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				r.Body = cr
				defer func(cr *compressReader) {
					errCr := cr.Close()
					if errCr != nil {
						zap.L().Error("error closing compress writer", zap.Error(errCr))
					}
				}(cr)
			}
		}

		ww := middleware.NewWrapResponseWriter(ow, r.ProtoMajor)
		next.ServeHTTP(ww, r)

		duration := time.Since(start)

		// Объединенное логирование запроса и ответа
		zap.L().Info("request",
			zap.String("path", r.URL.Path),
			zap.String("method", r.Method),
			zap.Duration("duration", duration))
		zap.L().Info("response",
			zap.Int("status", ww.Status()),
			zap.Int("bytes", ww.BytesWritten()))
	})
}

*/

func CompressionMiddleware(next http.Handler) http.Handler {
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

func LoggingMiddleware(next http.Handler) http.Handler {
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
	supportsGzip := strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")
	contentType := r.Header.Get("Content-Type")
	supportedContentType := contentType == "text/plain" || contentType == "application/json"

	return supportsGzip && supportedContentType
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
	if statusCode < 300 {
		c.w.Header().Set("Content-Encoding", "gzip")
	}
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
