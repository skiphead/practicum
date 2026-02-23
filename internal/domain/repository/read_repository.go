package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/skiphead/practicum/internal/domain/entity"
)

// Get retrieves a shortened URL record by its short code.
// It fetches the URL metadata from the database and validates its expiration.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - shortCode: Unique short code identifier for the URL
//
// Returns:
//   - *entity.ShortURL: Retrieved URL entity with metadata
//   - error: ErrNotFound if URL doesn't exist, expiry error, or database error
//
// The method performs the following checks:
// 1. Fetches URL data from database including active status
// 2. Validates the URL hasn't expired
// 3. Returns comprehensive error messages for different failure scenarios
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

	// Check expiration date
	if err := r.validateExpiry(expiresAt, shortCode); err != nil {
		return nil, err
	}

	return &shortURL, nil
}

// GetByOriginalURL retrieves a shortened URL record by its original (long) URL.
// This method is used for duplicate detection and reverse URL lookups.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - originalURL: The original URL to search for
//
// Returns:
//   - *entity.ShortURL: Retrieved URL entity with metadata
//   - error: ErrNotFound if URL doesn't exist, expiry error, or database error
//
// Note: This method assumes that original URLs have a uniqueness constraint
// in the database schema to prevent duplicate shortened URLs.
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
	// Check expiration date
	if err := r.validateExpiry(expiresAt, shortURL.ShortCode); err != nil {
		return nil, err
	}

	return &shortURL, nil
}

// GetByUserID retrieves all shortened URL records associated with a specific user.
// This enables user-specific URL management, listing, and dashboard functionality.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - userID: Unique identifier of the user whose URLs to retrieve
//
// Returns:
//   - []entity.ShortURL: Slice of URL entity belonging to the user
//   - error: Database query or scanning error if retrieval fails
//
// The method returns all URLs for the user regardless of their active status.
// Clients should filter the results based on IsActive field if needed.
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
