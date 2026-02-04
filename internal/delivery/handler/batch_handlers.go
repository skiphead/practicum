package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/skiphead/practicum/internal/domain/entity"
	"go.uber.org/zap"
)

// createBatchShortAPIURL handles batch creation of shortened URLs via JSON API.
// It accepts an array of URLs and returns an array of shortened URL objects.
// The method performs duplicate checking before creating new shortened URLs.
//
// Request:
//   - Method: POST
//   - Path: /api/shorten/batch
//   - Content-Type: application/json
//   - Body: JSON array of objects with "correlation_id" and "original_url" fields
//
// Response:
//   - 201 Created: Returns array of shortened URL objects with correlation IDs
//   - 409 Conflict: If duplicates found, returns existing shortened URLs
//   - 400 Bad Request: Invalid JSON or missing required fields
//   - 401 Unauthorized: User not authenticated
//   - 415 Unsupported Media Type: Incorrect Content-Type header
//   - 500 Internal Server Error: Storage or processing error
//
// The method includes a 5-second timeout for database operations.
func (h *URLHandler) createBatchShortAPIURL(w http.ResponseWriter, r *http.Request) {
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

	var original []entity.BatchShortenRequest
	if err = json.Unmarshal(body, &original); err != nil {
		zap.L().Error("unmarshal error", zap.Error(err))
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	userID := h.getUserIDFromContext(r.Context())
	if userID == "" {
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*5)
	defer cancel()

	duplicates, errDuplicates := h.storage.FindDuplicateURLs(ctx, original)
	if errDuplicates != nil {
		render.Status(r, http.StatusInternalServerError)
	}

	if len(duplicates) > 0 {
		w.WriteHeader(http.StatusConflict)
		render.JSON(w, r, duplicates)
		return
	}

	shortURLs, err := h.storage.BatchSave(ctx, original, userID)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	render.JSON(w, r, shortURLs)
}
