package entity

import (
	"database/sql"
	"errors"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"net/url"
	"time"
)

var (
	ErrEmptyURL         = errors.New("URL не может быть пустым")
	ErrInvalidURL       = errors.New("некорректный URL")
	ErrEmptyResult      = errors.New("поле result не может быть пустым")
	ErrEmptyCorrelation = errors.New("correlation_id не может быть пустым")
	ErrEmptyShortCode   = errors.New("short_code не может быть пустым")
	ErrInvalidExpiry    = errors.New("expires_at не может быть раньше created_at")
)

type Claims struct {
	jwt.RegisteredClaims
	UserID uuid.UUID
}

type ShortenRequest struct {
	URL string `json:"url"`
}

type ShortenResponse struct {
	Result string `json:"result"`
}

type ListByUserIDResponse struct {
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
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
	ID            string       `db:"id" json:"id"`
	CorrelationID string       `db:"correlation_id" json:"correlation_id"`
	OriginalURL   string       `db:"original_url" json:"original_url"`
	ShortCode     string       `db:"short_code" json:"short_code"`
	CreatedAt     time.Time    `db:"created_at" json:"created_at"`
	ExpiresAt     sql.NullTime `db:"expires_at" json:"expires_at,omitempty"`
	UserID        string       `db:"user_id" json:"user_id,omitempty"`
	IsActive      bool         `db:"is_active" json:"is_active"`
	ClickCount    int64        `db:"click_count" json:"click_count"`
}

func (r *ShortenRequest) Validate() error {
	if r.URL == "" {
		return ErrEmptyURL
	}
	if _, err := url.ParseRequestURI(r.URL); err != nil {
		return ErrInvalidURL
	}
	return nil
}

func (r *ShortenResponse) Validate() error {
	if r.Result == "" {
		return ErrEmptyResult
	}
	return nil
}

func (r *BatchShortenRequest) Validate() error {
	if r.CorrelationID == "" {
		return ErrEmptyCorrelation
	}
	if r.OriginalURL == "" {
		return ErrEmptyURL
	}
	if _, err := url.ParseRequestURI(r.OriginalURL); err != nil {
		return ErrInvalidURL
	}
	return nil
}

func (r *BatchShortenResponse) Validate() error {
	if r.CorrelationID == "" {
		return ErrEmptyCorrelation
	}
	if r.ShortURL == "" {
		return ErrEmptyResult
	}
	return nil
}

func (s *ShortURL) Validate() error {
	if s.OriginalURL == "" {
		return ErrEmptyURL
	}
	if _, err := url.ParseRequestURI(s.OriginalURL); err != nil {
		return ErrInvalidURL
	}
	if s.ShortCode == "" {
		return ErrEmptyShortCode
	}
	if s.ExpiresAt.Valid && s.ExpiresAt.Time.Before(s.CreatedAt) {
		return ErrInvalidExpiry
	}
	return nil
}
