package handlers

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type MockStorage struct {
	storage map[string]string
}

func NewMockStorage() *MockStorage {
	return &MockStorage{
		storage: make(map[string]string),
	}
}

func (m *MockStorage) Save(key, value string) {
	m.storage[key] = value
}

func (m *MockStorage) Get(key string) (string, bool) {
	value, exists := m.storage[key]
	return value, exists
}

func (m *MockStorage) Delete(key string) {
	delete(m.storage, key)
}

func TestURLHandler(t *testing.T) {

	logger, _ := zap.NewDevelopment()
	zap.ReplaceGlobals(logger)
	defer logger.Sync()

	mockStorage := NewMockStorage()
	handler := NewURLHandler(mockStorage, "localhost:8080", "http://short.ru")

	tests := []struct {
		name           string
		method         string
		path           string
		body           string
		contentType    string
		expectedStatus int
		expectedBody   string
		setup          func()
		cleanup        func()
	}{
		{
			name:           "Create short URL - success",
			method:         "POST",
			path:           "/",
			body:           "https://example.com",
			contentType:    "text/plain",
			expectedStatus: http.StatusCreated,
			expectedBody:   "http://short.ru/",
		},
		{
			name:           "Create short URL - empty body",
			method:         "POST",
			path:           "/",
			body:           "",
			contentType:    "text/plain",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "URL is required",
		},
		{
			name:           "Create short URL - invalid URL",
			method:         "POST",
			path:           "/",
			body:           "invalid-url",
			contentType:    "text/plain",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Invalid URL",
		},
		{
			name:           "Create short API URL - success",
			method:         "POST",
			path:           "/api/shorten",
			body:           `{"url": "https://api.example.com"}`,
			contentType:    "application/json",
			expectedStatus: http.StatusCreated,
			expectedBody:   `{"result":"http://short.ru/`,
		},
		{
			name:           "Create short API URL - wrong content type",
			method:         "POST",
			path:           "/api/shorten",
			body:           `{"url": "https://api.example.com"}`,
			contentType:    "text/plain",
			expectedStatus: http.StatusUnsupportedMediaType,
			expectedBody:   "Content-Type must be application/json",
		},
		{
			name:           "Create short API URL - invalid JSON",
			method:         "POST",
			path:           "/api/shorten",
			body:           `invalid json`,
			contentType:    "application/json",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "URL is required",
		},
		{
			name:           "Create short API URL - empty URL",
			method:         "POST",
			path:           "/api/shorten",
			body:           `{"url": ""}`,
			contentType:    "application/json",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "URL is required",
		},
		{
			name:           "Redirect URL - success",
			method:         "GET",
			path:           "/abc123",
			expectedStatus: http.StatusTemporaryRedirect,
			setup: func() {
				mockStorage.Save("abc123", "https://example.com")
			},
			cleanup: func() {
				mockStorage.Delete("abc123")
			},
		},
		{
			name:           "Redirect URL - not found",
			method:         "GET",
			path:           "/nonexistent",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "Short URL not found",
		},
		{
			name:           "Redirect URL - root path",
			method:         "GET",
			path:           "/",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   "Method Not Allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if tt.setup != nil {
				tt.setup()
			}
			if tt.cleanup != nil {
				defer tt.cleanup()
			}

			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}

			req := httptest.NewRequest(tt.method, tt.path, body)
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			rr := httptest.NewRecorder()

			router := handler.ChiMux()
			router.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tt.expectedStatus)
			}

			if tt.expectedStatus == http.StatusTemporaryRedirect {
				location := rr.Header().Get("Location")
				if location != "https://example.com" {
					t.Errorf("handler returned wrong location header: got %v want %v", location, "https://example.com")
				}
			}
		})
	}
}

func TestURLHandler_HandleRequest(t *testing.T) {
	mockStorage := NewMockStorage()
	handler := NewURLHandler(mockStorage, "localhost:8080", "")

	tests := []struct {
		name           string
		method         string
		path           string
		body           string
		expectedStatus int
	}{
		{
			name:           "GET method with root path",
			method:         "GET",
			path:           "/",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "POST method with empty body",
			method:         "POST",
			path:           "/",
			body:           "",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "PUT method - not allowed",
			method:         "PUT",
			path:           "/",
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			req := httptest.NewRequest(tt.method, tt.path, body)
			rr := httptest.NewRecorder()

			handler.HandleRequest(rr, req)

			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("HandleRequest returned wrong status code: got %v want %v", status, tt.expectedStatus)
			}
		})
	}
}

func TestChiRouterSpecific(t *testing.T) {
	mockStorage := NewMockStorage()
	handler := NewURLHandler(mockStorage, "localhost:8080", "http://short.ru")
	router := handler.ChiMux()

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
		description    string
	}{
		{
			name:           "GET to root path - method not allowed",
			method:         "GET",
			path:           "/",
			expectedStatus: http.StatusMethodNotAllowed,
			description:    "Chi возвращает 405 для GET /, так как маршрут зарегистрирован только для POST",
		},
		{
			name:           "POST to nonexistent path - method not allowed",
			method:         "POST",
			path:           "/nonexistent",
			expectedStatus: http.StatusMethodNotAllowed,
			description:    "Chi возвращает 405 для POST /nonexistent, потому что путь существует для GET, но не для POST",
		},
		{
			name:           "PUT to any route - method not allowed",
			method:         "PUT",
			path:           "/",
			expectedStatus: http.StatusMethodNotAllowed,
			description:    "Chi возвращает 405 для неподдерживаемых методов",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("%s: got status %v, want %v", tt.description, status, tt.expectedStatus)
			}
		})
	}
}

func TestCompression(t *testing.T) {
	mockStorage := NewMockStorage()
	handler := NewURLHandler(mockStorage, "localhost:8080", "http://short.ru")

	t.Run("Gzip compression response", func(t *testing.T) {
		body := `{"url": "https://compressed.example.com"}`
		req := httptest.NewRequest("POST", "/api/shorten", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept-Encoding", "gzip")

		rr := httptest.NewRecorder()

		router := handler.ChiMux()
		router.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusCreated {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusCreated)
		}

		if encoding := rr.Header().Get("Content-Encoding"); encoding != "gzip" {
			t.Errorf("expected gzip encoding, got: %s", encoding)
		}

		reader, err := gzip.NewReader(rr.Body)
		if err != nil {
			t.Fatalf("failed to create gzip reader: %v", err)
		}
		defer reader.Close()

		decompressed, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("failed to decompress response: %v", err)
		}

		var response map[string]string
		if err := json.Unmarshal(decompressed, &response); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if result, exists := response["result"]; !exists || !strings.HasPrefix(result, "http://short.ru/") {
			t.Errorf("unexpected result: %v", response)
		}
	})

	t.Run("Gzip decompression request", func(t *testing.T) {
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		_, err := gz.Write([]byte(`{"url": "https://gzipped.example.com"}`))
		if err != nil {
			t.Fatalf("failed to write gzipped data: %v", err)
		}
		gz.Close()

		req := httptest.NewRequest("POST", "/api/shorten", &buf)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")

		rr := httptest.NewRecorder()

		router := handler.ChiMux()
		router.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusCreated {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusCreated)
		}
	})
}

func TestURLValidation(t *testing.T) {
	mockStorage := NewMockStorage()
	handler := NewURLHandler(mockStorage, "localhost:8080", "http://short.ru")

	invalidURLs := []struct {
		name string
		url  string
	}{
		{"No scheme", "example.com"},
		{"Invalid scheme", "ftp://example.com"},
		{"No host", "http://"},
		{"With spaces", "https://exa mple.com"},
		{"Empty", ""},
	}

	for _, tt := range invalidURLs {
		t.Run(tt.name, func(t *testing.T) {
			req1 := httptest.NewRequest("POST", "/", strings.NewReader(tt.url))
			rr1 := httptest.NewRecorder()
			handler.createShortURL(rr1, req1)

			if status := rr1.Code; status != http.StatusBadRequest {
				t.Errorf("createShortURL for invalid URL %s returned status %v, want %v", tt.url, status, http.StatusBadRequest)
			}

			jsonBody := fmt.Sprintf(`{"url": "%s"}`, tt.url)
			req2 := httptest.NewRequest("POST", "/api/shorten", strings.NewReader(jsonBody))
			req2.Header.Set("Content-Type", "application/json")
			rr2 := httptest.NewRecorder()
			handler.createShortAPIURL(rr2, req2)

			if status := rr2.Code; status != http.StatusBadRequest {
				t.Errorf("createShortAPIURL for invalid URL %s returned status %v, want %v", tt.url, status, http.StatusBadRequest)
			}
		})
	}
}

func TestKeyGeneration(t *testing.T) {
	mockStorage := NewMockStorage()
	handler := NewURLHandler(mockStorage, "localhost:8080", "http://short.ru")

	keys := make(map[string]bool)
	for i := 0; i < 100; i++ {
		key := handler.generateRandomKey()
		if len(key) != keyLength {
			t.Errorf("generateRandomKey returned key of wrong length: got %v want %v", len(key), keyLength)
		}
		if keys[key] {
			t.Errorf("generateRandomKey returned duplicate key: %s", key)
		}
		keys[key] = true
	}

	mockStorage.Save("abcde", "https://example.com")
	key := handler.generateUniqueKey()
	if key == "abcde" {
		t.Errorf("generateUniqueKey returned existing key: %s", key)
	}
}

func TestBuildShortURL(t *testing.T) {
	tests := []struct {
		name       string
		serverAddr string
		baseURL    string
		key        string
		expected   string
	}{
		{
			name:       "With base URL",
			serverAddr: "localhost:8080",
			baseURL:    "http://short.ru",
			key:        "abc123",
			expected:   "http://short.ru/abc123",
		},
		{
			name:       "Without base URL",
			serverAddr: "localhost:8080",
			baseURL:    "",
			key:        "abc123",
			expected:   "http://localhost:8080/abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewURLHandler(NewMockStorage(), tt.serverAddr, tt.baseURL)
			result := handler.buildShortURL(tt.key)
			if result != tt.expected {
				t.Errorf("buildShortURL returned wrong URL: got %v want %v", result, tt.expected)
			}
		})
	}
}

func BenchmarkCreateShortURL(b *testing.B) {
	mockStorage := NewMockStorage()
	handler := NewURLHandler(mockStorage, "localhost:8080", "http://short.ru")
	router := handler.ChiMux()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		url := fmt.Sprintf("https://example.com/page/%d", i)
		req := httptest.NewRequest("POST", "/", strings.NewReader(url))
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
	}
}

func BenchmarkCreateShortAPIURL(b *testing.B) {
	mockStorage := NewMockStorage()
	handler := NewURLHandler(mockStorage, "localhost:8080", "http://short.ru")
	router := handler.ChiMux()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		body := fmt.Sprintf(`{"url": "https://api.example.com/page/%d"}`, i)
		req := httptest.NewRequest("POST", "/api/shorten", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
	}
}

func TestMiddleware(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	tests := []struct {
		name            string
		acceptEncoding  string
		contentEncoding string
		contentType     string
		expectGzip      bool
	}{
		{
			name:           "Supports gzip with JSON",
			acceptEncoding: "gzip",
			contentType:    "application/json",
			expectGzip:     true,
		},
		{
			name:           "Supports gzip with HTML",
			acceptEncoding: "gzip",
			contentType:    "text/html",
			expectGzip:     true,
		},
		{
			name:           "No gzip support",
			acceptEncoding: "",
			contentType:    "application/json",
			expectGzip:     false,
		},
		{
			name:            "Gzip request body",
			contentEncoding: "gzip",
			contentType:     "application/json",
			expectGzip:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.acceptEncoding != "" {
				req.Header.Set("Accept-Encoding", tt.acceptEncoding)
			}
			if tt.contentEncoding != "" {
				req.Header.Set("Content-Encoding", tt.contentEncoding)
			}
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			rr := httptest.NewRecorder()

			middleware := xMiddleware(testHandler)
			middleware.ServeHTTP(rr, req)

			if tt.expectGzip {
				if encoding := rr.Header().Get("Content-Encoding"); encoding != "gzip" {
					t.Errorf("expected gzip encoding, got: %s", encoding)
				}
			}
		})
	}
}

func TestChiRouter(t *testing.T) {
	mockStorage := NewMockStorage()
	handler := NewURLHandler(mockStorage, "localhost:8080", "http://short.ru")
	router := handler.ChiMux()

	routes := []struct {
		method string
		path   string
	}{
		{"GET", "/{key}"},
		{"POST", "/"},
		{"POST", "/api/shorten"},
	}

	for _, route := range routes {
		t.Run(fmt.Sprintf("Route %s %s", route.method, route.path), func(t *testing.T) {
			ctx := chi.NewRouteContext()
			matched := router.Match(ctx, route.method, route.path)
			if !matched {
				t.Errorf("Route %s %s not found in router", route.method, route.path)
			}
		})
	}
}
