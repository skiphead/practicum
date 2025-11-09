package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/skiphead/practicum/internal/domain/entity"
	"github.com/skiphead/practicum/pkg/utils"
	"go.uber.org/zap"
)

const (
	storageTableName = "shorts_url"
	defaultBatchSize = 100
)

var (
	ErrNotFound = errors.New("not found")
)

type URLRepository interface {
	Ping(ctx context.Context) error
	Create(ctx context.Context, shortCode, originalURL string) (*entity.ShortURL, error)
	CreateBatch(ctx context.Context, in []entity.BatchShortenRequest, batchSize int) ([]entity.ShortURL, error)
	Get(ctx context.Context, shortCode string) (*entity.ShortURL, error)
	Update(ctx context.Context) (*entity.ShortURL, error)
	Delete(ctx context.Context, id string) (string, error)
}

type storageRepository struct {
	table   string
	storage entity.ShortURL // Потенциальная проблема: состояние в репозитории
	db      *pgxpool.Pool
}

func NewStorageRepository(db *pgxpool.Pool) URLRepository {
	return &storageRepository{
		db:    db,
		table: storageTableName,
	}
}

func (r *storageRepository) Ping(ctx context.Context) error {
	if err := r.db.Ping(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}
	return nil
}

// Create создает новую запись сокращенного URL в базе данных.
func (r *storageRepository) Create(ctx context.Context, shortCode, originalURL string) (*entity.ShortURL, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer r.rollbackTxOnError(ctx, tx, &err)

	query := fmt.Sprintf(
		`INSERT INTO %s (
            short_code, 
            original_url, 
            expires_at
        ) VALUES ($1, $2, $3) 
        RETURNING 
            id, 
            original_url, 
            short_code, 
            created_at`,
		r.table,
	)

	var shortURL entity.ShortURL

	// Добавляем expiration time (например, 1 год)
	expiresAt := time.Now().AddDate(1, 0, 0)

	err = tx.QueryRow(ctx, query, shortCode, originalURL, expiresAt).Scan(
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
	ctx context.Context,
	in []entity.BatchShortenRequest,
	batchSize int,
) ([]entity.ShortURL, error) {
	if len(in) == 0 {
		return []entity.ShortURL{}, nil
	}

	// Используем размер по умолчанию, если не указан
	if batchSize <= 0 {
		batchSize = defaultBatchSize
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer r.rollbackTxOnError(ctx, tx, &err)

	response := make([]entity.ShortURL, 0, len(in))

	for start := 0; start < len(in); start += batchSize {
		end := start + batchSize
		if end > len(in) {
			end = len(in)
		}

		batchURLs, err := r.insertBatch(ctx, tx, in[start:end])
		if err != nil {
			return nil, fmt.Errorf("insert batch [%d:%d]: %w", start, end, err)
		}
		response = append(response, batchURLs...)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return response, nil
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

// insertBatch выполняет вставку одного пакета записей
func (r *storageRepository) insertBatch(
	ctx context.Context,
	tx pgx.Tx,
	batch []entity.BatchShortenRequest,
) ([]entity.ShortURL, error) {
	if len(batch) == 0 {
		return []entity.ShortURL{}, nil
	}

	placeholders := make([]string, 0, len(batch))
	args := make([]interface{}, 0, len(batch)*4) // 4 параметра на запись

	// Добавляем expiration time для всех записей
	expiresAt := time.Now().AddDate(1, 0, 0)

	for i, item := range batch {
		code := utils.GenerateRandomKey()
		placeholders = append(placeholders,
			fmt.Sprintf("($%d, $%d, $%d, $%d)", i*4+1, i*4+2, i*4+3, i*4+4))
		args = append(args, item.CorrelationID, code, item.OriginalURL, expiresAt)
	}

	query := fmt.Sprintf(
		`INSERT INTO %s (
            correlation_id, 
            short_code, 
            original_url, 
            expires_at
        ) VALUES %s 
        RETURNING id, correlation_id, original_url, short_code, created_at`,
		r.table,
		strings.Join(placeholders, ", "),
	)

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("execute batch query: %w", err)
	}
	defer rows.Close()

	var result []entity.ShortURL
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
		result = append(result, url)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	return result, nil
}

// Get возвращает запись сокращенного URL по его короткому коду.
func (r *storageRepository) Get(ctx context.Context, shortCode string) (*entity.ShortURL, error) {
	query := fmt.Sprintf(
		`SELECT 
            id, 
            original_url, 
            short_code, 
            created_at,
            expires_at
        FROM %s 
        WHERE short_code = $1`,
		r.table,
	)

	var shortURL entity.ShortURL
	var expiresAt time.Time

	err := r.db.QueryRow(ctx, query, shortCode).Scan(
		&shortURL.ID,
		&shortURL.OriginalURL,
		&shortURL.ShortCode,
		&shortURL.CreatedAt,
		&expiresAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("short URL with code '%s' not found: %w", shortCode, ErrNotFound)
		}
		return nil, fmt.Errorf("get short URL by code '%s': %w", shortCode, err)
	}

	// Проверяем срок действия
	if time.Now().After(expiresAt) {
		return nil, fmt.Errorf("short URL with code '%s' has expired", shortCode)
	}

	return &shortURL, nil
}

// Update обновляет существующую запись сокращенного URL в базе данных.
func (r *storageRepository) Update(ctx context.Context) (*entity.ShortURL, error) {
	query := fmt.Sprintf(
		`UPDATE %s 
         SET original_url = $1, short_code = $2 
         WHERE id = $3 
         RETURNING id, original_url, short_code, created_at`,
		r.table,
	)

	var shortURL entity.ShortURL
	err := r.db.QueryRow(
		ctx,
		query,
		r.storage.OriginalURL,
		r.storage.ShortCode,
		r.storage.ID,
	).Scan(
		&shortURL.ID,
		&shortURL.OriginalURL,
		&shortURL.ShortCode,
		&shortURL.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("short URL with id '%s' not found: %w", r.storage.ID, ErrNotFound)
		}
		return nil, fmt.Errorf("update short URL: %w", err)
	}

	return &shortURL, nil
}

// Delete удаляет запись сокращенного URL по идентификатору.
func (r *storageRepository) Delete(ctx context.Context, id string) (string, error) {
	query := fmt.Sprintf(
		`DELETE FROM %s 
         WHERE id = $1 
         RETURNING id`,
		r.table,
	)

	var deletedID string
	err := r.db.QueryRow(ctx, query, id).Scan(&deletedID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("short URL with id '%s' not found: %w", id, ErrNotFound)
		}
		return "", fmt.Errorf("delete short URL: %w", err)
	}

	return deletedID, nil
}
