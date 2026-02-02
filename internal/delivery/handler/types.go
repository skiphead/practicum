// Package handler provides HTTP handlers for URL shortening service.
// It handles creation of short URLs via both JSON API and plain text endpoints,
// retrieval of user's URLs, batch deletion, and redirection to original URLs.
package handler

import (
	"github.com/skiphead/practicum/internal/audit"
	"github.com/skiphead/practicum/internal/usecase"
)

// URLHandler provides HTTP handlers for URL shortening operations.
// It encapsulates all dependencies needed to handle URL-related HTTP requests.
type URLHandler struct {
	storage     usecase.URLUseCase // Business logic layer for URL operations
	auditClient *audit.Adapter     // Audit logging client
	serverAddr  string             // Server address for binding
	baseURL     string             // Base URL for generating short URLs (e.g., "http://localhost:8080")
	sessionKey  string             // Key for session management/authentication
}

// NewURLHandler creates a new URLHandler with the specified dependencies.
//
// Parameters:
//   - storage: Business logic layer implementing URLUseCase interface
//   - serverAddr: Server address (e.g., ":8080" for binding)
//   - baseURL: Base URL for generating short URLs (e.g., "http://localhost:8080")
//   - sessionKey: Secret key for session management/authentication
//   - auditClient: Audit logging adapter for tracking operations
//
// Returns a configured URLHandler ready to handle HTTP requests.
//
// Example:
//
//	storage := // initialize your use case
//	auditClient := audit.NewAdapter()
//	handler := NewURLHandler(
//	    storage,
//	    ":8080",
//	    "http://localhost:8080",
//	    "your-secret-session-key",
//	    auditClient,
//	)
func NewURLHandler(storage usecase.URLUseCase, serverAddr, baseURL, sessionKey string, auditClient *audit.Adapter) *URLHandler {
	return &URLHandler{
		storage:     storage,
		serverAddr:  serverAddr,
		baseURL:     baseURL,
		sessionKey:  sessionKey,
		auditClient: auditClient,
	}
}
