package handlers

import (
	"context"
	"encoding/json"
	"github.com/go-chi/render"
	"github.com/skiphead/practicum/internal/domain/entity"
	"go.uber.org/zap"
	"net/http"
	"time"
)

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
	if err := json.Unmarshal(body, &original); err != nil {
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
