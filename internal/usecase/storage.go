// Package usecase implements the business logic layer for the URL shortening service.
// It orchestrates operations between domain entities and repositories, providing
// fallback mechanisms, batch processing, and transaction management for reliable URL operations.
package usecase

import (
	"context"
	"fmt"

	"github.com/skiphead/practicum/internal/domain/entity"
	"github.com/skiphead/practicum/internal/domain/repository"
	"github.com/skiphead/practicum/pkg/utils"

	"time"

	"go.uber.org/zap"
)

// Batch size for database operations to optimize performance.
const batchSize = 100

// ErrDuplicateURL indicates that a URL already exists in the system.
var ErrDuplicateURL = fmt.Errorf("duplicate URL")

// URLUseCase defines the business logic interface for URL shortening operations.
// It provides methods for URL management, batch operations, and fallback handling.
type URLUseCase interface {
	Ping(ctx context.Context) error
	IsDuplicateError(err error) bool
	Get(ctx context.Context, shortCode string) (*entity.ShortURL, error)
	GetByUserID(ctx context.Context, userID string) ([]entity.ShortURL, error)
	Save(ctx context.Context, originalURL, userID string) (*entity.ShortURL, error)
	BatchSave(ctx context.Context, in []entity.BatchShortenRequest, userID string) ([]entity.BatchShortenResponse, error)
	FindDuplicateURLs(ctx context.Context, urls []entity.BatchShortenRequest) ([]entity.BatchShortenResponse, error)
	Deleted(ctx context.Context, shortCodes []string, userID string) error
}

// urlUseCase implements business logic for shortened URL operations.
// It provides fallback mechanisms between database and file storage.
type urlUseCase struct {
	baseURL     string                   // Base URL for shortened links
	fileStorage repository.FileStorage   // Fallback file storage
	storageRepo repository.URLRepository // Primary database storage
}

// NewStorageUseCase creates a new URL use case instance.
// It sets up the business logic layer with both primary and fallback storage.
//
// Parameters:
//   - baseURL: Base URL for constructing shortened URLs
//   - fileStorage: Fallback file storage implementation
//   - urlRepository: Primary database repository implementation
//
// Returns:
//   - URLUseCase: Initialized use case with configured storage
func NewStorageUseCase(baseURL string,
	fileStorage repository.FileStorage,
	urlRepository repository.URLRepository) URLUseCase {

	return &urlUseCase{
		baseURL:     baseURL,
		fileStorage: fileStorage,
		storageRepo: urlRepository,
	}
}

// Ping checks the availability of the primary storage backend.
// This is used for health checks and availability determination.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//
// Returns:
//   - error: Connection error if primary storage is unavailable
func (uc *urlUseCase) Ping(ctx context.Context) error {
	return uc.storageRepo.Ping(ctx)
}

// isDatabaseAvailable checks if the primary database storage is accessible.
// Used to determine whether to use primary storage or fallback.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//
// Returns:
//   - bool: True if database is accessible, false otherwise
func (uc *urlUseCase) isDatabaseAvailable(ctx context.Context) bool {
	return uc.storageRepo.Ping(ctx) == nil
}

// buildShortURL constructs a complete shortened URL from a short code.
//
// Parameters:
//   - shortCode: Unique identifier for the shortened URL
//
// Returns:
//   - string: Complete shortened URL in format "baseURL/shortCode"
func (uc *urlUseCase) buildShortURL(shortCode string) string {
	return fmt.Sprintf("%s/%s", uc.baseURL, shortCode)
}

// IsDuplicateError provides unified duplicate error checking.
// It delegates to the repository's duplicate detection logic.
//
// Parameters:
//   - err: Error to check for duplicate violations
//
// Returns:
//   - bool: True if the error represents a duplicate violation
func (uc *urlUseCase) IsDuplicateError(err error) bool {
	return uc.storageRepo.IsDuplicateError(err)
}

// Save creates a shortened URL for an original URL with user ownership.
// It automatically falls back to file storage if database is unavailable.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - originalURL: URL to be shortened
//   - userID: ID of the user creating the URL
//
// Returns:
//   - *entity.ShortURL: Created or existing shortened URL
//   - error: Storage or duplicate error if creation fails
//
// Behavior:
//  1. Checks database availability
//  2. Falls back to file storage if database is unavailable
//  3. Handles duplicate URLs by returning existing entries
func (uc *urlUseCase) Save(ctx context.Context, originalURL, userID string) (*entity.ShortURL, error) {
	shortCode := utils.GenerateRandomKey()

	if !uc.isDatabaseAvailable(ctx) {
		// Use file storage as fallback
		if err := uc.fileStorage.Save(userID, "", shortCode, originalURL); err != nil {
			return nil, fmt.Errorf("save to file storage: %w", err)
		}

		return &entity.ShortURL{
			OriginalURL: originalURL,
			ShortCode:   shortCode,
		}, nil
	}

	// Use primary storage (database)
	shortURL, err := uc.storageRepo.Create(ctx, userID, shortCode, originalURL)
	if uc.storageRepo.IsDuplicateError(err) {
		duplicate, errGet := uc.storageRepo.GetByOriginalURL(ctx, originalURL)
		if errGet != nil {
			return nil, errGet
		}

		return duplicate, err
	}

	if err != nil {
		return nil, fmt.Errorf("create in database: %w", err)
	}

	return shortURL, nil
}

// Get retrieves a shortened URL by its short code.
// It falls back to file storage if database is unavailable.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - shortCode: Unique short code identifier
//
// Returns:
//   - *entity.ShortURL: Retrieved URL entity
//   - error: ErrNotFound if URL doesn't exist, or storage error
func (uc *urlUseCase) Get(ctx context.Context, shortCode string) (*entity.ShortURL, error) {
	if !uc.isDatabaseAvailable(ctx) {
		// Use file storage as fallback
		resp, exists, err := uc.fileStorage.Get(shortCode)
		if err != nil {
			return nil, fmt.Errorf("get from file storage: %w", err)
		}
		if !exists {
			return nil, fmt.Errorf("short URL with code '%s' not found", shortCode)
		}

		return resp, nil
	}

	// Use primary storage (database)
	shortURL, err := uc.storageRepo.Get(ctx, shortCode)
	if err != nil {
		return nil, fmt.Errorf("get from database: %w", err)
	}

	return shortURL, nil
}

// GetByUserID retrieves all shortened URLs for a specific user.
// It falls back to file storage if database is unavailable.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - userID: User ID to search for
//
// Returns:
//   - []entity.ShortURL: Slice of URL entities belonging to the user
//   - error: Storage error if retrieval fails
func (uc *urlUseCase) GetByUserID(ctx context.Context, userID string) ([]entity.ShortURL, error) {
	if !uc.isDatabaseAvailable(ctx) {
		// Use file storage as fallback
		resp, err := uc.fileStorage.FindByUserID(userID)
		if err != nil {
			return nil, fmt.Errorf("get from file storage: %w", err)
		}

		return resp, nil
	}

	// Use primary storage (database)
	shortURL, err := uc.storageRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get from database: %w", err)
	}

	return shortURL, nil
}

// FindDuplicateURLs identifies which URLs from a batch already exist in storage.
// This prevents duplicate URL creation in batch operations.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - urls: Batch of URL requests to check
//
// Returns:
//   - []entity.BatchShortenResponse: Responses for existing URLs
//   - error: Storage error if lookup fails
func (uc *urlUseCase) FindDuplicateURLs(ctx context.Context, urls []entity.BatchShortenRequest) ([]entity.BatchShortenResponse, error) {
	response := make([]entity.BatchShortenResponse, 0, len(urls))

	createdURLs, err := uc.storageRepo.FindDuplicateURLs(ctx, urls)
	if err != nil {
		return nil, fmt.Errorf("find duplicate URLs: %w", err)
	}

	for _, url := range createdURLs {
		response = append(response, entity.BatchShortenResponse{
			CorrelationID: url.CorrelationID,
			ShortURL:      uc.buildShortURL(url.ShortCode),
		})
	}

	return response, nil
}

// BatchSave creates multiple shortened URLs in a single operation.
// It falls back to file storage if database is unavailable.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - urls: Batch of URL requests to process
//   - userID: ID of the user creating the URLs
//
// Returns:
//   - []entity.BatchShortenResponse: Responses for created URLs
//   - error: Storage error if batch creation fails
//
// Note: Each URL gets a unique correlation ID to match requests with responses.
func (uc *urlUseCase) BatchSave(ctx context.Context, urls []entity.BatchShortenRequest, userID string) ([]entity.BatchShortenResponse, error) {
	if len(urls) == 0 {
		return []entity.BatchShortenResponse{}, nil
	}

	response := make([]entity.BatchShortenResponse, 0, len(urls))

	if !uc.isDatabaseAvailable(ctx) {
		// Use file storage as fallback
		for _, item := range urls {
			code := utils.GenerateRandomKey()
			if err := uc.fileStorage.Save(userID, item.CorrelationID, code, item.OriginalURL); err != nil {
				return nil, fmt.Errorf("save batch item to file storage: %w", err)
			}

			response = append(response, entity.BatchShortenResponse{
				CorrelationID: item.CorrelationID,
				ShortURL:      uc.buildShortURL(code),
			})
		}
		return response, nil
	}

	// Use primary storage (database)
	createdURLs, err := uc.storageRepo.CreateBatch(ctx, userID, urls, batchSize)
	if err != nil {
		return nil, fmt.Errorf("create batch urls database: %w", err)
	}

	for _, url := range createdURLs {
		response = append(response, entity.BatchShortenResponse{
			CorrelationID: url.CorrelationID,
			ShortURL:      uc.buildShortURL(url.ShortCode),
		})
	}

	return response, nil
}

// Deleted performs logical deletion of URLs for a specific user.
// It processes deletions asynchronously in the background for better performance.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - shortCodes: Short codes to mark as deleted
//   - userID: User ID for authorization
//
// Returns:
//   - error: Validation error if inputs are invalid
//
// Behavior:
//  1. For file storage: Synchronous deletion
//  2. For database: Asynchronous batch deletion with fan-out/fan-in pattern
//  3. Returns immediately for database operations (fire-and-forget)
func (uc *urlUseCase) Deleted(ctx context.Context, shortCodes []string, userID string) error {
	if len(shortCodes) == 0 {
		return fmt.Errorf("empty short code")
	}
	if userID == "" {
		return fmt.Errorf("empty user ID")
	}

	// If database is unavailable, work synchronously with file storage
	if !uc.isDatabaseAvailable(ctx) {
		if err := uc.fileStorage.SetDeletedByUserIDAndURLs(userID, shortCodes, true); err != nil {
			return fmt.Errorf("delete shortCode: %w", err)
		}
		return nil
	}

	// Start background processing for database deletion
	go uc.processDeletionInBackground(shortCodes, userID)

	return nil
}

// processDeletionInBackground asynchronously processes URL deletions in batches.
// It uses a fan-out/fan-in pattern for parallel processing of deletion batches.
//
// Parameters:
//   - shortCodes: Short codes to delete
//   - userID: User ID for authorization
//
// This method:
// 1. Splits short codes into batches
// 2. Processes batches concurrently in goroutines
// 3. Collects and logs results
// 4. Implements 30-second timeout for the entire operation
func (uc *urlUseCase) processDeletionInBackground(shortCodes []string, userID string) {
	// Create separate context for background operation
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Split into batches
	batches := make([][]string, 0, (len(shortCodes)+batchSize-1)/batchSize)
	for i := 0; i < len(shortCodes); i += batchSize {
		end := i + batchSize
		if end > len(shortCodes) {
			end = len(shortCodes)
		}
		batches = append(batches, shortCodes[i:end])
	}

	// Fan-out: start batch processing in separate goroutines
	results := make(chan struct {
		noFounds []string
		err      error
	}, len(batches))

	for _, batch := range batches {
		go func(b []string) {
			noFounds, err := uc.storageRepo.UpdateIsActive(ctx, b, userID, false, len(b))
			results <- struct {
				noFounds []string
				err      error
			}{noFounds, err}
		}(batch)
	}

	// Fan-in: collect results from all goroutines
	var allNoFounds []string
	var errors []error

	for i := 0; i < len(batches); i++ {
		result := <-results
		if result.err != nil {
			errors = append(errors, result.err)
		}
		allNoFounds = append(allNoFounds, result.noFounds...)
	}
	close(results)

	// Log background operation results
	if len(errors) > 0 {
		zap.L().Error("errors during background deletion",
			zap.Errors("errors", errors),
			zap.Strings("short_codes", shortCodes))
	}

	if len(allNoFounds) > 0 {
		zap.L().Warn("some short codes not found during deletion",
			zap.Strings("not_found_short_codes", allNoFounds))
	}
}
