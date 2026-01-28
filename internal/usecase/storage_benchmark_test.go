// internal/usecase/storage_benchmark_test.go
package usecase

import (
	"context"
	"fmt"
	"testing"

	"github.com/skiphead/practicum/internal/domain/entity"
	"github.com/skiphead/practicum/pkg/utils"

	"github.com/stretchr/testify/mock"
)

// Mock структуры
type mockFileStorage struct {
	mock.Mock
}

type mockURLRepository struct {
	mock.Mock
}

// Mock методы для FileStorage - ВСЕ методы интерфейса
func (m *mockFileStorage) Save(userID, correlationID, shortCode, originalURL string) error {
	args := m.Called(userID, correlationID, shortCode, originalURL)
	return args.Error(0)
}

func (m *mockFileStorage) Get(shortCode string) (*entity.ShortURL, bool, error) {
	args := m.Called(shortCode)
	if args.Get(0) == nil {
		return nil, args.Bool(1), args.Error(2)
	}
	return args.Get(0).(*entity.ShortURL), args.Bool(1), args.Error(2)
}

func (m *mockFileStorage) GetByID(id string) (*entity.ShortURL, bool, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Bool(1), args.Error(2)
	}
	return args.Get(0).(*entity.ShortURL), args.Bool(1), args.Error(2)
}

func (m *mockFileStorage) FindByUserID(userID string) ([]entity.ShortURL, error) {
	args := m.Called(userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]entity.ShortURL), args.Error(1)
}

func (m *mockFileStorage) FindByOriginalURL(originalURL string) (*entity.ShortURL, error) {
	args := m.Called(originalURL)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.ShortURL), args.Error(1)
}

func (m *mockFileStorage) Delete(shortURL string) error {
	args := m.Called(shortURL)
	return args.Error(0)
}

func (m *mockFileStorage) DeleteByID(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *mockFileStorage) SetDeletedByUserIDAndURLs(userID string, shortCodes []string, isActive bool) error {
	args := m.Called(userID, shortCodes, isActive)
	return args.Error(0)
}

func (m *mockFileStorage) BatchSave(ctx context.Context, urls []entity.ShortURL) error {
	args := m.Called(ctx, urls)
	return args.Error(0)
}

func (m *mockFileStorage) Stats() map[string]interface{} {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(map[string]interface{})
}

// Mock методы для URLRepository - ВСЕ методы интерфейса
func (m *mockURLRepository) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockURLRepository) IsDuplicateError(err error) bool {
	args := m.Called(err)
	return args.Bool(0)
}

func (m *mockURLRepository) Create(ctx context.Context, userID, shortCode, originalURL string) (*entity.ShortURL, error) {
	args := m.Called(ctx, userID, shortCode, originalURL)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.ShortURL), args.Error(1)
}

func (m *mockURLRepository) Get(ctx context.Context, shortCode string) (*entity.ShortURL, error) {
	args := m.Called(ctx, shortCode)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.ShortURL), args.Error(1)
}

func (m *mockURLRepository) GetByOriginalURL(ctx context.Context, originalURL string) (*entity.ShortURL, error) {
	args := m.Called(ctx, originalURL)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.ShortURL), args.Error(1)
}

func (m *mockURLRepository) GetByUserID(ctx context.Context, userID string) ([]entity.ShortURL, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]entity.ShortURL), args.Error(1)
}

func (m *mockURLRepository) CreateBatch(ctx context.Context, userID string, urls []entity.BatchShortenRequest, batchSize int) ([]entity.ShortURL, error) {
	args := m.Called(ctx, userID, urls, batchSize)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]entity.ShortURL), args.Error(1)
}

func (m *mockURLRepository) FindDuplicateURLs(ctx context.Context, urls []entity.BatchShortenRequest) ([]entity.ShortURL, error) {
	args := m.Called(ctx, urls)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]entity.ShortURL), args.Error(1)
}

func (m *mockURLRepository) UpdateIsActive(ctx context.Context, shortCodes []string, userID string, isActive bool, expectedRows int) ([]string, error) {
	args := m.Called(ctx, shortCodes, userID, isActive, expectedRows)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

// Новые методы
func (m *mockURLRepository) Update(ctx context.Context, shortURL *entity.ShortURL) (*entity.ShortURL, error) {
	args := m.Called(ctx, shortURL)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.ShortURL), args.Error(1)
}

func (m *mockURLRepository) Delete(ctx context.Context, id string) (string, error) {
	args := m.Called(ctx, id)
	return args.String(0), args.Error(1)
}

// Основные Benchmark функции
func BenchmarkSave(b *testing.B) {
	ctx := context.Background()
	originalURL := "https://example.com"
	userID := "test-user"
	shortCode := utils.GenerateRandomKey()

	mockRepo := new(mockURLRepository)
	mockFile := new(mockFileStorage)

	uc := NewStorageUseCase("http://localhost:8080", mockFile, mockRepo)

	// Настройка моков для успешного случая
	mockRepo.On("Ping", ctx).Return(nil)
	mockRepo.On("Create", ctx, userID, mock.Anything, originalURL).Return(&entity.ShortURL{
		OriginalURL: originalURL,
		ShortCode:   shortCode,
	}, nil)
	mockRepo.On("IsDuplicateError", mock.Anything).Return(false)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		uc.Save(ctx, originalURL, userID)
	}
}

func BenchmarkGet(b *testing.B) {
	ctx := context.Background()
	shortCode := "abc123"

	mockRepo := new(mockURLRepository)
	mockFile := new(mockFileStorage)

	uc := NewStorageUseCase("http://localhost:8080", mockFile, mockRepo)

	// Настройка моков
	mockRepo.On("Ping", ctx).Return(nil)
	mockRepo.On("Get", ctx, shortCode).Return(&entity.ShortURL{
		OriginalURL: "https://example.com",
		ShortCode:   shortCode,
	}, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		uc.Get(ctx, shortCode)
	}
}

func BenchmarkBatchSave(b *testing.B) {
	ctx := context.Background()
	userID := "test-user"

	// Подготовка тестовых данных
	urls := make([]entity.BatchShortenRequest, 100)
	for i := range urls {
		urls[i] = entity.BatchShortenRequest{
			CorrelationID: fmt.Sprintf("corr-%d", i),
			OriginalURL:   fmt.Sprintf("https://example.com/page%d", i),
		}
	}

	mockRepo := new(mockURLRepository)
	mockFile := new(mockFileStorage)

	uc := NewStorageUseCase("http://localhost:8080", mockFile, mockRepo)

	// Настройка моков
	shortURLs := make([]entity.ShortURL, len(urls))
	for i := range shortURLs {
		shortURLs[i] = entity.ShortURL{
			OriginalURL:   urls[i].OriginalURL,
			ShortCode:     utils.GenerateRandomKey(),
			CorrelationID: urls[i].CorrelationID,
		}
	}

	mockRepo.On("Ping", ctx).Return(nil)
	mockRepo.On("CreateBatch", ctx, userID, urls, 100).Return(shortURLs, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		uc.BatchSave(ctx, urls, userID)
	}
}

func BenchmarkDeleted(b *testing.B) {
	ctx := context.Background()
	userID := "test-user"
	shortCodes := []string{"code1", "code2", "code3", "code4", "code5"}

	mockRepo := new(mockURLRepository)
	mockFile := new(mockFileStorage)

	uc := NewStorageUseCase("http://localhost:8080", mockFile, mockRepo)

	// Настройка моков - база доступна
	mockRepo.On("Ping", ctx).Return(nil)
	mockRepo.On("UpdateIsActive", mock.Anything, mock.Anything, userID, false, mock.Anything).
		Return([]string{}, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		uc.Deleted(ctx, shortCodes, userID)
	}
}

func BenchmarkPing(b *testing.B) {
	ctx := context.Background()

	mockRepo := new(mockURLRepository)
	mockFile := new(mockFileStorage)

	uc := NewStorageUseCase("http://localhost:8080", mockFile, mockRepo)

	// Настройка моков
	mockRepo.On("Ping", ctx).Return(nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		uc.Ping(ctx)
	}
}
