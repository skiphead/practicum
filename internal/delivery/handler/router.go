package handler

import (
	"github.com/go-chi/chi/v5"
	mw "github.com/skiphead/practicum/internal/middleware"
)

// ChiMux - создание роутера с middleware
func (h *URLHandler) ChiMux() *chi.Mux {
	r := chi.NewRouter()

	// Порядок middleware важен!
	r.Use(mw.CompressionMiddleware)
	r.Use(h.sessionMiddleware)
	r.Use(mw.LoggingMiddleware)

	// Маршруты (все покрыты аудитом через middleware)
	r.Get("/{key}", h.redirectURL)                         // GET /{id} - переход по ссылке
	r.Get("/api/user/urls", h.getAPIUserUrls)              // GET /api/user/urls
	r.Delete("/api/user/urls", h.deleteAPIUserUrls)        // DELETE /api/user/urls
	r.Post("/", h.createShortURL)                          // POST / - создание через форму
	r.Post("/api/shorten", h.createShortAPIURL)            // POST /api/shorten - создание через JSON API
	r.Post("/api/shorten/batch", h.createBatchShortAPIURL) // POST /api/shorten/batch
	r.Get("/ping", h.pingDB)                               // GET /ping

	return r
}
