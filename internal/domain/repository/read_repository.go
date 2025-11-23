package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/skiphead/practicum/internal/domain/entity"
)

// Get возвращает запись сокращенного URL по его короткому коду.
func (r *storageRepository) Get(ctx context.Context, shortCode string) (*entity.ShortURL, error) {
	var shortURL entity.ShortURL
	var expiresAt time.Time

	err := r.db.QueryRow(ctx, r.getQuery, shortCode).Scan(
		&shortURL.ID,
		&shortURL.OriginalURL,
		&shortURL.ShortCode,
		&shortURL.CreatedAt,
		&shortURL.IsActive,
		&expiresAt,
	)
	if err != nil {
		return r.handleQueryError(err, "short URL for original URL", shortCode)
	}

	// Проверяем срок действия
	if err := r.validateExpiry(expiresAt, shortCode); err != nil {
		return nil, err
	}

	return &shortURL, nil
}

// GetByOriginalURL возвращает запись сокращенного URL по его оригинальной ссылке.
func (r *storageRepository) GetByOriginalURL(ctx context.Context, originalURL string) (*entity.ShortURL, error) {
	var shortURL entity.ShortURL
	var expiresAt time.Time

	err := r.db.QueryRow(ctx, r.getByOriginalURL, originalURL).Scan(
		&shortURL.ID,
		&shortURL.OriginalURL,
		&shortURL.ShortCode,
		&shortURL.CreatedAt,
		&expiresAt,
	)
	if err != nil {
		return r.handleQueryError(err, "short URL for original URL", originalURL)
	}
	// Проверяем срок действия
	if err := r.validateExpiry(expiresAt, shortURL.ShortCode); err != nil {
		return nil, err
	}

	return &shortURL, nil
}

// GetByUserID возвращает запись сокращенного URL по его оригинальной ссылке.
func (r *storageRepository) GetByUserID(ctx context.Context, userID string) ([]entity.ShortURL, error) {
	var shortURL entity.ShortURL
	var expiresAt time.Time

	rows, err := r.db.Query(ctx, r.getByUserID, userID)
	if err != nil {
		return nil, fmt.Errorf("error querying database: %w", err)
	}
	var list []entity.ShortURL
	for rows.Next() {
		errScan := rows.Scan(
			&shortURL.ID,
			&shortURL.OriginalURL,
			&shortURL.ShortCode,
			&shortURL.CreatedAt,
			&shortURL.IsActive,
			&expiresAt,
		)
		if errScan != nil {
			return nil, fmt.Errorf("error scanning row: %w", errScan)
		}
		list = append(list, shortURL)
	}

	return list, nil
}
