package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/skiphead/practicum/internal/domain/entity"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// storageRepository implements the URLRepository interface for PostgreSQL storage.
// It provides methods for CRUD operations on shortened URLs with support for
// batch operations, user isolation, and URL expiration.
type storageRepository struct {
	table               string        // Database table name for URLs
	createQuery         string        // SQL query for creating a single URL
	createBatchQuery    string        // SQL query for batch URL creation
	getQuery            string        // SQL query for retrieving URL by short code
	getByOriginalURL    string        // SQL query for retrieving URL by original URL
	getByUserID         string        // SQL query for retrieving URLs by user ID
	updateQuery         string        // SQL query for updating URL
	updateIsActive      string        // SQL query for updating URL active status
	deleteQuery         string        // SQL query for deleting URL
	findDuplicateURL    string        // SQL query for finding duplicate URLs (deprecated)
	findDuplicatesQuery string        // SQL query for finding multiple duplicate URLs
	db                  *pgxpool.Pool // PostgreSQL connection pool
}

// NewStorageRepository creates a new PostgreSQL repository instance.
// It initializes the repository with default queries and applies any provided options.
//
// Parameters:
//   - db: PostgreSQL connection pool
//   - opts: Optional configuration functions for customizing the repository
//
// Returns:
//   - URLRepository: Initialized repository instance
func NewStorageRepository(db *pgxpool.Pool, opts ...Option) URLRepository {
	repo := &storageRepository{
		db:    db,
		table: storageTableName,
	}

	for _, opt := range opts {
		opt(repo)
	}

	repo.initQueries()
	return repo
}

// initQueries initializes all SQL queries used by the repository.
// This method sets up parameterized queries for all CRUD operations.
// Queries are built using the table name to ensure proper schema isolation.
func (r *storageRepository) initQueries() {
	r.findDuplicatesQuery = fmt.Sprintf(
		`SELECT 
			id,
		    created_at,
		    expires_at,
		    correlation_id,
		    short_code,
		    original_url,
		    user_id,
		    is_active,
		    click_count
		FROM %s WHERE original_url IN `,
		r.table,
	)

	r.createQuery = fmt.Sprintf(
		`INSERT INTO %s (
            user_id,    
			short_code, 
			original_url, 
			expires_at
		) VALUES ($1, $2, $3, $4) 
		RETURNING 
			id, 
			original_url, 
			short_code, 
			created_at`,
		r.table,
	)

	r.createBatchQuery = `INSERT INTO %s (
            user_id,    
			correlation_id, 
			short_code, 
			original_url, 
			expires_at
		) VALUES %s 
		RETURNING id, correlation_id, original_url, short_code, created_at`

	r.getQuery = fmt.Sprintf(
		`SELECT 
			id, 
			original_url, 
			short_code, 
			created_at,
			is_active,
			expires_at
		FROM %s 
		WHERE short_code = $1`,
		r.table,
	)

	r.getByOriginalURL = fmt.Sprintf(`SELECT 
			id, 
			original_url, 
			short_code, 
			created_at,
			expires_at
		FROM %s 
		WHERE original_url = $1`,
		r.table)

	r.getByUserID = fmt.Sprintf(`SELECT 
			id, 
			original_url, 
			short_code, 
			created_at,
			is_active,
			expires_at
		FROM %s 
		WHERE user_id = $1`,
		r.table)

	r.updateQuery = fmt.Sprintf(
		`UPDATE %s 
		SET original_url = $1, short_code = $2 
		WHERE id = $3 
		RETURNING id, original_url, short_code, created_at`,
		r.table,
	)

	r.updateIsActive = fmt.Sprintf(`
		UPDATE %s SET is_active = $3 WHERE short_code = $2 AND user_id = $1
	`, r.table)

	r.deleteQuery = fmt.Sprintf(
		`DELETE FROM %s 
		WHERE id = $1 
		RETURNING id`,
		r.table,
	)
}

// Ping checks the database connection health.
// It performs a simple ping operation to verify the database is accessible.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//
// Returns:
//   - error: Connection error if ping fails, nil otherwise
func (r *storageRepository) Ping(ctx context.Context) error {
	if err := r.db.Ping(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}
	return nil
}

// rollbackTxOnError safely rolls back a transaction if an error occurred.
// It logs rollback errors but doesn't mask the original error.
// This is a helper function to ensure proper transaction cleanup.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - tx: PostgreSQL transaction to rollback
//   - err: Pointer to the error that triggered the rollback
func (r *storageRepository) rollbackTxOnError(ctx context.Context, tx pgx.Tx, err *error) {
	if *err != nil {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			zap.L().Error("transaction rollback failed",
				zap.Error(*err),
				zap.NamedError("rollback_error", rollbackErr),
			)
		}
	}
}

// getEffectiveBatchSize determines the appropriate batch size for operations.
// If the requested size is invalid (≤0), it returns the default batch size.
//
// Parameters:
//   - requestedSize: User-requested batch size
//
// Returns:
//   - int: Effective batch size to use
func (r *storageRepository) getEffectiveBatchSize(requestedSize int) int {
	if requestedSize <= 0 {
		return defaultBatchSize
	}
	return requestedSize
}

// calculateExpiryTime calculates the default expiration time for URLs.
// By default, URLs expire after a predefined number of years.
//
// Returns:
//   - time.Time: Calculated expiry time
func (r *storageRepository) calculateExpiryTime() time.Time {
	return time.Now().AddDate(defaultExpiryYears, 0, 0)
}

// handleQueryError processes query errors and returns standardized error messages.
// It converts database-level errors (like "no rows") to application-level errors.
//
// Parameters:
//   - err: Original database error
//   - entityDesc: Description of the entity being queried (e.g., "URL")
//   - identifier: Identifier used in the query (e.g., short code)
//
// Returns:
//   - *entity.ShortURL: Always nil
//   - error: Formatted application error
func (r *storageRepository) handleQueryError(err error, entityDesc string, identifier string) (*entity.ShortURL, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("%s '%s': %w", entityDesc, identifier, ErrNotFound)
	}
	return nil, fmt.Errorf("query %s: %w", entityDesc, err)
}

// validateExpiry checks if a URL has expired based on its expiry time.
// Returns an error if the URL has expired, nil otherwise.
//
// Parameters:
//   - expiresAt: Expiration time to validate
//   - identifier: Identifier for error reporting (e.g., short code)
//
// Returns:
//   - error: Expiry error if URL has expired, nil otherwise
func (r *storageRepository) validateExpiry(expiresAt time.Time, identifier string) error {
	if time.Now().After(expiresAt) {
		return fmt.Errorf("short URL with code '%s' has expired", identifier)
	}
	return nil
}
