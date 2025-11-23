package usecase

import (
	"context"
	"fmt"
	"github.com/skiphead/practicum/internal/domain/entity"
	"github.com/skiphead/practicum/internal/domain/repository"
	"github.com/skiphead/practicum/pkg/utils"
	"go.uber.org/zap"
	"time"
)

const batchSize = 100

type URLUseCase interface {
	Ping(ctx context.Context) error
	IsDuplicateError(err error) bool
	Get(ctx context.Context, shortCode string) (*entity.ShortURL, error)
	GetByUserID(ctx context.Context, userID string) ([]entity.ShortURL, error)
	Save(ctx context.Context, originalURL, userID string) (*entity.ShortURL, error)
	BatchSave(ctx context.Context, in []entity.BatchShortenRequest, userID string) ([]entity.BatchShortenResponse, error)
	FindDuplicateURLs(ctx context.Context, urls []entity.BatchShortenRequest) ([]entity.BatchShortenResponse, error)
	Deleted(ctx context.Context, shortCodes []string, userID string) error
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
func (uc *urlUseCase) Ping(ctx context.Context) error {
	return uc.storageRepo.Ping(ctx)
}

// isDatabaseAvailable проверяет, доступна ли база данных
func (uc *urlUseCase) isDatabaseAvailable(ctx context.Context) bool {
	return uc.storageRepo.Ping(ctx) == nil
}

// buildShortURL создает полный короткий URL
func (uc *urlUseCase) buildShortURL(shortCode string) string {
	return fmt.Sprintf("%s/%s", uc.baseURL, shortCode)
}

// IsDuplicateError унифицированная проверка на ошибку дублирования
func (uc *urlUseCase) IsDuplicateError(err error) bool {

	return uc.storageRepo.IsDuplicateError(err)
}

// Save сохраняет оригинальный URL и возвращает сокращенную версию
func (uc *urlUseCase) Save(ctx context.Context, originalURL, userID string) (*entity.ShortURL, error) {
	shortCode := utils.GenerateRandomKey()

	if !uc.isDatabaseAvailable(ctx) {
		// Используем файловое хранилище как fallback
		if err := uc.fileStorage.Save(userID, "", shortCode, originalURL); err != nil {
			return nil, fmt.Errorf("save to file storage: %w", err)
		}

		return &entity.ShortURL{
			OriginalURL: originalURL,
			ShortCode:   shortCode,
		}, nil
	}

	// Используем основное хранилище (базу данных)
	shortURL, err := uc.storageRepo.Create(ctx, userID, shortCode, originalURL)
	if uc.storageRepo.IsDuplicateError(err) {
		duplicate, errGet := uc.storageRepo.GetByOriginalURL(ctx, originalURL)
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
func (uc *urlUseCase) Get(ctx context.Context, shortCode string) (*entity.ShortURL, error) {
	if !uc.isDatabaseAvailable(ctx) {
		// Используем файловое хранилище как fallback
		resp, exists, err := uc.fileStorage.Get(shortCode)
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
			IsActive:    resp.Deleted == false,
		}, nil
	}

	// Используем основное хранилище (базу данных)
	shortURL, err := uc.storageRepo.Get(ctx, shortCode)
	if err != nil {
		return nil, fmt.Errorf("get from database: %w", err)
	}

	return shortURL, nil
}

// GetByUserID возвращает список коротких url по id пользователя
func (uc *urlUseCase) GetByUserID(ctx context.Context, userID string) ([]entity.ShortURL, error) {
	var list []entity.ShortURL
	if !uc.isDatabaseAvailable(ctx) {
		// Используем файловое хранилище как fallback
		resp, err := uc.fileStorage.FindByUserID(userID)
		if err != nil {
			return nil, fmt.Errorf("get from file storage: %w", err)
		}

		for _, url := range resp {
			list = append(list, entity.ShortURL{
				ID:          url.UUID,
				OriginalURL: url.OriginalURL,
				ShortCode:   url.ShortURL,
				IsActive:    url.Deleted == false,
			})
		}

		return list, nil
	}

	// Используем основное хранилище (базу данных)
	shortURL, err := uc.storageRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get from database: %w", err)
	}

	return shortURL, nil
}

func (uc *urlUseCase) FindDuplicateURLs(ctx context.Context, urls []entity.BatchShortenRequest) ([]entity.BatchShortenResponse, error) {

	response := make([]entity.BatchShortenResponse, 0, len(urls))

	createdURLs, err := uc.storageRepo.FindDuplicateURLs(ctx, urls)
	if err != nil {
		return nil, fmt.Errorf("find duplicate URLs: %w", err)
	}

	for _, url := range createdURLs {
		response = append(response, entity.BatchShortenResponse{
			CorrelationID: url.CorrelationID,
			ShortURL:      uc.buildShortURL(url.ShortCode),
		})
	}

	return response, nil
}

// BatchSave сохраняет пакет URL и возвращает сокращенные версии
func (uc *urlUseCase) BatchSave(ctx context.Context, urls []entity.BatchShortenRequest, userID string) ([]entity.BatchShortenResponse, error) {
	if len(urls) == 0 {
		return []entity.BatchShortenResponse{}, nil
	}

	response := make([]entity.BatchShortenResponse, 0, len(urls))

	if !uc.isDatabaseAvailable(ctx) {
		// Используем файловое хранилище как fallback
		for _, item := range urls {
			code := utils.GenerateRandomKey()
			if err := uc.fileStorage.Save(userID, item.CorrelationID, code, item.OriginalURL); err != nil {
				return nil, fmt.Errorf("save batch item to file storage: %w", err)
			}

			response = append(response, entity.BatchShortenResponse{
				CorrelationID: item.CorrelationID,
				ShortURL:      uc.buildShortURL(code),
			})
		}
		return response, nil
	}

	// Используем основное хранилище (базу данных)

	createdURLs, err := uc.storageRepo.CreateBatch(ctx, userID, urls, batchSize)
	if err != nil {

		return nil, fmt.Errorf("create batch urls database: %w", err)
	}

	for _, url := range createdURLs {
		response = append(response, entity.BatchShortenResponse{
			CorrelationID: url.CorrelationID,
			ShortURL:      uc.buildShortURL(url.ShortCode),
		})
	}

	return response, nil
}

func (uc *urlUseCase) Deleted(ctx context.Context, shortCodes []string, userID string) error {
	if len(shortCodes) == 0 {
		return fmt.Errorf("empty short code")
	}
	if userID == "" {
		return fmt.Errorf("empty user ID")
	}

	// Если база недоступна, синхронно работаем с файловым хранилищем
	if !uc.isDatabaseAvailable(ctx) {
		if err := uc.fileStorage.SetDeletedByUserIDAndURLs(userID, shortCodes, true); err != nil {
			return fmt.Errorf("delete shortCode: %w", err)
		}
		return nil
	}

	// Запускаем фоновую обработку базы данных
	go uc.processDeletionInBackground(shortCodes, userID)

	return nil
}

func (uc *urlUseCase) processDeletionInBackground(shortCodes []string, userID string) {
	// Создаем отдельный контекст для фоновой операции
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Разбиваем на батчи
	batches := make([][]string, 0, (len(shortCodes)+batchSize-1)/batchSize)
	for i := 0; i < len(shortCodes); i += batchSize {
		end := i + batchSize
		if end > len(shortCodes) {
			end = len(shortCodes)
		}
		batches = append(batches, shortCodes[i:end])
	}

	// Fan-out: запускаем обработку батчей в отдельных горутинах
	results := make(chan struct {
		noFounds []string
		err      error
	}, len(batches))

	for _, batch := range batches {
		go func(b []string) {
			noFounds, err := uc.storageRepo.UpdateIsActive(ctx, b, userID, false, len(b))
			results <- struct {
				noFounds []string
				err      error
			}{noFounds, err}
		}(batch)
	}

	// Fan-in: собираем результаты из всех горутин
	var allNoFounds []string
	var errors []error

	for i := 0; i < len(batches); i++ {
		result := <-results
		if result.err != nil {
			errors = append(errors, result.err)
		}
		allNoFounds = append(allNoFounds, result.noFounds...)
	}
	close(results)

	// Логируем результаты фоновой операции
	if len(errors) > 0 {
		zap.L().Error("errors during background deletion",
			zap.Errors("errors", errors),
			zap.Strings("short_codes", shortCodes))
	}

	if len(allNoFounds) > 0 {
		zap.L().Warn("some short codes not found during deletion",
			zap.Strings("not_found_short_codes", allNoFounds))
	}
}
