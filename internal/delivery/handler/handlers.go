package handlers

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/skiphead/practicum/internal/domain/entity"
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
	"unicode"
)

const (
	keyLength     = 5
	randomCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

type URLHandler struct {
	storage    storage.Storage
	serverAddr string
	baseURL    string
}

func NewURLHandler(storage storage.Storage, serverAddr, baseURL string) *URLHandler {
	return &URLHandler{
		storage:    storage,
		serverAddr: serverAddr,
		baseURL:    baseURL,
	}
}

func (h *URLHandler) ChiMux() *chi.Mux {
	r := chi.NewRouter()
	r.Use(xMiddleware)
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Get("/{key}", h.redirectURL)
	r.Post("/", h.createShortURL)
	r.Post("/api/shorten", h.createShortAPIURL)

	return r
}

func (h *URLHandler) HandleRequest(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.createShortURL(w, r)
	case http.MethodGet:
		h.redirectURL(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
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
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	// Очищаем URL из JSON
	original.URL = strings.TrimSpace(h.sanitizeURL(original.URL))

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

	originalURL := strings.TrimSpace(string(body))
	shortURL, err := h.processAndSaveURL(originalURL, w)
	if err != nil {
		return
	}

	w.WriteHeader(http.StatusCreated)
	_, err = w.Write([]byte(shortURL))
	if err != nil {
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
	// Очищаем URL перед обработкой
	cleanedURL := strings.TrimSpace(h.sanitizeURL(originalURL))

	if err := h.validateURL(cleanedURL, w); err != nil {
		return "", err
	}

	key := h.generateUniqueKey()
	h.storage.Save(key, cleanedURL)
	return h.buildShortURL(key), nil
}

func (h *URLHandler) sanitizeURL(urlString string) string {
	// Удаляем управляющие символы
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, urlString)
}

func (h *URLHandler) validateURL(originalURL string, w http.ResponseWriter) error {
	if originalURL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return fmt.Errorf("URL is required")
	}

	// Очищаем URL перед валидацией
	cleanedURL := strings.TrimSpace(h.sanitizeURL(originalURL))

	if cleanedURL == "" {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return fmt.Errorf("invalid URL after sanitization")
	}

	if !utils.IsValidURL(cleanedURL) {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return fmt.Errorf("invalid URL")
	}

	u, err := url.Parse(cleanedURL)
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
	return fmt.Sprintf("http://%s/%s", h.serverAddr, key)
}

func (h *URLHandler) redirectURL(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	key := r.URL.Path[1:]
	originalURL, exists := h.storage.Get(key)
	if !exists {
		http.Error(w, "Short URL not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Location", originalURL)
	w.WriteHeader(http.StatusTemporaryRedirect)
}

func (h *URLHandler) generateUniqueKey() string {
	for {
		key := h.generateRandomKey()
		if _, exists := h.storage.Get(key); !exists {
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

func xMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ow := w

		// Унифицированная обработка сжатия
		acceptEncoding := r.Header.Get("Accept-Encoding")
		supportsGzip := strings.Contains(acceptEncoding, "gzip")

		contentType := r.Header.Get("Content-Type")
		supportedContentType := contentType == "text/html" || contentType == "application/json"

		if supportedContentType {
			if supportsGzip {
				cw := newCompressWriter(w)
				ow = cw
				defer cw.Close()
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
				defer cr.Close()
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

func (c compressReader) Read(p []byte) (n int, err error) {
	return c.zr.Read(p)
}

func (c *compressReader) Close() error {
	if err := c.r.Close(); err != nil {
		return err
	}
	return c.zr.Close()
}
