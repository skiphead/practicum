// Package entity defines core data structures and business objects for the URL shortening service.
// It includes request/response models, database entity, and validation logic
// for maintaining data integrity across the application layers.
package entity

import (
	"database/sql"
	"errors"
	"net/url"
	"time"
)

// Package-level error definitions for URL validation and entity constraints.
var (
	ErrEmptyURL         = errors.New("URL cannot be empty")
	ErrInvalidURL       = errors.New("invalid URL")
	ErrEmptyResult      = errors.New("result field cannot be empty")
	ErrEmptyCorrelation = errors.New("correlation_id cannot be empty")
	ErrEmptyShortCode   = errors.New("short_code cannot be empty")
	ErrInvalidExpiry    = errors.New("expires_at cannot be earlier than created_at")
)

// ShortenRequest represents a request to shorten a single URL.
// Used in JSON API endpoints for URL shortening.
type ShortenRequest struct {
	URL string `json:"url"`
}

// ShortenResponse represents the response for a shortened URL.
// Contains the full shortened URL in the Result field.
type ShortenResponse struct {
	Result string `json:"result"`
}

// ListByUserIDResponse represents a URL entry in user's URL list.
// Contains both the shortened and original URLs for display purposes.
type ListByUserIDResponse struct {
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

// BatchShortenRequest represents a request item for batch URL shortening.
// CorrelationID is used to match requests with responses in batch operations.
type BatchShortenRequest struct {
	CorrelationID string `json:"correlation_id"`
	OriginalURL   string `json:"original_url"`
}

// BatchShortenResponse represents a response item for batch URL shortening.
// Contains the correlation ID from the request and the resulting shortened URL.
type BatchShortenResponse struct {
	CorrelationID string `json:"correlation_id"`
	ShortURL      string `json:"short_url"`
}

// ShortURL represents a complete URL entity in the database.
// Contains all metadata about a shortened URL including usage statistics.
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

// Validate performs validation checks on a ShortenRequest.
// Ensures the URL is not empty and has a valid format.
//
// Returns:
//   - ErrEmptyURL if URL is empty
//   - ErrInvalidURL if URL format is invalid
//   - nil if validation passes
func (r *ShortenRequest) Validate() error {
	if r.URL == "" {
		return ErrEmptyURL
	}
	if _, err := url.ParseRequestURI(r.URL); err != nil {
		return ErrInvalidURL
	}
	return nil
}

// Validate performs validation checks on a ShortenResponse.
// Ensures the Result field is not empty.
//
// Returns:
//   - ErrEmptyResult if Result is empty
//   - nil if validation passes
func (r *ShortenResponse) Validate() error {
	if r.Result == "" {
		return ErrEmptyResult
	}
	return nil
}

// Validate performs validation checks on a BatchShortenRequest.
// Ensures both CorrelationID and OriginalURL are present and valid.
//
// Returns:
//   - ErrEmptyCorrelation if CorrelationID is empty
//   - ErrEmptyURL if OriginalURL is empty
//   - ErrInvalidURL if OriginalURL format is invalid
//   - nil if validation passes
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

// Validate performs validation checks on a BatchShortenResponse.
// Ensures both CorrelationID and ShortURL are present.
//
// Returns:
//   - ErrEmptyCorrelation if CorrelationID is empty
//   - ErrEmptyResult if ShortURL is empty
//   - nil if validation passes
func (r *BatchShortenResponse) Validate() error {
	if r.CorrelationID == "" {
		return ErrEmptyCorrelation
	}
	if r.ShortURL == "" {
		return ErrEmptyResult
	}
	return nil
}

// Validate performs comprehensive validation checks on a ShortURL entity.
// Validates URL format, required fields, and logical constraints.
//
// Returns:
//   - ErrEmptyURL if OriginalURL is empty
//   - ErrInvalidURL if OriginalURL format is invalid
//   - ErrEmptyShortCode if ShortCode is empty
//   - ErrInvalidExpiry if ExpiresAt is earlier than CreatedAt
//   - nil if all validations pass
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
