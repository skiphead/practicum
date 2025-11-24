package repository

import (
	"context"
	"errors"
	"fmt"
	"github.com/skiphead/practicum/internal/domain/entity"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type storageRepository struct {
	table               string
	createQuery         string
	createBatchQuery    string
	getQuery            string
	getByOriginalURL    string
	getByUserID         string
	updateQuery         string
	updateIsActive      string
	deleteQuery         string
	findDuplicateURL    string
	findDuplicatesQuery string
	db                  *pgxpool.Pool
}

func NewStorageRepository(db *pgxpool.Pool, opts ...RepositoryOption) URLRepository {
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

func (r *storageRepository) Ping(ctx context.Context) error {
	if err := r.db.Ping(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}
	return nil
}

// rollbackTxOnError откатывает транзакцию если переданный error не nil
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

// getEffectiveBatchSize determines the batch size to use
func (r *storageRepository) getEffectiveBatchSize(requestedSize int) int {
	if requestedSize <= 0 {
		return defaultBatchSize
	}
	return requestedSize
}

func (r *storageRepository) calculateExpiryTime() time.Time {
	return time.Now().AddDate(defaultExpiryYears, 0, 0)
}

func (r *storageRepository) handleQueryError(err error, entityDesc string, identifier string) (*entity.ShortURL, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("%s '%s': %w", entityDesc, identifier, ErrNotFound)
	}
	return nil, fmt.Errorf("query %s: %w", entityDesc, err)
}

func (r *storageRepository) validateExpiry(expiresAt time.Time, identifier string) error {
	if time.Now().After(expiresAt) {
		return fmt.Errorf("short URL with code '%s' has expired", identifier)
	}
	return nil
}
