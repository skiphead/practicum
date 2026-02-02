package repository

// Package constants defining default configuration values for the storage repository.
const (
	// storageTableName is the default table name for storing shortened URLs.
	storageTableName = "shorts_url"

	// defaultBatchSize is the default number of records to process in batch operations.
	// This helps optimize database performance for bulk operations.
	defaultBatchSize = 100

	// defaultExpiryYears is the default number of years before a shortened URL expires.
	// URLs created without explicit expiry will use this default value.
	defaultExpiryYears = 1
)

// Config holds configuration options for the storage repository.
// This struct allows for customization of repository behavior without changing the implementation.
type Config struct {
	// TableName specifies the database table name for storing URLs.
	// If empty, the default storageTableName will be used.
	TableName string
}

// Option is a function type that configures a storageRepository.
// This functional option pattern provides a clean way to customize repository behavior.
// Options can be passed to NewStorageRepository to modify its configuration.
type Option func(*storageRepository)
