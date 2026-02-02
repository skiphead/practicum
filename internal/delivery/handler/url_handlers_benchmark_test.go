package handler

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/skiphead/practicum/internal/domain/entity"
	"github.com/stretchr/testify/mock"
)

// Бенчмарк для создания короткого URL через текст
func BenchmarkURLHandler_CreateShortURL(b *testing.B) {
	handler, mockStorage := setupTestHandler()

	// Настраиваем мок для всех итераций
	mockStorage.On("Save", mock.Anything, "https://benchmark.example.com", "benchmark-user").
		Return(&entity.ShortURL{
			ShortCode:   "bench123",
			OriginalURL: "https://benchmark.example.com",
		}, nil)

	// Если используется проверка дубликатов
	mockStorage.On("FindByOriginalURL", mock.Anything, "https://benchmark.example.com", "benchmark-user").
		Return(nil, nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := createTestRequest("POST", "/", []byte("https://benchmark.example.com"))
		req = addUserContext(req, "benchmark-user")
		req.Header.Set("Content-Type", "text/plain")

		rr := httptest.NewRecorder()
		handler.createShortURL(rr, req)
	}
}

// Бенчмарк для создания короткого URL через API (JSON)
func BenchmarkURLHandler_CreateShortAPIURL(b *testing.B) {
	handler, mockStorage := setupTestHandler()

	// Настраиваем мок для всех итераций
	mockStorage.On("Save", mock.Anything, "https://api.benchmark.example.com", "benchmark-user").
		Return(&entity.ShortURL{
			ShortCode:   "api456",
			OriginalURL: "https://api.benchmark.example.com",
		}, nil)

	// Если используется проверка дубликатов
	mockStorage.On("FindByOriginalURL", mock.Anything, "https://api.benchmark.example.com", "benchmark-user").
		Return(nil, nil)

	request := entity.ShortenRequest{URL: "https://api.benchmark.example.com"}
	body, _ := json.Marshal(request)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := createTestRequest("POST", "/api/shorten", body)
		req = addUserContext(req, "benchmark-user")
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		handler.CreateShortAPIURL(rr, req)
	}
}

// Бенчмарк для редиректа по короткой ссылке
func BenchmarkURLHandler_RedirectURL(b *testing.B) {
	handler, mockStorage := setupTestHandler()

	// Настраиваем мок для всех итераций
	mockStorage.On("Get", mock.Anything, "benchmark123").
		Return(&entity.ShortURL{
			ShortCode:   "benchmark123",
			OriginalURL: "https://benchmark-redirect.example.com",
			IsActive:    true,
		}, nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := createTestRequest("GET", "/benchmark123", nil)
		rr := httptest.NewRecorder()

		// Создаем роутер для каждой итерации
		router := chi.NewRouter()
		router.Get("/{key}", handler.RedirectURL)
		router.ServeHTTP(rr, req)
	}
}

// Бенчмарк для сценария с ошибкой (не найден URL)
func BenchmarkURLHandler_RedirectURL_NotFound(b *testing.B) {
	handler, mockStorage := setupTestHandler()

	// Настраиваем мок для ошибки "не найден"
	mockStorage.On("Get", mock.Anything, "nonexistent").
		Return(nil, errors.New("not found"))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := createTestRequest("GET", "/nonexistent", nil)
		rr := httptest.NewRecorder()

		router := chi.NewRouter()
		router.Get("/{key}", handler.RedirectURL)
		router.ServeHTTP(rr, req)
	}
}

// Комплексный бенчмарк: создание + редирект
func BenchmarkURLHandler_CreateAndRedirect(b *testing.B) {
	handler, mockStorage := setupTestHandler()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Часть 1: Создание короткой ссылки
		shortCode := "dynamic123"
		originalURL := "https://dynamic.example.com"

		// Настройка моков для этой итерации
		mockStorage.ExpectedCalls = nil // Очищаем предыдущие вызовы

		// Для создания
		mockStorage.On("FindByOriginalURL", mock.Anything, originalURL, "benchmark-user").
			Return(nil, nil)
		mockStorage.On("Save", mock.Anything, originalURL, "benchmark-user").
			Return(&entity.ShortURL{
				ShortCode:   shortCode,
				OriginalURL: originalURL,
				IsActive:    true,
			}, nil)

		// Для редиректа
		mockStorage.On("Get", mock.Anything, shortCode).
			Return(&entity.ShortURL{
				ShortCode:   shortCode,
				OriginalURL: originalURL,
				IsActive:    true,
			}, nil)

		// Создание URL
		reqCreate := createTestRequest("POST", "/", []byte(originalURL))
		reqCreate = addUserContext(reqCreate, "benchmark-user")
		reqCreate.Header.Set("Content-Type", "text/plain")

		rrCreate := httptest.NewRecorder()
		handler.createShortURL(rrCreate, reqCreate)

		// Часть 2: Редирект по созданной ссылке
		reqRedirect := createTestRequest("GET", "/"+shortCode, nil)
		rrRedirect := httptest.NewRecorder()

		router := chi.NewRouter()
		router.Get("/{key}", handler.RedirectURL)
		router.ServeHTTP(rrRedirect, reqRedirect)
	}
}

// Бенчмарк для разных размеров URL
func BenchmarkURLHandler_CreateShortURL_VariousSizes(b *testing.B) {
	handler, mockStorage := setupTestHandler()

	testCases := []struct {
		name string
		url  string
	}{
		{"Short URL", "https://short.com"},
		{"Medium URL", "https://medium.example.com/path/to/resource?query=param"},
		{"Long URL", "https://very-long-domain-name.example.com/very/deep/path/with/many/segments/and/query/parameters?param1=value1&param2=value2&param3=value3&param4=value4"},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			// Настраиваем моки для каждого теста
			mockStorage.On("FindByOriginalURL", mock.Anything, tc.url, "benchmark-user").
				Return(nil, nil)
			mockStorage.On("Save", mock.Anything, tc.url, "benchmark-user").
				Return(&entity.ShortURL{
					ShortCode:   "code123",
					OriginalURL: tc.url,
				}, nil)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				req := createTestRequest("POST", "/", []byte(tc.url))
				req = addUserContext(req, "benchmark-user")
				req.Header.Set("Content-Type", "text/plain")

				rr := httptest.NewRecorder()
				handler.createShortURL(rr, req)
			}
		})
	}
}
