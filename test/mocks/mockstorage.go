package mocks

import (
	"context"

	"github.com/skiphead/practicum/internal/domain/entity"
)

type MockStorage struct {
	SaveFunc    func(ctx context.Context, url entity.ShortURL) error
	GetByIDFunc func(ctx context.Context, id string) (*entity.ShortURL, error)
	CloseFunc   func() error
}

func (m *MockStorage) Save(ctx context.Context, url entity.ShortURL) error {
	if m.SaveFunc != nil {
		return m.SaveFunc(ctx, url)
	}
	return nil
}

func (m *MockStorage) GetByID(ctx context.Context, id string) (*entity.ShortURL, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, id)
	}
	return &entity.ShortURL{}, nil
}

func (m *MockStorage) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}
