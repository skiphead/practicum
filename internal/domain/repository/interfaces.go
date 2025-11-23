package repository

import (
	"context"
	"errors"

	"github.com/skiphead/practicum/internal/domain/entity"
)

var (
	ErrNotFound = errors.New("not found")
)

type URLRepository interface {
	Ping(ctx context.Context) error
	IsDuplicateError(err error) bool
	Create(ctx context.Context, userID, shortCode, originalURL string) (*entity.ShortURL, error)
	CreateBatch(ctx context.Context, userID string, in []entity.BatchShortenRequest, batchSize int) ([]entity.ShortURL, error)
	Get(ctx context.Context, shortCode string) (*entity.ShortURL, error)
	GetByOriginalURL(ctx context.Context, originalURL string) (*entity.ShortURL, error)
	GetByUserID(ctx context.Context, userID string) ([]entity.ShortURL, error)
	Update(ctx context.Context, shortURL *entity.ShortURL) (*entity.ShortURL, error)
	Delete(ctx context.Context, id string) (string, error)
	FindDuplicateURLs(ctx context.Context, urls []entity.BatchShortenRequest) ([]entity.ShortURL, error)
	UpdateIsActive(ctx context.Context, shortCodes []string, userID string, isActive bool, batchSize int) ([]string, error)
}
