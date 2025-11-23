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

// Create создает новую запись сокращенного URL в базе данных.
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

// CreateBatch создает пакет коротких URL в транзакции с обработкой пакетами фиксированного размера.
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
		end := min(start+effectiveBatchSize, len(requests))
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

// insertBatch выполняет вставку одного пакета записей
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
