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
