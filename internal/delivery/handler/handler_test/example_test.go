// handler/example_test.go
package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/skiphead/practicum/internal/audit"
	"github.com/skiphead/practicum/internal/delivery/handler"
	"github.com/skiphead/practicum/internal/domain/entity"
)

// MockURLUseCase реализация мока для usecase
type MockURLUseCase struct {
	mock.Mock
}

func (m *MockURLUseCase) Save(ctx context.Context, originalURL, userID string) (*entity.ShortURL, error) {
	args := m.Called(ctx, originalURL, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.ShortURL), args.Error(1)
}

func (m *MockURLUseCase) Get(ctx context.Context, shortCode string) (*entity.ShortURL, error) {
	args := m.Called(ctx, shortCode)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.ShortURL), args.Error(1)
}

func (m *MockURLUseCase) GetByUserID(ctx context.Context, userID string) ([]entity.ShortURL, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]entity.ShortURL), args.Error(1)
}

func (m *MockURLUseCase) BatchSave(ctx context.Context, urls []entity.BatchShortenRequest, userID string) ([]entity.BatchShortenResponse, error) {
	args := m.Called(ctx, urls, userID)
	return args.Get(0).([]entity.BatchShortenResponse), args.Error(1)
}

func (m *MockURLUseCase) Deleted(ctx context.Context, shortCodes []string, userID string) error {
	args := m.Called(ctx, shortCodes, userID)
	return args.Error(0)
}

func (m *MockURLUseCase) FindDuplicateURLs(ctx context.Context, urls []entity.BatchShortenRequest) ([]entity.BatchShortenResponse, error) {
	args := m.Called(ctx, urls)
	return args.Get(0).([]entity.BatchShortenResponse), args.Error(1)
}

func (m *MockURLUseCase) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockURLUseCase) IsDuplicateError(err error) bool {
	args := m.Called(err)
	return args.Bool(0)
}

// Test helpers
func setupTestHandler() (*handler.URLHandler, *MockURLUseCase) {
	mockStorage := new(MockURLUseCase)
	// Создаем пустой аудит адаптер для тестов
	auditAdapter := &audit.Adapter{}
	handler := handler.NewURLHandler(mockStorage, "localhost:8080", "http://localhost:8080", "secret", auditAdapter)
	return handler, mockStorage
}

// Helper для добавления cookie с session ID
func addSessionCookie(req *http.Request, sessionID string) *http.Request {
	if sessionID != "" {
		req.AddCookie(&http.Cookie{
			Name:  "session",
			Value: sessionID,
		})
	}
	return req
}

// Example_createShortAPIURL демонстрирует создание короткой ссылки через JSON API
func Example_createShortAPIURL() {
	h, mockStorage := setupTestHandler()
	router := h.ChiMux()

	// Подготавливаем тестовые данные
	jsonData := `{"url": "https://example.com/very/long/path"}`
	req := httptest.NewRequest("POST", "/api/shorten", strings.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")

	// Добавляем cookie для имитации сессии
	req = addSessionCookie(req, "test-session-id")

	// Настраиваем мок с mock.Anything для userID (так как middleware генерирует UUID)
	mockStorage.On("Save", mock.Anything, "https://example.com/very/long/path", mock.AnythingOfType("string")).
		Return(&entity.ShortURL{
			OriginalURL: "https://example.com/very/long/path",
			ShortCode:   "abc123",
		}, nil)

	// Выполняем запрос
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Проверяем результат
	fmt.Printf("Status Code: %d\n", w.Code)
	fmt.Printf("Content-Type: %s\n", w.Header().Get("Content-Type"))

	var response map[string]string
	json.Unmarshal(w.Body.Bytes(), &response)
	fmt.Printf("Short URL: %s\n", response["result"])

	// Output:
	// Status Code: 201
	// Content-Type: application/json
	// Short URL: http://localhost:8080/abc123
}

// Unit тесты для handler
func TestURLHandler(t *testing.T) {
	// Вспомогательная функция для тестирования с реальным middleware
	testWithMiddleware := func(t *testing.T, testFunc func(*testing.T, *handler.URLHandler, *MockURLUseCase)) {
		h, mockStorage := setupTestHandler()
		testFunc(t, h, mockStorage)
	}

	t.Run("createShortAPIURL успешное создание", func(t *testing.T) {
		testWithMiddleware(t, func(t *testing.T, h *handler.URLHandler, mockStorage *MockURLUseCase) {
			router := h.ChiMux()

			jsonData := `{"url": "https://example.com"}`
			req := httptest.NewRequest("POST", "/api/shorten", strings.NewReader(jsonData))
			req.Header.Set("Content-Type", "application/json")

			// Используем mock.Anything для userID, так как middleware генерирует UUID
			mockStorage.On("Save", mock.Anything, "https://example.com", mock.AnythingOfType("string")).
				Return(&entity.ShortURL{
					OriginalURL: "https://example.com",
					ShortCode:   "test123",
				}, nil)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusCreated, w.Code)
			assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

			var response map[string]string
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Equal(t, "http://localhost:8080/test123", response["result"])

			mockStorage.AssertExpectations(t)
		})
	})

	t.Run("createShortAPIURL неверный JSON", func(t *testing.T) {
		testWithMiddleware(t, func(t *testing.T, h *handler.URLHandler, mockStorage *MockURLUseCase) {
			router := h.ChiMux()

			jsonData := `{"url": "invalid-url"` // Неполный JSON
			req := httptest.NewRequest("POST", "/api/shorten", strings.NewReader(jsonData))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	})

	t.Run("createShortAPIURL дубликат", func(t *testing.T) {
		testWithMiddleware(t, func(t *testing.T, h *handler.URLHandler, mockStorage *MockURLUseCase) {
			router := h.ChiMux()

			jsonData := `{"url": "https://duplicate.com"}`
			req := httptest.NewRequest("POST", "/api/shorten", strings.NewReader(jsonData))
			req.Header.Set("Content-Type", "application/json")

			duplicateErr := fmt.Errorf("duplicate URL")
			mockStorage.On("Save", mock.Anything, "https://duplicate.com", mock.AnythingOfType("string")).
				Return(&entity.ShortURL{
					OriginalURL: "https://duplicate.com",
					ShortCode:   "existing123",
				}, duplicateErr)
			mockStorage.On("IsDuplicateError", duplicateErr).Return(true)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusConflict, w.Code)

			var response map[string]string
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Equal(t, "http://localhost:8080/existing123", response["result"])

			mockStorage.AssertExpectations(t)
		})
	})

	t.Run("redirectURL успешный редирект", func(t *testing.T) {
		testWithMiddleware(t, func(t *testing.T, h *handler.URLHandler, mockStorage *MockURLUseCase) {
			router := h.ChiMux()

			req := httptest.NewRequest("GET", "/test123", nil)

			mockStorage.On("Get", mock.Anything, "test123").
				Return(&entity.ShortURL{
					OriginalURL: "https://example.com",
					ShortCode:   "test123",
					IsActive:    true,
				}, nil)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusTemporaryRedirect, w.Code)
			assert.Equal(t, "https://example.com", w.Header().Get("Location"))

			mockStorage.AssertExpectations(t)
		})
	})

	t.Run("redirectURL не найден", func(t *testing.T) {
		testWithMiddleware(t, func(t *testing.T, h *handler.URLHandler, mockStorage *MockURLUseCase) {
			router := h.ChiMux()

			req := httptest.NewRequest("GET", "/notfound", nil)

			mockStorage.On("Get", mock.Anything, "notfound").
				Return((*entity.ShortURL)(nil), fmt.Errorf("not found"))

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusNotFound, w.Code)

			mockStorage.AssertExpectations(t)
		})
	})

	t.Run("redirectURL удален", func(t *testing.T) {
		testWithMiddleware(t, func(t *testing.T, h *handler.URLHandler, mockStorage *MockURLUseCase) {
			router := h.ChiMux()

			req := httptest.NewRequest("GET", "/deleted123", nil)

			mockStorage.On("Get", mock.Anything, "deleted123").
				Return(&entity.ShortURL{
					OriginalURL: "https://example.com",
					ShortCode:   "deleted123",
					IsActive:    false, // Удален
				}, nil)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusGone, w.Code)

			mockStorage.AssertExpectations(t)
		})
	})

	t.Run("getAPIUserUrls успешное получение", func(t *testing.T) {
		testWithMiddleware(t, func(t *testing.T, h *handler.URLHandler, mockStorage *MockURLUseCase) {
			router := h.ChiMux()

			req := httptest.NewRequest("GET", "/api/user/urls", nil)
			// Добавляем cookie для создания сессии
			req = addSessionCookie(req, "user123-session")

			// Мокаем GetByUserID с любым userID
			mockStorage.On("GetByUserID", mock.Anything, mock.AnythingOfType("string")).
				Return([]entity.ShortURL{
					{
						OriginalURL: "https://example.com/1",
						ShortCode:   "code1",
					},
					{
						OriginalURL: "https://example.com/2",
						ShortCode:   "code2",
					},
				}, nil)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

			var urls []map[string]string
			err := json.Unmarshal(w.Body.Bytes(), &urls)
			require.NoError(t, err)
			assert.Len(t, urls, 2)

			mockStorage.AssertExpectations(t)
		})
	})

	t.Run("getAPIUserUrls пустой список", func(t *testing.T) {
		testWithMiddleware(t, func(t *testing.T, h *handler.URLHandler, mockStorage *MockURLUseCase) {
			router := h.ChiMux()

			req := httptest.NewRequest("GET", "/api/user/urls", nil)
			req = addSessionCookie(req, "empty-user-session")

			mockStorage.On("GetByUserID", mock.Anything, mock.AnythingOfType("string")).
				Return([]entity.ShortURL{}, nil)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusNoContent, w.Code)

			mockStorage.AssertExpectations(t)
		})
	})

	t.Run("deleteAPIUserUrls успешное удаление", func(t *testing.T) {
		testWithMiddleware(t, func(t *testing.T, h *handler.URLHandler, mockStorage *MockURLUseCase) {
			router := h.ChiMux()

			deleteData := `["code1", "code2"]`
			req := httptest.NewRequest("DELETE", "/api/user/urls", strings.NewReader(deleteData))
			req.Header.Set("Content-Type", "application/json")
			req = addSessionCookie(req, "delete-user-session")

			mockStorage.On("Deleted", mock.Anything, []string{"code1", "code2"}, mock.AnythingOfType("string")).
				Return(nil)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusAccepted, w.Code)

			mockStorage.AssertExpectations(t)
		})
	})

	t.Run("pingDB ошибка базы данных", func(t *testing.T) {
		testWithMiddleware(t, func(t *testing.T, h *handler.URLHandler, mockStorage *MockURLUseCase) {
			router := h.ChiMux()

			req := httptest.NewRequest("GET", "/ping", nil)

			mockStorage.On("Ping", mock.Anything).Return(fmt.Errorf("database down"))

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusInternalServerError, w.Code)

			mockStorage.AssertExpectations(t)
		})
	})

}

// Пример работы с middleware
func TestURLHandler_Middleware(t *testing.T) {

	t.Run("Сжатие ответов", func(t *testing.T) {
		h, mockStorage := setupTestHandler()
		router := h.ChiMux()

		req := httptest.NewRequest("GET", "/ping", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		mockStorage.On("Ping", mock.Anything).Return(nil)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		// Проверяем, что ответ сжат
		assert.Equal(t, "gzip", w.Header().Get("Content-Encoding"))

		mockStorage.AssertExpectations(t)
	})

	t.Run("Session middleware создает userID", func(t *testing.T) {
		h, mockStorage := setupTestHandler()
		router := h.ChiMux()

		// Запрос без cookie
		req := httptest.NewRequest("POST", "/api/shorten", strings.NewReader(`{"url": "https://test.com"}`))
		req.Header.Set("Content-Type", "application/json")

		// Middleware должен создать новую сессию
		mockStorage.On("Save", mock.Anything, "https://test.com", mock.AnythingOfType("string")).
			Return(&entity.ShortURL{
				OriginalURL: "https://test.com",
				ShortCode:   "new123",
			}, nil)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		// Должны получить cookie в ответе
		assert.NotEmpty(t, w.Header().Get("Set-Cookie"))

		mockStorage.AssertExpectations(t)
	})
}
