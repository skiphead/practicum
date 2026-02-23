package handler

import (
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	mw "github.com/skiphead/practicum/internal/middleware"
	"go.uber.org/zap"
)

// ChiMux creates and configures a Chi router with middleware and routes for the URL shortening service.
//
// Middleware Stack Order (applied in sequence):
//  1. CompressionMiddleware - Handles gzip compression for requests/responses
//  2. sessionMiddleware - Manages user sessions and authentication via cookies
//  3. LoggingMiddleware - Logs request details (method, path, duration, status)
//
// Route Groups:
//
//	Public Routes (all requests):
//	  GET    /{key}                    - Redirect to original URL by short key
//	  GET    /api/user/urls             - Get all shortened URLs for current user
//	  DELETE /api/user/urls             - Delete user's URLs (batch operation)
//	  POST   /                          - Create short URL via form submission
//	  POST   /api/shorten                - Create short URL via JSON API
//	  POST   /api/shorten/batch           - Create multiple short URLs via batch API
//	  GET    /ping                       - Database health check endpoint
//
//	Protected Internal Routes (require IP validation):
//	  GET    /api/internal/stats          - Get service statistics (requires trusted IP)
//	  (Additional internal routes can be added here)
//
// Returns:
//   - *chi.Mux: Fully configured router with all middleware and routes registered
//
// Notes:
//   - All routes are automatically covered by audit logging through the LoggingMiddleware
//   - IP validation for internal routes uses X-Real-IP header (set by proxy/load balancer)
//   - Session middleware handles anonymous users by creating sessions when needed
//   - The router is ready to be used with http.Server
func (h *URLHandler) ChiMux() *chi.Mux {
	r := chi.NewRouter()

	// Public routes - accessible to all clients
	r.Group(func(r chi.Router) {
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
	})

	// Protected internal routes - require trusted IP validation
	r.Group(func(r chi.Router) {
		// Apply IPCheckMiddleware only to this group of routes
		r.Use(h.IPCheckMiddleware)

		// Statistics endpoint requiring IP validation
		r.Get("/api/internal/stats", h.statsHandler)

		// Additional protected routes can be added here
		// r.Get("/api/internal/other", h.OtherInternalHandler)
	})

	return r
}

// IPCheckMiddleware validates that the client IP belongs to the trusted subnet
// before allowing access to protected internal endpoints.
//
// The middleware extracts the client IP from the X-Real-IP header, which should
// be set by a reverse proxy or load balancer. Direct access without this header
// will result in access denial.
//
// Flow:
//  1. Extract IP from X-Real-IP header
//  2. Validate IP format and check against trusted subnet using IPCheckerUseCase
//  3. Allow access if IP is trusted, otherwise return 403 Forbidden
//
// Headers:
//   - X-Real-IP: Required header containing the actual client IP address
//
// Status Codes:
//   - 200 OK: Request proceeds to the next handler (IP is trusted)
//   - 403 Forbidden: IP is not in trusted subnet or X-Real-IP header is missing
//
// Logging:
//   - Logs access attempts with client IP and any validation errors
//   - Uses zap logger for structured logging of successful validations
//
// Usage:
//
//	r.Group(func(r chi.Router) {
//	    r.Use(h.IPCheckMiddleware)
//	    r.Get("/api/internal/stats", h.statsHandler)
//	})
//
// Security Notes:
//   - Never trust X-Real-IP header directly from internet-facing clients
//   - Ensure reverse proxy overwrites this header and strips it from client requests
//   - Consider adding X-Forwarded-For support if behind multiple proxies
func (h *URLHandler) IPCheckMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get IP from header (set by reverse proxy)
		realIP := r.Header.Get("X-Real-IP")
		realIP = strings.TrimSpace(realIP)

		// Check IP through use case
		isTrusted, err := h.ipCheckerUseCase.CheckIP(realIP)
		if err != nil || !isTrusted {
			log.Printf("Access denied for internal endpoint from IP: %s, error: %v", realIP, err)
			http.Error(w, "Forbidden: IP not in trusted subnet", http.StatusForbidden)
			return
		}

		zap.L().Sugar().Infow("IP check passed", "ip", realIP)

		// Pass control to the next handler
		next.ServeHTTP(w, r)
	})
}
