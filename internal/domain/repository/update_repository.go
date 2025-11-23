package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/skiphead/practicum/internal/domain/entity"
)

// Update обновляет существующую запись сокращенного URL в базе данных.
func (r *storageRepository) Update(ctx context.Context, shortURL *entity.ShortURL) (*entity.ShortURL, error) {
	var updatedURL entity.ShortURL

	err := r.db.QueryRow(
		ctx,
		r.updateQuery,
		shortURL.OriginalURL,
		shortURL.ShortCode,
		shortURL.ID,
	).Scan(
		&updatedURL.ID,
		&updatedURL.OriginalURL,
		&updatedURL.ShortCode,
		&updatedURL.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("short URL with id '%s' not found: %w", shortURL.ID, ErrNotFound)
		}
		return nil, fmt.Errorf("update short URL: %w", err)
	}

	return &updatedURL, nil
}

// UpdateIsActive устанавливает is_active в true или false для user_id и short_code.
// Возвращает список не найденных кодов и ошибку (если есть).
func (r *storageRepository) UpdateIsActive(ctx context.Context, shortCodes []string, userID string, isActive bool, batchSize int) ([]string, error) {
	if userID == "" {
		return nil, fmt.Errorf("userID cannot be empty")
	}
	if len(shortCodes) == 0 {
		return nil, fmt.Errorf("shortCodes cannot be empty")
	}
	if batchSize <= 0 {
		return nil, fmt.Errorf("batchSize must be positive")
	}
	var notFound []string

	// Разбиваем на пакеты по batchSize элементов
	for i := 0; i < len(shortCodes); i += batchSize {
		end := i + batchSize
		if end > len(shortCodes) {
			end = len(shortCodes)
		}
		batchCodes := shortCodes[i:end]
		batchNotFound, err := r.processBatchUpdateIsActive(ctx, batchCodes, userID, isActive)
		if err != nil {
			return nil, err
		}

		notFound = append(notFound, batchNotFound...)
	}

	return notFound, nil
}

// processBatchUpdateIsActive обрабатывает один пакет кодов
func (r *storageRepository) processBatchUpdateIsActive(ctx context.Context, shortCodes []string, userID string, isActive bool) ([]string, error) {
	batch := &pgx.Batch{}

	// Добавляем все запросы в пакет
	for _, shortCode := range shortCodes {
		batch.Queue(r.updateIsActive, userID, shortCode, isActive)
	}

	// Выполняем пакетный запрос
	results := r.db.SendBatch(ctx, batch)
	defer results.Close()

	var notFound []string

	// Обрабатываем результаты
	for i := 0; i < batch.Len(); i++ {
		_, err := results.Exec()
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// Тот случай когда код не найден
				notFound = append(notFound, shortCodes[i])
			} else {
				return nil, fmt.Errorf("update short URL: %w", err)
			}
		}
	}

	return notFound, nil
}
