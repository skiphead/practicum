package mocks

import (
	"context"
	"time"

	"github.com/skiphead/practicum/internal/audit"
	"github.com/skiphead/practicum/internal/domain/entity"
)

type MockStorage struct {
	SaveFunc func(ctx context.Context, url, userID string) (string, bool, error)
	GetFunc  func(ctx context.Context, key string) (*entity.ShortURL, error)
}

func (m *MockStorage) Save(ctx context.Context, url, userID string) (string, bool, error) {
	if m.SaveFunc != nil {
		return m.SaveFunc(ctx, url, userID)
	}
	return "abc12345", false, nil
}

func (m *MockStorage) Get(ctx context.Context, key string) (*entity.ShortURL, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, key)
	}
	return &entity.ShortURL{
		OriginalURL: "https://example.com",
		ShortCode:   "http://localhost:8080/abc12345",
		UserID:      "user123",
		IsActive:    true,
		CreatedAt:   time.Now(),
	}, nil
}

type MockAuditClient struct {
	LogEventFunc func(ctx context.Context, event *audit.Event) error
}

func (m *MockAuditClient) LogEvent(ctx context.Context, event *audit.Event) error {
	if m.LogEventFunc != nil {
		return m.LogEventFunc(ctx, event)
	}
	return nil
}
