package handlers

import "github.com/go-chi/chi/v5"

func (h *URLHandler) ChiMux() *chi.Mux {
	r := chi.NewRouter()
	r.Use(compressionMiddleware)
	r.Use(loggingMiddleware)
	r.Use(h.sessionMiddleware)
	r.Get("/{key}", h.redirectURL)
	r.Get("/api/user/urls", h.getAPIUserUrls)
	r.Delete("/api/user/urls", h.deleteAPIUserUrls)
	r.Post("/", h.createShortURL)
	r.Post("/api/shorten", h.createShortAPIURL)
	r.Post("/api/shorten/batch", h.createBatchShortAPIURL)
	r.Get("/ping", h.pingDB)

	return r
}
