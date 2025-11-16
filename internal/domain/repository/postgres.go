package repository

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgconn"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/skiphead/practicum/internal/domain/entity"
	"github.com/skiphead/practicum/pkg/utils"
	"go.uber.org/zap"
)

const (
	storageTableName   = "shorts_url"
	defaultBatchSize   = 100
	defaultExpiryYears = 1
)

var (
	ErrNotFound = errors.New("not found")
)

type URLRepository interface {
	Ping(ctx context.Context) error
	IsDuplicateError(err error) bool
	Create(ctx context.Context, shortCode, originalURL string) (*entity.ShortURL, error)
	CreateBatch(ctx context.Context, in []entity.BatchShortenRequest, batchSize int) ([]entity.ShortURL, error)
	Get(ctx context.Context, shortCode string) (*entity.ShortURL, error)
	GetByOriginalURL(ctx context.Context, originalURL string) (*entity.ShortURL, error)
	Update(ctx context.Context, shortURL *entity.ShortURL) (*entity.ShortURL, error) // Исправлена сигнатура
	Delete(ctx context.Context, id string) (string, error)
}

type storageRepository struct {
	table            string
	createQuery      string
	createBatchQuery string
	getQuery         string
	getByOriginalURL string
	updateQuery      string
	deleteQuery      string
	db               *pgxpool.Pool
}

type RepositoryConfig struct {
	TableName string
}

type RepositoryOption func(*storageRepository)

func WithTableName(tableName string) RepositoryOption {
	return func(r *storageRepository) {
		r.table = tableName
	}
}

func NewStorageRepository(db *pgxpool.Pool, opts ...RepositoryOption) URLRepository {
	repo := &storageRepository{
		db:    db,
		table: storageTableName,
	}

	// Применяем опции
	for _, opt := range opts {
		opt(repo)
	}

	// Инициализируем SQL-запросы после установки имени таблицы
	repo.initQueries()

	return repo
}

func (r *storageRepository) initQueries() {
	r.createQuery = fmt.Sprintf(
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

	r.createBatchQuery = `INSERT INTO %s (
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

	r.updateQuery = fmt.Sprintf(
		`UPDATE %s 
		SET original_url = $1, short_code = $2 
		WHERE id = $3 
		RETURNING id, original_url, short_code, created_at`,
		r.table,
	)

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

func (r *storageRepository) IsDuplicateError(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		zap.L().Debug("Duplicate key violation detected",
			zap.String("constraint", pgErr.ConstraintName),
			zap.String("detail", pgErr.Detail))
		return true
	}
	return false
}

// Create создает новую запись сокращенного URL в базе данных.
func (r *storageRepository) Create(ctx context.Context, shortCode, originalURL string) (*entity.ShortURL, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer r.rollbackTxOnError(ctx, tx, &err)

	var shortURL entity.ShortURL
	expiresAt := time.Now().AddDate(1, 0, 0)

	err = tx.QueryRow(ctx, r.createQuery, shortCode, originalURL, expiresAt).Scan(
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

		batchResults, err := r.insertBatch(ctx, tx, batch)
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
	args := make([]interface{}, 0, len(batch)*4)

	expiresAt := r.calculateExpiryTime()

	for i, item := range batch {
		code := utils.GenerateRandomKey()
		pos := i * 4
		placeholders = append(placeholders,
			fmt.Sprintf("($%d, $%d, $%d, $%d)", pos+1, pos+2, pos+3, pos+4))
		args = append(args, item.CorrelationID, code, item.OriginalURL, expiresAt)
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

// Get возвращает запись сокращенного URL по его короткому коду.
func (r *storageRepository) Get(ctx context.Context, shortCode string) (*entity.ShortURL, error) {
	var shortURL entity.ShortURL
	var expiresAt time.Time

	err := r.db.QueryRow(ctx, r.getQuery, shortCode).Scan(
		&shortURL.ID,
		&shortURL.OriginalURL,
		&shortURL.ShortCode,
		&shortURL.CreatedAt,
		&expiresAt,
	)
	if err != nil {
		return r.handleQueryError(err, "short URL for original URL", shortCode)
	}

	// Проверяем срок действия
	if err := r.validateExpiry(expiresAt, shortCode); err != nil {
		return nil, err
	}

	return &shortURL, nil
}

// GetByOriginalURL возвращает запись сокращенного URL по его оригинальной ссылке.
func (r *storageRepository) GetByOriginalURL(ctx context.Context, originalURL string) (*entity.ShortURL, error) {
	var shortURL entity.ShortURL
	var expiresAt time.Time

	err := r.db.QueryRow(ctx, r.getByOriginalURL, originalURL).Scan(
		&shortURL.ID,
		&shortURL.OriginalURL,
		&shortURL.ShortCode,
		&shortURL.CreatedAt,
		&expiresAt,
	)
	if err != nil {
		return r.handleQueryError(err, "short URL for original URL", originalURL)
	}
	// Проверяем срок действия
	if err := r.validateExpiry(expiresAt, shortURL.ShortCode); err != nil {
		return nil, err
	}

	return &shortURL, nil
}

// Update обновляет существующую запись сокращенного URL в базе данных.
func (r *storageRepository) Update(ctx context.Context, shortURL *entity.ShortURL) (*entity.ShortURL, error) {
	var updatedURL entity.ShortURL

	err := r.db.QueryRow(
		ctx,
		r.updateQuery,
		shortURL.OriginalURL,
		shortURL.ShortCode,
		shortURL.ID,
	).Scan(
		&updatedURL.ID,
		&updatedURL.OriginalURL,
		&updatedURL.ShortCode,
		&updatedURL.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("short URL with id '%s' not found: %w", shortURL.ID, ErrNotFound)
		}
		return nil, fmt.Errorf("update short URL: %w", err)
	}

	return &updatedURL, nil
}

// Delete удаляет запись сокращенного URL по идентификатору.
func (r *storageRepository) Delete(ctx context.Context, id string) (string, error) {
	var deletedID string
	err := r.db.QueryRow(ctx, r.deleteQuery, id).Scan(&deletedID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("short URL with id '%s' not found: %w", id, ErrNotFound)
		}
		return "", fmt.Errorf("delete short URL: %w", err)
	}

	return deletedID, nil
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
