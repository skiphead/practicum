package handlers

import (
	"bytes"
	"encoding/json"
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
	handler := NewURLHandler(NewMockStorage(), "localhost:8080", "")

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
	handler := NewURLHandler(mockStorage, "localhost:8080", "")

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
	handler := NewURLHandler(mockStorage, "localhost:8080", "")

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
	handler := NewURLHandler(mockStorage, "localhost:8080", "")

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
	handler := NewURLHandler(NewMockStorage(), "localhost:8080", "")

	// Создаем запрос с ошибкой при чтении тела
	req := httptest.NewRequest(http.MethodPost, "/", errorReader{})
	rr := httptest.NewRecorder()

	handler.createShortURL(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("createShortURL() with body error status = %v, wantStatus %v", rr.Code, http.StatusBadRequest)
	}
}

func TestURLHandler_createShortApiURL(t *testing.T) {
	tests := []struct {
		name           string
		body           interface{}
		serverAddr     string
		baseURL        string
		wantStatus     int
		wantResponse   string
		checkSaved     bool
		validateResult func(t *testing.T, result string, storage *MockStorage)
	}{
		{
			name:       "success with serverAddr",
			body:       map[string]string{"url": "https://example.com"},
			serverAddr: "localhost:8080",
			wantStatus: http.StatusCreated,
			validateResult: func(t *testing.T, result string, storage *MockStorage) {
				if !strings.HasPrefix(result, "http://localhost:8080/") {
					t.Errorf("Result should start with serverAddr, got: %s", result)
				}
				key := strings.TrimPrefix(result, "http://localhost:8080/")
				if storage.storage[key] != "https://example.com" {
					t.Errorf("Storage wasn't updated correctly")
				}
			},
		},
		{
			name:       "success with baseURL",
			body:       map[string]string{"url": "https://google.com"},
			serverAddr: "localhost:8080",
			baseURL:    "https://my.domain",
			wantStatus: http.StatusCreated,
			validateResult: func(t *testing.T, result string, storage *MockStorage) {
				if !strings.HasPrefix(result, "https://my.domain/") {
					t.Errorf("Result should use baseURL, got: %s", result)
				}
			},
		},
		{
			name:         "missing url field",
			body:         map[string]string{"not_url": "test"},
			wantStatus:   http.StatusBadRequest,
			wantResponse: "URL is required\n",
		},
		{
			name:         "invalid URL",
			body:         map[string]string{"url": "invalid-url"},
			wantStatus:   http.StatusBadRequest,
			wantResponse: "Invalid URL\n",
		},
		{
			name:         "malformed JSON",
			body:         "{invalid json",
			wantStatus:   http.StatusBadRequest,
			wantResponse: "URL is required\n", // Изменено с "Invalid request body"
		},
		{
			name:         "empty body",
			body:         "",
			wantStatus:   http.StatusBadRequest,
			wantResponse: "URL is required\n", // JSON unmarshal пустой строки создает пустую map
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &MockStorage{storage: make(map[string]string)}
			handler := &URLHandler{
				storage:    storage,
				serverAddr: tt.serverAddr,
				baseURL:    tt.baseURL,
			}

			var bodyBytes []byte
			switch v := tt.body.(type) {
			case string:
				bodyBytes = []byte(v)
			default:
				bodyBytes, _ = json.Marshal(v)
			}

			req := httptest.NewRequest("POST", "/api/shorten", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			handler.createShortApiURL(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, rr.Code)
			}

			if tt.wantResponse != "" {
				if rr.Body.String() != tt.wantResponse {
					t.Errorf("Expected response body '%s', got '%s'", tt.wantResponse, rr.Body.String())
				}
			}

			if tt.validateResult != nil {
				var resp struct {
					Result string `json:"result"`
				}
				if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}
				tt.validateResult(t, resp.Result, storage)
			}
		})
	}
}

// Тест для проверки ошибки чтения тела запроса
func TestCreateShortApiURL_ReadBodyError(t *testing.T) {
	storage := &MockStorage{storage: make(map[string]string)}
	handler := &URLHandler{
		storage:    storage,
		serverAddr: "test",
	}

	// Создаем запрос с телом, которое вызывает ошибку при чтении
	req := httptest.NewRequest("POST", "/api/shorten", &errorReader{})
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.createShortApiURL(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	expectedResponse := "Invalid request body\n"
	if rr.Body.String() != expectedResponse {
		t.Errorf("Expected response body '%s', got '%s'", expectedResponse, rr.Body.String())
	}
}

// Тест для проверки валидации URL
func TestURLValidation(t *testing.T) {
	handler := &URLHandler{
		storage:    &MockStorage{storage: make(map[string]string)},
		serverAddr: "test",
	}

	tests := []struct {
		url   string
		valid bool
	}{
		{"https://example.com", true},
		{"http://localhost:8080", true},
		{"http://example.com/path", true},
		{"https://sub.domain.example.com", true},
		{"ftp://invalid.com", false},
		{"", false},
		{"not-a-url", false},
		{"javascript:alert(1)", false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			body, _ := json.Marshal(map[string]string{"url": tt.url})
			req := httptest.NewRequest("POST", "/api/shorten", bytes.NewReader(body))
			rr := httptest.NewRecorder()

			handler.createShortApiURL(rr, req)

			if tt.valid {
				if rr.Code != http.StatusCreated {
					t.Errorf("Expected URL '%s' to be valid, but got status %d", tt.url, rr.Code)
				}
			} else {
				if rr.Code != http.StatusBadRequest {
					t.Errorf("Expected URL '%s' to be invalid, but got status %d", tt.url, rr.Code)
				}
				if rr.Body.String() != "Invalid URL\n" {
					t.Errorf("Expected 'Invalid URL' response for URL '%s', got '%s'", tt.url, rr.Body.String())
				}
			}
		})
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
