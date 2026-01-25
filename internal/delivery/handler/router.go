package handler

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	mw "github.com/skiphead/practicum/internal/middleware"
)

// ChiMux - создание роутера с middleware
func (h *URLHandler) ChiMux() *chi.Mux {
	r := chi.NewRouter()

	// Порядок middleware важен!
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(mw.CompressionMiddleware) // Существующий middleware сжатия
	r.Use(h.auditMiddleware.Wrap)   // Наш middleware аудита
	r.Use(mw.LoggingMiddleware)     // Существующий middleware логирования

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
