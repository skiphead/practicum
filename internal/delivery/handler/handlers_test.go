package handlers

import (
	"bytes"
	"context"
	"github.com/skiphead/practicum/internal/domain/entity"
	"github.com/stretchr/testify/mock"
	"net/http"
	"net/http/httptest"
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
func setupTestHandler() (*URLHandler, *MockURLUseCase) {
	mockStorage := new(MockURLUseCase)
	handler := NewURLHandler(mockStorage, "localhost:8080", "http://localhost:8080", "secret")
	return handler, mockStorage
}

func createTestRequest(method, url string, body []byte) *http.Request {
	req := httptest.NewRequest(method, url, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func addUserContext(req *http.Request, userID string) *http.Request {
	ctx := context.WithValue(req.Context(), keyUserID, userID)
	return req.WithContext(ctx)
}
