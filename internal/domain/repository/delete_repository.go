package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Delete удаляет запись сокращенного URL по идентификатору.
func (r *storageRepository) Delete(ctx context.Context, id string) (string, error) {
	var deletedID string
	err := r.db.QueryRow(ctx, r.deleteQuery, id).Scan(&deletedID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("short URL with id '%s' not found: %w", id, ErrNotFound)
		}
		return "", fmt.Errorf("delete short URL: %w", err)
	}

	return deletedID, nil
}
