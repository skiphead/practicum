package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/skiphead/practicum/internal/domain/entity"
)

// FindDuplicateURLs ищет дубликаты URL в базе данных
func (r *storageRepository) FindDuplicateURLs(ctx context.Context, urls []entity.BatchShortenRequest) ([]entity.ShortURL, error) {
	if len(urls) == 0 {
		return []entity.ShortURL{}, nil
	}

	// Собираем все OriginalURL для проверки
	urlMap := make(map[string]bool)
	var placeholders []string
	var args []interface{}

	for _, url := range urls {
		// Используем map для уникальности URL
		if !urlMap[url.OriginalURL] {
			urlMap[url.OriginalURL] = true
			placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)+1))
			args = append(args, url.OriginalURL)
		}
	}

	if len(args) == 0 {
		return []entity.ShortURL{}, nil
	}

	// Формируем SQL запрос
	query := fmt.Sprintf(r.findDuplicatesQuery+"(%s)", strings.Join(placeholders, ", "))

	// Выполняем запрос
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("error querying database: %w", err)
	}
	defer rows.Close()

	var duplicates []entity.ShortURL

	for rows.Next() {
		var url entity.ShortURL
		err := rows.Scan(
			&url.ID,
			&url.CreatedAt,
			&url.ExpiresAt,
			&url.CorrelationID,
			&url.ShortCode,
			&url.OriginalURL,
			&url.UserID,
			&url.IsActive,
			&url.ClickCount,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}
		duplicates = append(duplicates, url)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return duplicates, nil
}
