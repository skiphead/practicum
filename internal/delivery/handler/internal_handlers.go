package handler

import (
	"encoding/json"
	"net/http"
	"time"
)

// statsHandler handles requests for internal server statistics.
// This endpoint requires IP-based access control and returns various
// runtime metrics about the service.
//
// Access Control:
//   - Requires trusted IP check before processing (implemented separately via middleware)
//   - Extracts client IP from X-Real-IP header for logging/audit purposes
//
// HTTP Method: GET (implied, should be validated by router)
// Endpoint: Internal/stats (actual path depends on router configuration)
//
// Response Format (JSON):
//
//	{
//	  "message": "Internal statistics",
//	  "client_ip": "192.168.1.100",           // Client IP from X-Real-IP header
//	  "stats": {
//	    "requests_total": 12345,               // Total requests handled since startup
//	    "active_connections": 42,               // Currently active connections
//	    "uptime_seconds": 3600,                  // Service uptime in seconds
//	    "memory_usage_mb": 128.5                 // Current memory usage in MB
//	  },
//	  "timestamp": "2024-01-15T10:30:00Z"       // Response timestamp (RFC3339)
//	}
//
// Status Codes:
//   - 200 OK: Statistics successfully retrieved
//   - 403 Forbidden: If IP-based access control fails (handled by middleware)
//   - 405 Method Not Allowed: If non-GET request is received
//
// Note: This is a mock implementation. In production, statistics should be
// collected from actual service metrics (e.g., Prometheus, expvar, or custom counters).
func (h *URLHandler) statsHandler(w http.ResponseWriter, r *http.Request) {
	// Extract client IP from header (should be set by proxy/load balancer)
	realIP := r.Header.Get("X-Real-IP")

	// Build response with mock statistics
	response := map[string]interface{}{
		"message":   "Internal statistics",
		"client_ip": realIP,
		"stats": map[string]interface{}{
			"requests_total":     12345,
			"active_connections": 42,
			"uptime_seconds":     3600,
			"memory_usage_mb":    128.5,
		},
		"timestamp": time.Now().Format(time.RFC3339),
	}

	// Set JSON content type and encode response
	w.Header().Set("Content-Type", "application/json")

	// Note: Error handling for JSON encoding is omitted for brevity
	// In production, should check and handle json.NewEncoder.Encode() error
	json.NewEncoder(w).Encode(response)
}
