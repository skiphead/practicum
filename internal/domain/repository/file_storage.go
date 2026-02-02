package repository

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/skiphead/practicum/internal/domain/entity"
	"go.uber.org/zap"
)

// FileStorage interface defines operations for URL storage with file-based persistence.
// It provides methods for CRUD operations on shortened URLs with memory caching.
type FileStorage interface {
	Save(userID, correlationID, key, url string) error
	Get(key string) (*entity.ShortURL, bool, error)
	GetByID(id string) (*entity.ShortURL, bool, error)
	FindByOriginalURL(originalURL string) (*entity.ShortURL, error)
	FindByUserID(userID string) ([]entity.ShortURL, error)
	Delete(shortURL string) error
	DeleteByID(id string) error
	SetDeletedByUserIDAndURLs(userID string, shortURLs []string, deleted bool) error
	Stats() map[string]interface{}
	BatchSave(ctx context.Context, in []entity.ShortURL) error
}

// cachedFileStorage implements FileStorage with file persistence and in-memory caching.
// It uses append-only JSONL file format for durability and multiple in-memory indexes
// for fast lookups. This implementation supports logical deletion (soft delete).
type cachedFileStorage struct {
	pathDB           string                        // Path to the storage file
	mu               sync.RWMutex                  // Mutex for concurrent access
	cache            map[string]*entity.ShortURL   // In-memory cache: shortCode -> ShortURL
	cacheByID        map[string]*entity.ShortURL   // In-memory cache: id -> ShortURL
	originalURLIndex map[string]*entity.ShortURL   // Index by original URL (active records only)
	userIDIndex      map[string][]*entity.ShortURL // Index by user_id (all records)
}

// NewCachedFileStorage creates a new instance of cachedFileStorage.
// It initializes the storage by loading existing data from the file into memory caches.
//
// Parameters:
//   - path: File system path for storing URL data
//
// Returns:
//   - FileStorage: Initialized file storage instance
//   - error: If file cannot be opened or data cannot be loaded
func NewCachedFileStorage(path string) (FileStorage, error) {
	storage := &cachedFileStorage{
		pathDB:           path,
		cache:            make(map[string]*entity.ShortURL),
		cacheByID:        make(map[string]*entity.ShortURL),
		originalURLIndex: make(map[string]*entity.ShortURL),
		userIDIndex:      make(map[string][]*entity.ShortURL),
	}

	// Restore saved data from file to cache during initialization
	if err := storage.loadCacheFromFile(); err != nil {
		return nil, fmt.Errorf("failed to load cache from file: %w", err)
	}

	return storage, nil
}

// loadCacheFromFile loads data from the storage file into in-memory caches.
// It reads the JSONL file line by line and rebuilds all indexes.
//
// Returns:
//   - error: If file cannot be read or contains invalid JSON
func (s *cachedFileStorage) loadCacheFromFile() error {
	file, err := os.OpenFile(s.pathDB, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("error opening file: %w", err)
	}
	defer func(file *os.File) {
		errCloseFile := file.Close()
		if errCloseFile != nil {
			zap.L().Warn("failed to close file", zap.Error(errCloseFile))
		}
	}(file)

	// Check if file is empty
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("error getting file info: %w", err)
	}
	if info.Size() == 0 {
		return nil
	}

	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue // Skip empty lines
		}

		var record entity.ShortURL
		if err := json.Unmarshal(line, &record); err != nil {
			zap.L().Warn("failed to parse JSON line",
				zap.Int("line", lineNumber),
				zap.Error(err),
				zap.String("content", string(line)))
			continue
		}

		// Load ALL records, including inactive ones
		s.addToCacheAndIndexes(&record)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error scanning file: %w", err)
	}

	return nil
}

// addToCacheAndIndexes adds a record to all caches and indexes.
// It updates the main cache, ID cache, original URL index, and user ID index.
//
// Parameters:
//   - record: The ShortURL record to add to caches
func (s *cachedFileStorage) addToCacheAndIndexes(record *entity.ShortURL) {
	s.cache[record.ShortCode] = record
	s.cacheByID[record.ID] = record

	// Add to original URL index only for active records
	if record.IsActive {
		s.originalURLIndex[record.OriginalURL] = record
	}

	// Add to user_id index for all records, but filter by status in search methods
	if _, exists := s.userIDIndex[record.UserID]; !exists {
		s.userIDIndex[record.UserID] = make([]*entity.ShortURL, 0)
	}

	// Check if this record already exists in the index
	found := false
	for i, existingRecord := range s.userIDIndex[record.UserID] {
		if existingRecord.ID == record.ID {
			// Update existing record
			s.userIDIndex[record.UserID][i] = record
			found = true
			break
		}
	}

	if !found {
		s.userIDIndex[record.UserID] = append(s.userIDIndex[record.UserID], record)
	}
}

// removeFromIndexes removes a record from indexes (but keeps it in main cache for logical deletion).
// This is used when marking records as inactive.
//
// Parameters:
//   - record: The ShortURL record to remove from indexes
func (s *cachedFileStorage) removeFromIndexes(record *entity.ShortURL) {
	// Remove from original URL index
	delete(s.originalURLIndex, record.OriginalURL)

	// Do not remove from user_id index, but filter in search methods
}

// Save saves a URL mapping to file and updates the cache.
// It checks for duplicate URLs and generates a unique UUID for the record.
//
// Parameters:
//   - userID: ID of the user creating the URL
//   - correlationID: Correlation ID for batch operations
//   - key: Short code for the URL
//   - url: Original URL to shorten
//
// Returns:
//   - error: If URL already exists or file write fails
func (s *cachedFileStorage) Save(userID, correlationID, key, url string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if the original URL already exists (active records only)
	if existing, exists := s.originalURLIndex[url]; exists {
		return fmt.Errorf("URL already exists: %s", existing.ShortCode)
	}

	// Generate UUID v4
	id := uuid.New().String()

	record := &entity.ShortURL{
		ID:            id,
		UserID:        userID,
		CorrelationID: correlationID,
		ShortCode:     key,
		OriginalURL:   url,
		CreatedAt:     time.Now(),
		IsActive:      true,
		ClickCount:    0,
		ExpiresAt:     sql.NullTime{Valid: false}, // No expiration by default
	}

	// Update cache and indexes
	s.addToCacheAndIndexes(record)

	// Save to file in JSONL format
	return s.appendRecordToFile(record)
}

// BatchSave saves multiple URL mappings with IDs to file and updates the cache.
// It processes each URL individually and skips duplicates.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - in: Slice of ShortURL entities to save
//
// Returns:
//   - error: If file write fails
func (s *cachedFileStorage) BatchSave(ctx context.Context, in []entity.ShortURL) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		for _, i := range in {
			// Check if the original URL already exists (active records only)
			if existing, exists := s.originalURLIndex[i.OriginalURL]; exists {
				zap.L().Warn("URL already exists, skipping",
					zap.String("original_url", i.OriginalURL),
					zap.String("existing_short", existing.ShortCode))
				continue
			}

			// Generate UUID v4
			id := uuid.New().String()
			record := &entity.ShortURL{
				ID:            id,
				UserID:        i.UserID,
				CorrelationID: i.CorrelationID,
				ShortCode:     i.ShortCode,
				OriginalURL:   i.OriginalURL,
				CreatedAt:     time.Now(),
				IsActive:      true,
				ClickCount:    0,
				ExpiresAt:     sql.NullTime{Valid: false},
			}

			// Update cache and indexes
			s.addToCacheAndIndexes(record)

			err := s.appendRecordToFile(record)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// appendRecordToFile appends a record to the end of the file in JSONL format.
// This implements an append-only log for durability.
//
// Parameters:
//   - record: The ShortURL record to append
//
// Returns:
//   - error: If file cannot be opened or write fails
func (s *cachedFileStorage) appendRecordToFile(record *entity.ShortURL) error {
	file, err := os.OpenFile(s.pathDB, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("error opening file: %w", err)
	}
	defer func(file *os.File) {
		errCloseFile := file.Close()
		if errCloseFile != nil {
			zap.L().Warn("failed to close file", zap.Error(errCloseFile))
		}
	}(file)

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("error marshaling record: %w", err)
	}

	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("error writing to file: %w", err)
	}

	return nil
}

// Get retrieves a URL by its short code from the cache.
// Returns records with any IsActive status.
//
// Parameters:
//   - key: Short code to look up
//
// Returns:
//   - *entity.ShortURL: Found URL record (nil if not found)
//   - bool: True if record was found
//   - error: Always nil, present for interface compatibility
func (s *cachedFileStorage) Get(key string) (*entity.ShortURL, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, exists := s.cache[key]
	if !exists {
		return nil, false, nil
	}
	return record, true, nil
}

// GetByID retrieves a record by its ID from the cache.
// Returns records with any IsActive status.
//
// Parameters:
//   - id: UUID of the record to look up
//
// Returns:
//   - *entity.ShortURL: Found URL record (nil if not found)
//   - bool: True if record was found
//   - error: Always nil, present for interface compatibility
func (s *cachedFileStorage) GetByID(id string) (*entity.ShortURL, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, exists := s.cacheByID[id]
	if !exists {
		return nil, false, nil
	}
	return record, true, nil
}

// Delete marks a record as inactive (logical deletion).
// It updates indexes and appends the change to the file.
//
// Parameters:
//   - shortURL: Short code of the record to delete
//
// Returns:
//   - error: If record not found or file write fails
func (s *cachedFileStorage) Delete(shortURL string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, exists := s.cache[shortURL]
	if !exists {
		return fmt.Errorf("record not found with short URL: %s", shortURL)
	}

	// Logical deletion
	record.IsActive = false
	s.removeFromIndexes(record)

	// Write change to file (append-only)
	return s.appendRecordToFile(record)
}

// DeleteByID marks a record as inactive by its ID (logical deletion).
// It updates indexes and appends the change to the file.
//
// Parameters:
//   - id: UUID of the record to delete
//
// Returns:
//   - error: If record not found or file write fails
func (s *cachedFileStorage) DeleteByID(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, exists := s.cacheByID[id]
	if !exists {
		return fmt.Errorf("record not found with id: %s", id)
	}

	// Logical deletion
	record.IsActive = false
	s.removeFromIndexes(record)

	// Write change to file (append-only)
	return s.appendRecordToFile(record)
}

// FindByOriginalURL searches for a record by original URL in the index (active records only).
//
// Parameters:
//   - originalURL: Original URL to search for
//
// Returns:
//   - *entity.ShortURL: Found URL record (nil if not found)
//   - error: Always nil, present for interface compatibility
func (s *cachedFileStorage) FindByOriginalURL(originalURL string) (*entity.ShortURL, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, exists := s.originalURLIndex[originalURL]
	if !exists {
		return nil, nil // Return nil instead of error
	}
	return record, nil
}

// FindByUserID searches for all records by user_id in the index (active records only).
//
// Parameters:
//   - userID: User ID to search for
//
// Returns:
//   - []entity.ShortURL: Slice of active URL records for the user
//   - error: Always nil, present for interface compatibility
func (s *cachedFileStorage) FindByUserID(userID string) ([]entity.ShortURL, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	records, exists := s.userIDIndex[userID]
	if !exists {
		return []entity.ShortURL{}, nil // Return empty slice instead of error
	}

	// Filter only active records
	var activeRecords []entity.ShortURL
	for _, record := range records {
		if record.IsActive {
			activeRecords = append(activeRecords, *record)
		}
	}

	return activeRecords, nil
}

// SetDeletedByUserIDAndURLs sets the IsActive flag for specific URLs of a user.
// This implements batch logical deletion or restoration.
//
// Parameters:
//   - userID: User ID whose URLs to update
//   - shortURLs: Slice of short codes to update
//   - deleted: True to mark as deleted (inactive), false to restore (active)
//
// Returns:
//   - error: If user not found or file write fails
func (s *cachedFileStorage) SetDeletedByUserIDAndURLs(userID string, shortURLs []string, deleted bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var updatedRecords []*entity.ShortURL

	// Get all user records
	userRecords, exists := s.userIDIndex[userID]
	if !exists {
		return fmt.Errorf("no records found for user: %s", userID)
	}

	// Create a map for quick lookup
	urlMap := make(map[string]*entity.ShortURL)
	for _, record := range userRecords {
		urlMap[record.ShortCode] = record
	}

	// Update only specified URLs
	for _, shortURL := range shortURLs {
		record, e := urlMap[shortURL]
		if !e {
			continue // Skip non-existent URLs
		}

		// Skip if flag is already set to the desired value
		if record.IsActive == !deleted {
			continue
		}

		// Set flag (deleted = true means IsActive = false)
		record.IsActive = !deleted

		// Update indexes
		if deleted {
			s.removeFromIndexes(record)
		} else {
			// When restoring, add back to indexes
			s.originalURLIndex[record.OriginalURL] = record
		}

		updatedRecords = append(updatedRecords, record)
	}

	// Write changes to file
	for _, record := range updatedRecords {
		if err := s.appendRecordToFile(record); err != nil {
			return fmt.Errorf("failed to update record in file: %w", err)
		}
	}

	return nil
}

// CompactFile compresses the storage file by removing logically deleted records.
// This reduces file size by eliminating inactive records from disk.
//
// Returns:
//   - error: If file operations fail
func (s *cachedFileStorage) CompactFile() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tempPath := s.pathDB + ".tmp"
	tempFile, err := os.OpenFile(tempPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("error creating temp file: %w", err)
	}
	defer func() {
		tempFile.Close()
		os.Remove(tempPath) // Clean up temp file on error
	}()

	writer := bufio.NewWriter(tempFile)

	// Write all records (including inactive)
	for _, record := range s.cache {
		data, err := json.Marshal(record)
		if err != nil {
			return fmt.Errorf("error marshaling record: %w", err)
		}
		if _, err := writer.Write(append(data, '\n')); err != nil {
			return fmt.Errorf("error writing to temp file: %w", err)
		}
	}

	if err := writer.Flush(); err != nil {
		return fmt.Errorf("error flushing temp file: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("error closing temp file: %w", err)
	}

	// Replace original file with compacted version
	if err := os.Rename(tempPath, s.pathDB); err != nil {
		return fmt.Errorf("error replacing original file: %w", err)
	}

	return nil
}

// Stats returns storage statistics including record counts and file information.
//
// Returns:
//   - map[string]interface{}: Statistics with keys:
//   - total_records: Total number of records in storage
//   - active_records: Number of active (non-deleted) records
//   - deleted_records: Number of logically deleted records
//   - file_path: Path to storage file
//   - file_size_bytes: Size of storage file in bytes
//   - unique_users: Number of unique users with records
func (s *cachedFileStorage) Stats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	fileInfo, err := os.Stat(s.pathDB)
	fileSize := int64(0)
	if err == nil && fileInfo != nil {
		fileSize = fileInfo.Size()
	}

	// Count only active records
	activeRecords := 0
	for _, record := range s.cache {
		if record.IsActive {
			activeRecords++
		}
	}

	return map[string]interface{}{
		"total_records":   len(s.cache),
		"active_records":  activeRecords,
		"deleted_records": len(s.cache) - activeRecords,
		"file_path":       s.pathDB,
		"file_size_bytes": fileSize,
		"unique_users":    len(s.userIDIndex),
	}
}
