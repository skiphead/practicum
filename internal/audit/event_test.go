// audit/event_test.go
package audit

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEvent_Validation(t *testing.T) {
	tests := []struct {
		name    string
		event   Event
		wantErr bool
	}{
		{
			name: "Валидное событие shorten",
			event: Event{
				Timestamp: time.Now().Unix(),
				Action:    "shorten",
				UserID:    "user123",
				URL:       "https://example.com",
			},
			wantErr: false,
		},
		{
			name: "Валидное событие follow",
			event: Event{
				Timestamp: time.Now().Unix(),
				Action:    "follow",
				UserID:    "user456",
				URL:       "https://short.url/abc",
			},
			wantErr: false,
		},
		{
			name: "Невалидное действие",
			event: Event{
				Timestamp: time.Now().Unix(),
				Action:    "invalid_action",
				UserID:    "user123",
				URL:       "https://example.com",
			},
			wantErr: true,
		},
		{
			name: "Пустой URL",
			event: Event{
				Timestamp: time.Now().Unix(),
				Action:    "shorten",
				UserID:    "user123",
				URL:       "",
			},
			wantErr: true,
		},
		{
			name: "Некорректный timestamp",
			event: Event{
				Timestamp: -1,
				Action:    "shorten",
				UserID:    "user123",
				URL:       "https://example.com",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.event.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Validate проверяет валидность события
func (e *Event) Validate() error {
	// Простая валидация для примера
	if e.Timestamp <= 0 {
		return errors.New("invalid timestamp")
	}

	if e.Action != "shorten" && e.Action != "follow" {
		return errors.New("invalid action")
	}

	if e.URL == "" {
		return errors.New("URL cannot be empty")
	}

	return nil
}
