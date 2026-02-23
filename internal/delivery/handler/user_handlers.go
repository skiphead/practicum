// Package handler provides HTTP handler for URL shortening service.
// It handles creation of short URLs via both JSON API and plain text endpoints,
// retrieval of user's URLs, batch deletion, and redirection to original URLs.
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/skiphead/practicum/internal/domain/entity"
)

// getAPIUserUrls retrieves all URLs created by the authenticated user.
//
// Endpoint: GET /api/user/urls
// Content-Type: application/json
// Authentication: Requires valid user authentication via context
//
// Returns:
//   - 200 OK with JSON array of URLs on success
//   - 204 No Content if user has no URLs
//   - 401 Unauthorized if user is not authenticated
//   - 500 Internal Server Error for storage failures
//
// Response format (array of objects):
// [
//
//	{
//	  "original_url": "https://example.com",
//	  "short_url": "http://localhost:8080/abc123"
//	}
//
// ]
//
// The handler extracts user ID from request context (set by authentication middleware)
// and queries storage for all URLs belonging to that user.
func (h *URLHandler) getAPIUserUrls(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID := h.getUserIDFromContext(r.Context())
	if userID == "" {
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*5)
	defer cancel()

	urls, err := h.storage.GetByUserID(ctx, userID)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		return
	}

	if len(urls) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	var list []entity.ListByUserIDResponse
	for _, u := range urls {
		list = append(list, entity.ListByUserIDResponse{
			OriginalURL: u.OriginalURL,
			ShortURL:    fmt.Sprintf("%s/%s", h.baseURL, u.ShortCode),
		})
	}

	render.JSON(w, r, list)
}

// deleteAPIUserUrls handles batch deletion of user's URLs by their short codes.
//
// Endpoint: DELETE /api/user/urls
// Content-Type: application/json
// Authentication: Requires valid user authentication via context
//
// Request body: JSON array of short codes to delete
// ["abc123", "def456"]
//
// Returns:
//   - 202 Accepted on successful deletion request
//   - 400 Bad Request for invalid JSON or empty array
//   - 401 Unauthorized if user is not authenticated
//   - 500 Internal Server Error for storage failures
//
// Note: This operation marks URLs as deleted (soft delete) rather than
// physically removing them. The deletion is performed asynchronously,
// hence the 202 Accepted response.
func (h *URLHandler) deleteAPIUserUrls(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID := h.getUserIDFromContext(r.Context())
	if userID == "" {
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}

	body, err := h.readRequestBody(r)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var shortCodes []string
	err = json.Unmarshal(body, &shortCodes)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*5)
	defer cancel()

	if len(shortCodes) == 0 {
		return
	}

	err = h.storage.Deleted(ctx, shortCodes, userID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
