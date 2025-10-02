package handlers

import (
	"github.com/skiphead/practicum/pkg/storage"
	"github.com/skiphead/practicum/pkg/utils"
	"io"
	"log"
	"math/rand"
	"net/http"
)

const (
	keyLength     = 5
	randomCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

type URLHandler struct {
	storage    storage.Storage
	serverAddr string
}

func NewURLHandler(storage storage.Storage, serverAddr string) *URLHandler {
	return &URLHandler{
		storage:    storage,
		serverAddr: serverAddr,
	}
}

func (h *URLHandler) HandleRequest(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.CreateShortURL(w, r)
	case http.MethodGet:
		h.RedirectURL(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *URLHandler) CreateShortURL(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	if !utils.IsValidURL(originalURL) {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	key := h.generateUniqueKey()
	h.storage.Save(key, originalURL)

	response := "http://" + h.serverAddr + "/" + key

	w.WriteHeader(http.StatusCreated)

	_, err = w.Write([]byte(response))
	if err != nil {
		return
	}

}

func (h *URLHandler) RedirectURL(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		http.Error(w, "Short key is required", http.StatusBadRequest)
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
