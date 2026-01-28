package handler

import (
	"context"
	"net/http"
	"time"

	"go.uber.org/zap"
)

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
