package repository

import (
	"context"
	"errors"

	"github.com/skiphead/practicum/internal/domain/entity"
)

// Package-level error definitions for repository operations.
var (
	// ErrNotFound indicates that a requested resource was not found in the repository.
	// This error is returned when queries for specific entities yield no results.
	ErrNotFound = errors.New("not found")
)

// URLRepository defines the interface for URL storage operations.
// It provides methods for CRUD operations, batch processing, and duplicate detection
// for shortened URLs. Implementations can use various storage backends
// (database, file system, in-memory, etc.).
type URLRepository interface {
	// Ping checks the health and connectivity of the storage backend.
	// It verifies that the repository can communicate with its underlying storage.
	Ping(ctx context.Context) error

	// IsDuplicateError determines if an error represents a duplicate key violation.
	// This is used to identify conflicts when inserting unique records.
	IsDuplicateError(err error) bool

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
