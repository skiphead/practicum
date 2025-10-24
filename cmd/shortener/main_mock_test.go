package main

import (
	"testing"
	"time"

	"github.com/skiphead/practicum/pkg/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockServer замоканый сервер для тестирования
type MockServer struct {
	mock.Mock
}

func (m *MockServer) Start() <-chan error {
	args := m.Called()
	return args.Get(0).(<-chan error)
}

func (m *MockServer) Shutdown(timeout time.Duration) error {
	args := m.Called(timeout)
	return args.Error(0)
}

// MockStorage замоканое хранилище
type MockStorage struct {
	mock.Mock
}

func (m *MockStorage) Save(url string) (string, error) {
	args := m.Called(url)
	return args.String(0), args.Error(1)
}

func (m *MockStorage) Get(id string) (string, error) {
	args := m.Called(id)
	return args.String(0), args.Error(1)
}

func TestStorageIntegration(t *testing.T) {

	store := storage.NewMemoryStorage()

	// Проверяем, что хранилище работает
	store.Save("test", "https://example.com")
	short, _ := store.Get("test")
	assert.NotEmpty(t, short)
	assert.Equal(t, "https://example.com", short)
}
