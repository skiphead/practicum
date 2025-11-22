package usecase

import (
	"context"
	"errors"
	"fmt"
	"github.com/skiphead/practicum/internal/domain/entity"
	"github.com/skiphead/practicum/internal/domain/repository"
	"github.com/skiphead/practicum/pkg/utils"
)

// Ошибки базы данных
var (
	ErrDuplicateKey = errors.New("запись уже существует")
)

type URLUseCase interface {
	Ping(ctx context.Context) error
	IsDuplicateError(err error) bool
	Get(ctx context.Context, shortCode string) (*entity.ShortURL, error)
	GetByUserID(ctx context.Context, userID string) ([]entity.ShortURL, error)
	Save(ctx context.Context, originalURL, userID string) (*entity.ShortURL, error)
	BatchSave(ctx context.Context, in []entity.BatchShortenRequest, userID string) ([]entity.BatchShortenResponse, error)
	FindDuplicateURLs(ctx context.Context, urls []entity.BatchShortenRequest) ([]entity.BatchShortenResponse, error)
}

// urlUseCase реализует бизнес-логику работы с сокращенными URL
type urlUseCase struct {
	baseURL     string
	fileStorage repository.FileStorage
	storageRepo repository.URLRepository
}

// NewStorageUseCase создает новый экземпляр usecase для работы с хранилищем
func NewStorageUseCase(baseURL string, fs repository.FileStorage, repo repository.URLRepository) URLUseCase {
	return &urlUseCase{
		baseURL:     baseURL,
		fileStorage: fs,
		storageRepo: repo,
	}
}

// Ping проверяет доступность основного хранилища
func (s *urlUseCase) Ping(ctx context.Context) error {
	return s.storageRepo.Ping(ctx)
}

// isDatabaseAvailable проверяет, доступна ли база данных
func (s *urlUseCase) isDatabaseAvailable(ctx context.Context) bool {
	return s.storageRepo.Ping(ctx) == nil
}

// buildShortURL создает полный короткий URL
func (s *urlUseCase) buildShortURL(shortCode string) string {
	return fmt.Sprintf("%s/%s", s.baseURL, shortCode)
}

// IsDuplicateError унифицированная проверка на ошибку дублирования
func (s *urlUseCase) IsDuplicateError(err error) bool {

	return s.storageRepo.IsDuplicateError(err)
}

// Save сохраняет оригинальный URL и возвращает сокращенную версию
func (s *urlUseCase) Save(ctx context.Context, originalURL, userID string) (*entity.ShortURL, error) {
	shortCode := utils.GenerateRandomKey()

	if !s.isDatabaseAvailable(ctx) {
		// Используем файловое хранилище как fallback
		if err := s.fileStorage.Save(userID, "", shortCode, originalURL); err != nil {
			return nil, fmt.Errorf("save to file storage: %w", err)
		}

		return &entity.ShortURL{
			OriginalURL: originalURL,
			ShortCode:   shortCode,
		}, nil
	}

	// Используем основное хранилище (базу данных)
	shortURL, err := s.storageRepo.Create(ctx, "test", shortCode, originalURL)
	if s.storageRepo.IsDuplicateError(err) {
		duplicate, errGet := s.storageRepo.GetByOriginalURL(ctx, originalURL)
		if errGet != nil {
			return nil, errGet
		}

		return duplicate, err
	}

	if err != nil {
		return nil, fmt.Errorf("create in database: %w", err)
	}

	return shortURL, nil
}

// Get возвращает оригинальный URL по короткому коду
func (s *urlUseCase) Get(ctx context.Context, shortCode string) (*entity.ShortURL, error) {
	if !s.isDatabaseAvailable(ctx) {
		// Используем файловое хранилище как fallback
		resp, exists, err := s.fileStorage.Get(shortCode)
		if err != nil {
			return nil, fmt.Errorf("get from file storage: %w", err)
		}
		if !exists {
			return nil, fmt.Errorf("short URL with code '%s' not found", shortCode)
		}

		return &entity.ShortURL{
			ID:          resp.UUID,
			OriginalURL: resp.OriginalURL,
			ShortCode:   shortCode,
		}, nil
	}

	// Используем основное хранилище (базу данных)
	shortURL, err := s.storageRepo.Get(ctx, shortCode)
	if err != nil {
		return nil, fmt.Errorf("get from database: %w", err)
	}

	return shortURL, nil
}

// GetByUserID возвращает список коротких url по id пользователя
func (s *urlUseCase) GetByUserID(ctx context.Context, userID string) ([]entity.ShortURL, error) {
	var list []entity.ShortURL
	if !s.isDatabaseAvailable(ctx) {
		// Используем файловое хранилище как fallback
		resp, err := s.fileStorage.FindByUserID(userID)
		if err != nil {
			return nil, fmt.Errorf("get from file storage: %w", err)
		}

		for _, url := range resp {
			list = append(list, entity.ShortURL{
				ID:          url.UUID,
				OriginalURL: url.OriginalURL,
				ShortCode:   url.ShortURL,
			})
		}

		return list, nil
	}

	// Используем основное хранилище (базу данных)
	shortURL, err := s.storageRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get from database: %w", err)
	}

	return shortURL, nil
}

func (s *urlUseCase) FindDuplicateURLs(ctx context.Context, urls []entity.BatchShortenRequest) ([]entity.BatchShortenResponse, error) {

	response := make([]entity.BatchShortenResponse, 0, len(urls))

	createdURLs, err := s.storageRepo.FindDuplicateURLs(ctx, urls)
	if err != nil {
		return nil, fmt.Errorf("find duplicate URLs: %w", err)
	}

	for _, url := range createdURLs {
		response = append(response, entity.BatchShortenResponse{
			CorrelationID: url.CorrelationID,
			ShortURL:      s.buildShortURL(url.ShortCode),
		})
	}

	return response, nil
}

// BatchSave сохраняет пакет URL и возвращает сокращенные версии
func (s *urlUseCase) BatchSave(ctx context.Context, urls []entity.BatchShortenRequest, userID string) ([]entity.BatchShortenResponse, error) {
	if len(urls) == 0 {
		return []entity.BatchShortenResponse{}, nil
	}

	response := make([]entity.BatchShortenResponse, 0, len(urls))

	if !s.isDatabaseAvailable(ctx) {
		// Используем файловое хранилище как fallback
		for _, item := range urls {
			code := utils.GenerateRandomKey()
			if err := s.fileStorage.Save(userID, item.CorrelationID, code, item.OriginalURL); err != nil {
				return nil, fmt.Errorf("save batch item to file storage: %w", err)
			}

			response = append(response, entity.BatchShortenResponse{
				CorrelationID: item.CorrelationID,
				ShortURL:      s.buildShortURL(code),
			})
		}
		return response, nil
	}

	// Используем основное хранилище (базу данных)
	const batchSize = 1000
	createdURLs, err := s.storageRepo.CreateBatch(ctx, userID, urls, batchSize)
	if err != nil {

		return nil, fmt.Errorf("create batch urls database: %w", err)
	}

	for _, url := range createdURLs {
		response = append(response, entity.BatchShortenResponse{
			CorrelationID: url.CorrelationID,
			ShortURL:      s.buildShortURL(url.ShortCode),
		})
	}

	return response, nil
}
