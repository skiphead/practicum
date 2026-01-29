package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/skiphead/practicum/internal/domain/entity"
)

// Update updates an existing shortened URL record in the database.
// It modifies the original URL and short code while preserving the record ID.
// This method is typically used for URL redirection updates or metadata changes.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - shortURL: Updated URL entity with modified fields
//
// Returns:
//   - *entity.ShortURL: Updated URL entity with refreshed timestamps
//   - error: ErrNotFound if URL doesn't exist, or database error
//
// Note: The method updates only the mutable fields (OriginalURL and ShortCode).
// Other fields like CreatedAt, UserID, and expiration remain unchanged.
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

// UpdateIsActive sets the is_active flag to true or false for user-specific short codes.
// This implements batch logical deletion/restoration of user URLs.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - shortCodes: Slice of short codes to update
//   - userID: User ID for authorization (only user's URLs can be updated)
//   - isActive: New active status (true for active, false for logically deleted)
//   - batchSize: Number of records to process per database batch
//
// Returns:
//   - []string: Slice of short codes that were not found or don't belong to the user
//   - error: Validation or database error if the operation fails
//
// The method processes updates in batches to optimize database performance.
// It returns any short codes that couldn't be updated (not found or unauthorized).
// Use this method for soft delete/restore operations rather than physical deletion.
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

	// Split into batches of batchSize elements
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

// processBatchUpdateIsActive processes a single batch of short code updates.
// It uses pgx batch operations for efficient database updates.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - shortCodes: Batch of short codes to update
//   - userID: User ID for authorization
//   - isActive: New active status to set
//
// Returns:
//   - []string: Short codes in this batch that were not found
//   - error: Database error if batch processing fails
//
// The method executes all updates in a single database batch operation,
// reducing network round trips and improving performance.
func (r *storageRepository) processBatchUpdateIsActive(ctx context.Context, shortCodes []string, userID string, isActive bool) ([]string, error) {
	batch := &pgx.Batch{}

	// Add all queries to the batch
	for _, shortCode := range shortCodes {
		batch.Queue(r.updateIsActive, userID, shortCode, isActive)
	}

	// Execute batch query
	results := r.db.SendBatch(ctx, batch)
	defer results.Close()

	var notFound []string

	// Process results
	for i := 0; i < batch.Len(); i++ {
		_, err := results.Exec()
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// Short code not found or doesn't belong to user
				notFound = append(notFound, shortCodes[i])
			} else {
				return nil, fmt.Errorf("update short URL: %w", err)
			}
		}
	}

	return notFound, nil
}
