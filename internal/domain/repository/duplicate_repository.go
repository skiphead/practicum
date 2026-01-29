package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/skiphead/practicum/internal/domain/entity"
)

// FindDuplicateURLs searches for duplicate URLs in the database from a batch of URL requests.
// It checks which URLs from the incoming batch already exist in the database to prevent duplicates.
// This is useful for batch operations where users might resubmit the same URLs.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - urls: Slice of batch URL requests to check for duplicates
//
// Returns:
//   - []entity.ShortURL: Slice of existing URL entities that match the provided URLs
//   - error: Database query or scanning error if operation fails
//
// The method:
// 1. Deduplicates the input URLs to avoid redundant database checks
// 2. Builds a parameterized IN clause query for efficient batch lookup
// 3. Returns full ShortURL entities for any duplicates found
//
// Note: This method only checks for exact URL matches, not short code collisions.
func (r *storageRepository) FindDuplicateURLs(ctx context.Context, urls []entity.BatchShortenRequest) ([]entity.ShortURL, error) {
	if len(urls) == 0 {
		return []entity.ShortURL{}, nil
	}

	// Collect all OriginalURLs for duplicate checking
	urlMap := make(map[string]bool)
	var placeholders []string
	var args []interface{}

	for _, url := range urls {
		// Use map for URL uniqueness
		if !urlMap[url.OriginalURL] {
			urlMap[url.OriginalURL] = true
			placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)+1))
			args = append(args, url.OriginalURL)
		}
	}

	if len(args) == 0 {
		return []entity.ShortURL{}, nil
	}

	// Build SQL query with IN clause
	query := fmt.Sprintf(r.findDuplicatesQuery+"(%s)", strings.Join(placeholders, ", "))

	// Execute query
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
