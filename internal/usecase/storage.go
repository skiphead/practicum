package usecase

import (
	"context"
	"fmt"
	"github.com/skiphead/practicum/internal/domain/entity"
	"github.com/skiphead/practicum/internal/domain/repository"
	"github.com/skiphead/practicum/pkg/storage"
)

type Storage struct {
	FileStorage storage.Storage
	StorageRepo repository.StorageRepository
}

func NewStorage(fs storage.Storage, repo repository.StorageRepository) *Storage {

	return &Storage{
		FileStorage: fs,
		StorageRepo: repo,
	}
}

func (s Storage) Ping(ctx context.Context) error {
	return s.StorageRepo.Ping(ctx)
}

func (s Storage) Save(ctx context.Context, key, originalURL string) (*entity.ShortURL, error) {
	if s.StorageRepo.Ping(ctx) != nil {
		err := s.FileStorage.Save(key, originalURL)
		if err != nil {
			return nil, err
		}
		return &entity.ShortURL{
			OriginalURL: originalURL,
			ShortCode:   key,
		}, nil
	}

	resp, err := s.StorageRepo.Create(ctx, key, originalURL)
	if err != nil {
		return nil, err
	}

	return resp, nil

}

func (s Storage) Get(ctx context.Context, key string) (*entity.ShortURL, error) {
	if s.StorageRepo.Ping(ctx) != nil {
		resp, exists, err := s.FileStorage.Get(key)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, fmt.Errorf("short URL not found")
		}
		return &entity.ShortURL{
			ID:          resp.UUID,
			OriginalURL: resp.OriginalURL,
		}, nil
	}

	resp, err := s.StorageRepo.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	return resp, nil

}
