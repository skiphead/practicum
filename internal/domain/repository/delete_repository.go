package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Delete removes a shortened URL record from the database by its ID.
// It performs a soft or hard delete depending on the table structure.
// The method returns the ID of the deleted record for confirmation.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - id: Unique identifier of the URL to delete
//
// Returns:
//   - string: ID of the deleted URL record (empty if not found)
//   - error: Database error or ErrNotFound if the URL doesn't exist
//
// Note: This method performs a hard delete (permanent removal).
// For soft deletes, consider using UpdateIsActive instead.
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
