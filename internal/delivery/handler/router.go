package handler

import (
	"github.com/go-chi/chi/v5"
	mw "github.com/skiphead/practicum/internal/middleware"
)

// ChiMux creates and configures a Chi router with middleware and routes.
// The order of middleware is important for proper request processing.
// All routes are covered by audit through middleware.
//
// Returns a configured *chi.Mux router with the following middleware stack:
// 1. CompressionMiddleware - handles request/response compression
// 2. sessionMiddleware - manages user sessions and authentication
// 3. LoggingMiddleware - logs request details
//
// The router includes the following routes:
// - GET /{key} - redirect to original URL by short key
// - GET /api/user/urls - get all shortened URLs for current user
// - DELETE /api/user/urls - delete user's URLs
// - POST / - create short URL via form
// - POST /api/shorten - create short URL via JSON API
// - POST /api/shorten/batch - create multiple short URLs via batch API
// - GET /ping - database health check
func (h *URLHandler) ChiMux() *chi.Mux {
	r := chi.NewRouter()

	// Order of middleware matters!
	r.Use(mw.CompressionMiddleware)
	r.Use(h.sessionMiddleware)
	r.Use(mw.LoggingMiddleware)

	// Routes (all covered by audit through middleware)
	r.Get("/{key}", h.RedirectURL)                         // GET /{id} - redirect to original URL
	r.Get("/api/user/urls", h.getAPIUserUrls)              // GET /api/user/urls - get user's URLs
	r.Delete("/api/user/urls", h.deleteAPIUserUrls)        // DELETE /api/user/urls - delete user's URLs
	r.Post("/", h.createShortURL)                          // POST / - create short URL via form
	r.Post("/api/shorten", h.CreateShortAPIURL)            // POST /api/shorten - create short URL via JSON API
	r.Post("/api/shorten/batch", h.createBatchShortAPIURL) // POST /api/shorten/batch - batch URL creation
	r.Get("/ping", h.pingDB)                               // GET /ping - database health check

	return r
}
