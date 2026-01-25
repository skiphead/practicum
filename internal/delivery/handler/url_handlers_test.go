package handler

import (
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/skiphead/practicum/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestURLHandler_CreateShortURL(t *testing.T) {
	handler, mockStorage := setupTestHandler()

	tests := []struct {
		name           string
		body           string
		userID         string
		mockSetup      func()
		expectedStatus int
		expectedBody   string
	}{
		{
			name:   "Successfully create short URL",
			body:   "https://example.com",
			userID: "test-user-1",
			mockSetup: func() {
				mockStorage.On("Save", mock.Anything, "https://example.com", "test-user-1").
					Return(&entity.ShortURL{ShortCode: "abc123", OriginalURL: "https://example.com"}, nil)
			},
			expectedStatus: http.StatusCreated,
			expectedBody:   "http://localhost:8080/abc123",
		},

		{
			name:   "Invalid URL",
			body:   "invalid-url",
			userID: "test-user-1",
			mockSetup: func() {
				// No mock setup needed since validation fails before storage call
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Invalid URL\n",
		},
		{
			name:   "Empty body",
			body:   "",
			userID: "test-user-1",
			mockSetup: func() {
				// No mock setup needed
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "URL is required\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mockSetup != nil {
				tt.mockSetup()
			}

			req := createTestRequest("POST", "/", []byte(tt.body))
			req = addUserContext(req, tt.userID)
			req.Header.Set("Content-Type", "text/plain")

			rr := httptest.NewRecorder()
			handler.createShortURL(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			assert.Contains(t, rr.Body.String(), tt.expectedBody)
		})
	}
}

func TestURLHandler_CreateShortAPIURL(t *testing.T) {
	handler, mockStorage := setupTestHandler()

	tests := []struct {
		name           string
		request        entity.ShortenRequest
		userID         string
		mockSetup      func()
		expectedStatus int
	}{
		{
			name:    "Successfully create short URL via API",
			request: entity.ShortenRequest{URL: "https://api.example.com"},
			userID:  "test-user-2",
			mockSetup: func() {
				mockStorage.On("Save", mock.Anything, "https://api.example.com", "test-user-2").
					Return(&entity.ShortURL{ShortCode: "def456", OriginalURL: "https://api.example.com"}, nil)
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name:    "Missing user context",
			request: entity.ShortenRequest{URL: "https://api.example.com"},
			userID:  "",
			mockSetup: func() {
				// No storage calls expected
			},
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mockSetup != nil {
				tt.mockSetup()
			}

			body, _ := json.Marshal(tt.request)
			req := createTestRequest("POST", "/api/shorten", body)

			if tt.userID != "" {
				req = addUserContext(req, tt.userID)
			}

			rr := httptest.NewRecorder()
			handler.createShortAPIURL(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.expectedStatus == http.StatusCreated {
				var response map[string]string
				json.Unmarshal(rr.Body.Bytes(), &response)
				assert.Equal(t, "http://localhost:8080/def456", response["result"])
			}
		})
	}
}

func TestURLHandler_RedirectURL(t *testing.T) {
	handler, mockStorage := setupTestHandler()

	tests := []struct {
		name             string
		shortCode        string
		mockSetup        func()
		expectedStatus   int
		expectedLocation string
	}{
		{
			name:      "Successful redirect",
			shortCode: "abc123",
			mockSetup: func() {
				mockStorage.On("Get", mock.Anything, "abc123").
					Return(&entity.ShortURL{
						ShortCode:   "abc123",
						OriginalURL: "https://example.com",
						IsActive:    true,
					}, nil)
			},
			expectedStatus:   http.StatusTemporaryRedirect,
			expectedLocation: "https://example.com",
		},
		{
			name:      "URL not found",
			shortCode: "nonexistent",
			mockSetup: func() {
				mockStorage.On("Get", mock.Anything, "nonexistent").
					Return(nil, errors.New("not found"))
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:      "URL deleted",
			shortCode: "deleted123",
			mockSetup: func() {
				mockStorage.On("Get", mock.Anything, "deleted123").
					Return(&entity.ShortURL{
						ShortCode:   "deleted123",
						OriginalURL: "https://deleted.com",
						IsActive:    false,
					}, nil)
			},
			expectedStatus: http.StatusGone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mockSetup != nil {
				tt.mockSetup()
			}

			req := createTestRequest("GET", "/"+tt.shortCode, nil)
			rr := httptest.NewRecorder()

			// Создаем роутер и обрабатываем запрос
			router := chi.NewRouter()
			router.Get("/{key}", handler.redirectURL)
			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.expectedLocation != "" {
				assert.Equal(t, tt.expectedLocation, rr.Header().Get("Location"))
			}
		})
	}
}
