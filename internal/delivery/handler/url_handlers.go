// Package handler provides HTTP handlers for URL shortening service.
// It handles creation of short URLs via both JSON API and plain text endpoints,
// as well as redirection to original URLs.
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/skiphead/practicum/internal/audit"
	"github.com/skiphead/practicum/internal/domain/entity"
	"go.uber.org/zap"
)

// createShortAPIURL handles requests to create short URLs via JSON API.
//
// Endpoint: POST /api/shorten
// Content-Type: application/json
//
// The handler expects a JSON request body with the following structure:
//
//	{"url": "https://example.com"}
//
// Returns:
//   - 201 Created with JSON response {"result": "short-url"} on success
//   - 409 Conflict if the URL already exists (same response body)
//   - 400 Bad Request for invalid input
//   - 415 Unsupported Media Type for incorrect Content-Type
//   - 500 Internal Server Error for audit logging failures
//
// The handler also logs audit events for URL shortening operations.
func (h *URLHandler) createShortAPIURL(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return
	}

	body, err := h.readRequestBody(r)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var original entity.ShortenRequest
	if err := json.Unmarshal(body, &original); err != nil {
		zap.L().Error("unmarshal error", zap.Error(err))
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	shortURL, isConflict, err := h.processAndSaveURL(original.URL, w, r)
	if err != nil {
		return
	}

	if isConflict {
		render.Status(r, http.StatusConflict)
		render.JSON(w, r, map[string]string{"result": shortURL})
		return
	}

	errAuditClient := h.auditClient.LogEvent(context.Background(), &audit.Event{
		Timestamp: time.Now().Unix(),
		Action:    "shorten",
		UserID:    "",
		URL:       original.URL,
	})
	if errAuditClient != nil {
		render.Status(r, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	render.JSON(w, r, map[string]string{"result": shortURL})
}

// createShortURL handles requests to create short URLs via plain text endpoint.
//
// Endpoint: POST /
// Content-Type: text/plain
//
// The handler expects a plain text request body containing the URL to shorten.
//
// Returns:
//   - 201 Created with plain text short URL on success
//   - 409 Conflict if the URL already exists (same response body)
//   - 400 Bad Request for invalid input
//   - 500 Internal Server Error for audit logging failures
//
// The handler also logs audit events for URL shortening operations.
func (h *URLHandler) createShortURL(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	body, err := h.readRequestBody(r)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	originalURL := string(body)
	shortURL, isConflict, err := h.processAndSaveURL(originalURL, w, r)
	if err != nil {
		return
	}

	if isConflict {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusConflict)
		_, err = w.Write([]byte(shortURL))
		if err != nil {
			zap.L().Error("write error", zap.Error(err))
		}
		return
	}

	errAuditClient := h.auditClient.LogEvent(context.Background(), &audit.Event{
		Timestamp: time.Now().Unix(),
		Action:    "shorten",
		UserID:    "",
		URL:       originalURL,
	})
	if errAuditClient != nil {
		render.Status(r, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	_, err = w.Write([]byte(shortURL))
	if err != nil {
		zap.L().Error("write error", zap.Error(err))
		return
	}
}

// redirectURL handles requests to redirect to original URLs using short URL keys.
//
// Endpoint: GET /{key}
//
// The handler extracts the short URL key from the request path and redirects
// to the original URL if it exists and is active.
//
// Returns:
//   - 307 Temporary Redirect on successful redirection
//   - 404 Not Found if the short URL doesn't exist
//   - 410 Gone if the short URL exists but is marked as inactive
//   - 400 Bad Request for root path requests
//   - 500 Internal Server Error for audit logging failures
//
// The handler also logs audit events for URL follow operations with user tracking.
func (h *URLHandler) redirectURL(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	key := r.URL.Path[1:]
	data, err := h.storage.Get(ctx, key)

	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if !data.IsActive {
		w.WriteHeader(http.StatusGone)
		return
	}

	errAuditClient := h.auditClient.LogEvent(context.Background(), &audit.Event{
		Timestamp: time.Now().Unix(),
		Action:    "follow",
		UserID:    data.UserID,
		URL:       data.OriginalURL,
	})
	if errAuditClient != nil {
		render.Status(r, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Location", data.OriginalURL)
	w.WriteHeader(http.StatusTemporaryRedirect)
}
