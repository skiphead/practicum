package repository

import (
	"context"
	"errors"

	"github.com/skiphead/practicum/internal/domain/entity"
)

// URLRepository aggregates all URL repository interfaces for convenience.
// This maintains backward compatibility while allowing clients to depend only
// on the specific interfaces they need.
type URLRepository interface {
	HealthChecker
	ErrorClassifier
	URLCreator
	BatchURLCreator
	URLRetriever
	URLReverseLookup
	UserURLRetriever
	URLUpdater
	URLDeleter
	DuplicateFinder
	BatchStatusUpdater
}

// Package-level error definitions for repository operations.
var (
	// ErrNotFound indicates that a requested resource was not found in the repository.
	// This error is returned when queries for specific entities yield no results.
	ErrNotFound = errors.New("not found")
)

// HealthChecker defines the interface for checking storage backend health.
type HealthChecker interface {
	// Ping checks the health and connectivity of the storage backend.
	// It verifies that the repository can communicate with its underlying storage.
	Ping(ctx context.Context) error
}

// ErrorClassifier defines the interface for error classification.
type ErrorClassifier interface {
	// IsDuplicateError determines if an error represents a duplicate key violation.
	// This is used to identify conflicts when inserting unique records.
	IsDuplicateError(err error) bool
}

// URLCreator defines the interface for creating shortened URLs.
type URLCreator interface {
	// Create saves a single shortened URL to the repository.
	// It associates the URL with a user and generates a unique short code.
	//
	// Parameters:
	//   - ctx: Context for timeout and cancellation
	//   - userID: ID of the user creating the URL
	//   - shortCode: Unique identifier for the shortened URL
	//   - originalURL: The original URL to be shortened
	//
	// Returns:
	//   - *entity.ShortURL: Created URL entity
	//   - error: Storage or validation error if creation fails
	Create(ctx context.Context, userID, shortCode, originalURL string) (*entity.ShortURL, error)
}

// BatchURLCreator defines the interface for batch URL creation.
type BatchURLCreator interface {
	// CreateBatch saves multiple shortened URLs in a batch operation.
	// It processes URLs efficiently and returns the created entities.
	//
	// Parameters:
	//   - ctx: Context for timeout and cancellation
	//   - userID: ID of the user creating the URLs
	//   - in: Slice of batch request items
	//   - batchSize: Number of records to process per database batch
	//
	// Returns:
	//   - []entity.ShortURL: Created URL entities
	//   - error: Storage or validation error if batch creation fails
	CreateBatch(ctx context.Context, userID string, in []entity.BatchShortenRequest, batchSize int) ([]entity.ShortURL, error)
}

// URLRetriever defines the interface for retrieving URLs by short code.
type URLRetriever interface {
	// Get retrieves a shortened URL by its short code.
	// It returns the full URL entity including metadata and usage statistics.
	//
	// Parameters:
	//   - ctx: Context for timeout and cancellation
	//   - shortCode: Unique short code identifier
	//
	// Returns:
	//   - *entity.ShortURL: Found URL entity
	//   - error: ErrNotFound if URL doesn't exist, or storage error
	Get(ctx context.Context, shortCode string) (*entity.ShortURL, error)
}

// URLReverseLookup defines the interface for reverse URL lookups.
type URLReverseLookup interface {
	// GetByOriginalURL retrieves a shortened URL by its original (long) URL.
	// This is used for duplicate detection and reverse lookups.
	//
	// Parameters:
	//   - ctx: Context for timeout and cancellation
	//   - originalURL: Original URL to search for
	//
	// Returns:
	//   - *entity.ShortURL: Found URL entity
	//   - error: ErrNotFound if URL doesn't exist, or storage error
	GetByOriginalURL(ctx context.Context, originalURL string) (*entity.ShortURL, error)
}

// UserURLRetriever defines the interface for retrieving user-specific URLs.
type UserURLRetriever interface {
	// GetByUserID retrieves all shortened URLs associated with a specific user.
	// This enables user-specific URL management and listing.
	//
	// Parameters:
	//   - ctx: Context for timeout and cancellation
	//   - userID: User ID to search for
	//
	// Returns:
	//   - []entity.ShortURL: Slice of URL entities belonging to the user
	//   - error: Storage error if retrieval fails
	GetByUserID(ctx context.Context, userID string) ([]entity.ShortURL, error)
}

// URLUpdater defines the interface for updating existing URLs.
type URLUpdater interface {
	// Update modifies an existing shortened URL record.
	// It allows updating URL properties while preserving core metadata.
	//
	// Parameters:
	//   - ctx: Context for timeout and cancellation
	//   - shortURL: Updated URL entity with changes
	//
	// Returns:
	//   - *entity.ShortURL: Updated URL entity
	//   - error: ErrNotFound if URL doesn't exist, or storage error
	Update(ctx context.Context, shortURL *entity.ShortURL) (*entity.ShortURL, error)
}

// URLDeleter defines the interface for deleting URLs.
type URLDeleter interface {
	// Delete removes a shortened URL from the repository by its ID.
	// This may be a hard or soft delete depending on implementation.
	//
	// Parameters:
	//   - ctx: Context for timeout and cancellation
	//   - id: Unique identifier of the URL to delete
	//
	// Returns:
	//   - string: ID of the deleted URL for confirmation
	//   - error: ErrNotFound if URL doesn't exist, or storage error
	Delete(ctx context.Context, id string) (string, error)
}

// DuplicateFinder defines the interface for finding duplicate URLs.
type DuplicateFinder interface {
	// FindDuplicateURLs identifies which URLs from a batch already exist in the repository.
	// This helps prevent duplicate URL creation in batch operations.
	//
	// Parameters:
	//   - ctx: Context for timeout and cancellation
	//   - urls: Slice of batch URL requests to check
	//
	// Returns:
	//   - []entity.ShortURL: Existing URL entities that match the provided URLs
	//   - error: Storage error if lookup fails
	FindDuplicateURLs(ctx context.Context, urls []entity.BatchShortenRequest) ([]entity.ShortURL, error)
}

// BatchStatusUpdater defines the interface for batch URL status updates.
type BatchStatusUpdater interface {
	// UpdateIsActive updates the active status of multiple URLs in a batch operation.
	// This implements batch logical deletion/restoration for user URLs.
	//
	// Parameters:
	//   - ctx: Context for timeout and cancellation
	//   - shortCodes: Slice of short codes to update
	//   - userID: User ID for authorization (only user's URLs can be updated)
	//   - isActive: New active status (true for active, false for deleted)
	//   - batchSize: Number of records to process per database batch
	//
	// Returns:
	//   - []string: Slice of successfully updated short codes
	//   - error: Storage or authorization error if update fails
	UpdateIsActive(ctx context.Context, shortCodes []string, userID string, isActive bool, batchSize int) ([]string, error)
}
