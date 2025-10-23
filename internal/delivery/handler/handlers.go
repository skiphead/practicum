package handlers

import (
	"encoding/json"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/skiphead/practicum/pkg/storage"
	"github.com/skiphead/practicum/pkg/utils"
	"go.uber.org/zap"
	"io"
	"log"
	"math/rand"
	"net/http"
	"time"
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
	r.Use(LoggerMiddleware)
	r.Get("/{key}", h.redirectURL)
	r.Post("/", h.createShortURL)
	r.Post("/api", h.createShortAPIURL)

	return r
}

func (h *URLHandler) ServeMUX() *http.ServeMux {
	r := http.NewServeMux()
	r.HandleFunc("/", h.HandleRequest) // Регистрирует обработчик для всех путей

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
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	defer func(Body io.ReadCloser) {
		errClose := Body.Close()
		if errClose != nil {
			log.Printf("error close Body create short url: %v", errClose)
		}
	}(r.Body)
	var m map[string]string
	errUnmarshal := json.Unmarshal(body, &m)
	if errUnmarshal != nil {
		zap.L().Error("unmarshal error", zap.Error(errUnmarshal))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	originalURL, ok := m["url"]
	if !ok {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	if !utils.IsValidURL(originalURL) {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	key := h.generateUniqueKey()

	h.storage.Save(key, originalURL)

	baseURL := fmt.Sprintf("http://%s/%s", h.serverAddr, key)

	if h.baseURL != "" {
		baseURL = fmt.Sprintf("%s/%s", h.baseURL, key)
	}
	resp, errMarshal := json.Marshal(map[string]string{"result": baseURL})
	if errMarshal != nil {
		zap.L().Error("marshal error", zap.Error(errMarshal))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	_, err = w.Write(resp)
	if err != nil {
		return
	}
}

func (h *URLHandler) createShortURL(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	defer func(Body io.ReadCloser) {
		errClose := Body.Close()
		if errClose != nil {
			log.Printf("error close Body create short url: %v", errClose)
		}
	}(r.Body)

	originalURL := string(body)
	if originalURL == "" {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if !utils.IsValidURL(originalURL) {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	key := h.generateUniqueKey()

	h.storage.Save(key, originalURL)

	baseURL := fmt.Sprintf("http://%s/%s", h.serverAddr, key)

	if h.baseURL != "" {
		baseURL = fmt.Sprintf("%s/%s", h.baseURL, key)
	}

	w.WriteHeader(http.StatusCreated)

	_, err = w.Write([]byte(baseURL))
	if err != nil {
		return
	}

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

	//http.Redirect(w, r, originalURL, http.StatusTemporaryRedirect)
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

func LoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(ww, r)

		duration := time.Since(start)

		//Запрос
		zap.L().Info("msg",
			zap.String("path", r.URL.Path),
			zap.String("method", r.Method),
			zap.Duration("duration", duration))

		//Ответ
		zap.L().Info("msg",
			zap.Int("status", ww.Status()),
			zap.Int("bytes", ww.BytesWritten()))

	})
}
