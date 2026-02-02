package handler

import (
	"context"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// pingDB handles database health check requests.
// It verifies the connection to the database storage with a 5-second timeout.
//
// Response:
//   - 200 OK with "ok" message if the database is reachable
//   - 500 Internal Server Error with error message if the database is unreachable
//
// The method also handles write errors to the response writer by logging them.
func (h *URLHandler) pingDB(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second*5)
	defer cancel()
	err := h.storage.Ping(ctx)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, err = w.Write([]byte(err.Error()))
		if err != nil {
			zap.L().Error("write error", zap.Error(err))
			return
		}
		return
	}

	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte("ok"))
	if err != nil {
		zap.L().Error("write error", zap.Error(err))
		return
	}
}
