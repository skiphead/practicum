package handlers

import (
	"bytes"
	"errors"

	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// MockStorage реализует интерфейс storage.Storage для тестов
type MockStorage struct {
	storage map[string]string
}

func NewMockStorage() *MockStorage {
	return &MockStorage{storage: make(map[string]string)}
}

func (m *MockStorage) Save(key, url string) {
	m.storage[key] = url
}

func (m *MockStorage) Get(key string) (string, bool) {
	url, exists := m.storage[key]
	return url, exists
}

// MockErrorStorage для тестирования ошибок
type MockErrorStorage struct{}

func (m *MockErrorStorage) Save(key, url string) {}
func (m *MockErrorStorage) Get(key string) (string, bool) {
	return "", false
}

func TestURLHandler_HandleRequest(t *testing.T) {
	handler := NewURLHandler(NewMockStorage(), "localhost:8080")

	tests := []struct {
		name       string
		method     string
		wantStatus int
	}{
		{
			name:       "Method PUT not allowed",
			method:     http.MethodPut,
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "Method DELETE not allowed",
			method:     http.MethodDelete,
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/", nil)
			rr := httptest.NewRecorder()

			handler.HandleRequest(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("HandleRequest() status = %v, wantStatus %v", rr.Code, tt.wantStatus)
			}
		})
	}
}

func TestURLHandler_createShortURL(t *testing.T) {
	mockStorage := NewMockStorage()
	handler := NewURLHandler(mockStorage, "localhost:8080")

	tests := []struct {
		name           string
		body           string
		wantStatus     int
		wantBodyPrefix string
		checkStorage   bool
	}{
		{
			name:           "Valid URL",
			body:           "https://google.com",
			wantStatus:     http.StatusCreated,
			wantBodyPrefix: "http://localhost:8080/",
			checkStorage:   true,
		},
		{
			name:       "Empty body",
			body:       "",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "Invalid URL",
			body:       "invalid-url",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(tt.body))
			rr := httptest.NewRecorder()

			handler.createShortURL(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("createShortURL() status = %v, wantStatus %v", rr.Code, tt.wantStatus)
			}

			if tt.wantBodyPrefix != "" {
				if !strings.HasPrefix(rr.Body.String(), tt.wantBodyPrefix) {
					t.Errorf("createShortURL() body = %v, want prefix %v", rr.Body.String(), tt.wantBodyPrefix)
				}
			}

			if tt.checkStorage {
				key := strings.TrimPrefix(rr.Body.String(), "http://localhost:8080/")
				if _, exists := mockStorage.Get(key); !exists {
					t.Error("createShortURL() key not saved in storage")
				}
			}
		})
	}
}

func TestURLHandler_redirectURL(t *testing.T) {
	mockStorage := NewMockStorage()
	handler := NewURLHandler(mockStorage, "localhost:8080")

	// Предварительно сохраним тестовый URL
	key := "abc123"
	originalURL := "https://google.com"
	mockStorage.Save(key, originalURL)

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantLoc    string
	}{
		{
			name:       "Valid redirect",
			path:       "/" + key,
			wantStatus: http.StatusTemporaryRedirect,
			wantLoc:    originalURL,
		},
		{
			name:       "Empty path",
			path:       "/",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "Non-existent key",
			path:       "/nonexistent",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rr := httptest.NewRecorder()

			handler.redirectURL(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("redirectURL() status = %v, wantStatus %v", rr.Code, tt.wantStatus)
			}

			if tt.wantLoc != "" && rr.Header().Get("Location") != tt.wantLoc {
				t.Errorf("redirectURL() Location = %v, want %v", rr.Header().Get("Location"), tt.wantLoc)
			}
		})
	}
}

func TestURLHandler_generateUniqueKey(t *testing.T) {
	mockStorage := NewMockStorage()
	handler := NewURLHandler(mockStorage, "localhost:8080")

	// Тестируем уникальность ключа
	key1 := handler.generateUniqueKey()
	key2 := handler.generateUniqueKey()

	if key1 == key2 {
		t.Error("generateUniqueKey() generated duplicate keys")
	}

	if len(key1) != keyLength {
		t.Errorf("generateUniqueKey() key length = %v, want %v", len(key1), keyLength)
	}

	// Проверяем что ключ состоит из допустимых символов
	for _, char := range key1 {
		if !strings.ContainsRune(randomCharset, char) {
			t.Errorf("generateUniqueKey() generated invalid character: %c", char)
		}
	}
}

func TestURLHandler_createShortURL_ReadBodyError(t *testing.T) {
	handler := NewURLHandler(NewMockStorage(), "localhost:8080")

	// Создаем запрос с ошибкой при чтении тела
	req := httptest.NewRequest(http.MethodPost, "/", errorReader{})
	rr := httptest.NewRecorder()

	handler.createShortURL(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("createShortURL() with body error status = %v, wantStatus %v", rr.Code, http.StatusBadRequest)
	}
}

// Вспомогательная структура для эмуляции ошибок чтения
type errorReader struct{}

func (errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("simulated read error")
}
func (errorReader) Close() error {
	return nil
}
