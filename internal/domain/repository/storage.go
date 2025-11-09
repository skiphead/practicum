package repository

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/skiphead/practicum/internal/domain/entity"
)

const storageTableName = "shorts_url"

type StorageRepository interface {
	Ping(ctx context.Context) error
	Create(ctx context.Context, shortCode, originalURL string) (*entity.ShortURL, error)
	Get(ctx context.Context, shortCode string) (*entity.ShortURL, error)
}

type storageRepository struct {
	table   string
	storage entity.ShortURL
	db      *pgxpool.Pool
}

func NewStorageRepository(db *pgxpool.Pool) StorageRepository {
	return &storageRepository{db: db, table: storageTableName}
}

func (r *storageRepository) Ping(ctx context.Context) error {
	if err := r.db.Ping(ctx); err != nil {
		return err
	}

	return nil
}

func (r *storageRepository) Create(ctx context.Context, shortCode, originalURL string) (*entity.ShortURL, error) {

	const sql = `INSERT INTO %s (short_code, original_url, expires_at) VALUES ($1, $2, NOW()) RETURNING 
               id, 
               original_url, 
               short_code, 
               created_at`
	query := fmt.Sprintf(sql, r.table)

	var s entity.ShortURL

	err := r.db.QueryRow(ctx, query, shortCode, originalURL).
		Scan(&s.ID,
			&s.OriginalURL,
			&s.ShortCode,
			&s.CreatedAt)
	if err != nil {
		return nil, err
	}

	return &s, nil
}

func (r *storageRepository) Get(ctx context.Context, shortCode string) (*entity.ShortURL, error) {
	const sql = `SELECT id, 
       original_url, 
       short_code, 
       created_at 
	FROM %s WHERE short_code = $1`

	query := fmt.Sprintf(sql, r.table)
	var s entity.ShortURL
	err := r.db.QueryRow(ctx, query, shortCode).
		Scan(&s.ID,
			&s.OriginalURL,
			&s.ShortCode,
			&s.CreatedAt)
	if err != nil {
		return nil, err
	}

	return &s, nil
}

func (r *storageRepository) Update(ctx context.Context) (*entity.ShortURL, error) {
	const sql = `UPDATE %s SET original_url = $1, short_code = $2 WHERE id = $3 RETURNING id, original_url, short_code, created_at`
	query := fmt.Sprintf(sql, r.table)
	var s entity.ShortURL
	err := r.db.QueryRow(ctx, query, r.storage.OriginalURL, r.storage.ShortCode, r.storage.ID).
		Scan(&s.ID,
			&s.OriginalURL,
			&s.ShortCode,
			&s.CreatedAt)
	if err != nil {
		return nil, err
	}

	return &s, nil
}

func (r *storageRepository) Delete(ctx context.Context, id string) (string, error) {
	const sql = `DELETE FROM %s WHERE id = $1`
	query := fmt.Sprintf(sql, r.table)
	var respID string
	err := r.db.QueryRow(ctx, query, id).Scan(&respID)
	if err != nil {
		return "", err
	}

	return respID, nil
}
