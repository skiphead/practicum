package entity

import (
	"database/sql"
	"time"
)

type ShortenRequest struct {
	URL string `json:"url"`
}

type ShortenResponse struct {
	Result string `json:"result"`
}

type BatchShortenRequest struct {
	CorrelationID string `json:"correlation_id"`
	OriginalURL   string `json:"original_url"`
}

type BatchShortenResponse struct {
	CorrelationID string `json:"correlation_id"`
	ShortURL      string `json:"short_url"`
}

type ShortURL struct {
	ID            string        `db:"id" json:"id"`
	CorrelationID string        `db:"correlation_id" json:"correlation_id"`
	OriginalURL   string        `db:"original_url" json:"original_url"`
	ShortCode     string        `db:"short_code" json:"short_code"`
	CreatedAt     time.Time     `db:"created_at" json:"created_at"`
	ExpiresAt     sql.NullTime  `db:"expires_at" json:"expires_at,omitempty"`
	UserID        sql.NullInt64 `db:"user_id" json:"user_id,omitempty"`
	IsActive      bool          `db:"is_active" json:"is_active"`
	ClickCount    int64         `db:"click_count" json:"click_count"`
}
