package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/skiphead/practicum/internal/domain/entity"
	"github.com/skiphead/practicum/pkg/utils"
)

// Create creates a new shortened URL record in the database.
// It generates a unique short code and stores the URL with user ownership and expiration.
// The operation is performed within a transaction for data consistency.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - userID: ID of the user creating the URL
//   - shortCode: Unique identifier for the shortened URL
//   - originalURL: The original URL to be shortened
//
// Returns:
//   - *entity.ShortURL: Created URL entity with database-generated fields
//   - error: Database or transaction error if creation fails
//
// The URL expires automatically after 1 year from creation.
func (r *storageRepository) Create(ctx context.Context, userID, shortCode, originalURL string) (*entity.ShortURL, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer r.rollbackTxOnError(ctx, tx, &err)

	var shortURL entity.ShortURL
	expiresAt := time.Now().AddDate(1, 0, 0)

	err = tx.QueryRow(ctx, r.createQuery, userID, shortCode, originalURL, expiresAt).Scan(
		&shortURL.ID,
		&shortURL.OriginalURL,
		&shortURL.ShortCode,
		&shortURL.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert and scan record: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return &shortURL, nil
}

// CreateBatch creates multiple shortened URLs in a single operation with batch processing.
// It processes requests in configurable batch sizes to optimize database performance.
// Each batch is processed in a single transaction for atomicity.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - userID: ID of the user creating the URLs
//   - requests: Slice of batch request items with correlation IDs
//   - batchSize: Maximum number of records to insert per database batch (uses default if ≤0)
//
// Returns:
//   - []entity.ShortURL: Slice of created URL entities
//   - error: Database or transaction error if batch creation fails
//
// Each URL in the batch gets a unique short code and the same expiration date.
func (r *storageRepository) CreateBatch(
	ctx context.Context, userID string,
	requests []entity.BatchShortenRequest,
	batchSize int,
) ([]entity.ShortURL, error) {
	if len(requests) == 0 {
		return []entity.ShortURL{}, nil
	}

	effectiveBatchSize := r.getEffectiveBatchSize(batchSize)

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer r.rollbackTxOnError(ctx, tx, &err)

	var allResults []entity.ShortURL

	for start := 0; start < len(requests); start += effectiveBatchSize {
		end := utils.Min(start+effectiveBatchSize, len(requests))
		batch := requests[start:end]

		batchResults, err := r.insertBatch(ctx, tx, userID, batch)
		if err != nil {
			return nil, fmt.Errorf("insert batch [%d:%d]: %w", start, end, err)
		}
		allResults = append(allResults, batchResults...)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return allResults, nil
}

// insertBatch executes the insertion of a single batch of URL records.
// It builds a parameterized SQL query with placeholders for batch insertion.
// This method is called by CreateBatch for each chunk of requests.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - tx: Database transaction for batch insertion
//   - userID: ID of the user creating the URLs
//   - batch: Slice of batch request items to insert
//
// Returns:
//   - []entity.ShortURL: Created URL entities from this batch
//   - error: Database error if insertion fails
//
// Each URL gets a generated short code and the default expiration time.
func (r *storageRepository) insertBatch(
	ctx context.Context,
	tx pgx.Tx, userID string,
	batch []entity.BatchShortenRequest,
) ([]entity.ShortURL, error) {
	if len(batch) == 0 {
		return []entity.ShortURL{}, nil
	}

	placeholders := make([]string, 0, len(batch))
	args := make([]interface{}, 0, len(batch)*5)

	expiresAt := r.calculateExpiryTime()

	for i, item := range batch {
		code := utils.GenerateRandomKey()
		pos := i * 5
		placeholders = append(placeholders,
			fmt.Sprintf("($%d, $%d, $%d, $%d, $%d)", pos+1, pos+2, pos+3, pos+4, pos+5))
		args = append(args, userID, item.CorrelationID, code, item.OriginalURL, expiresAt)
	}

	query := fmt.Sprintf(r.createBatchQuery, r.table, strings.Join(placeholders, ", "))

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("execute batch query: %w", err)
	}
	defer rows.Close()

	return r.scanBatchResults(rows)
}

// scanBatchResults scans database rows into ShortURL entities.
// It processes the result set from a batch insertion query.
//
// Parameters:
//   - rows: Database rows from a batch insertion query
//
// Returns:
//   - []entity.ShortURL: Scanned URL entities
//   - error: Scanning or iteration error if any
func (r *storageRepository) scanBatchResults(rows pgx.Rows) ([]entity.ShortURL, error) {
	var results []entity.ShortURL

	for rows.Next() {
		var url entity.ShortURL
		if err := rows.Scan(
			&url.ID,
			&url.CorrelationID,
			&url.OriginalURL,
			&url.ShortCode,
			&url.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		results = append(results, url)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	return results, nil
}
