// Package handler provides HTTP handlers for the URL shortening service.
// It implements the presentation layer, handling:
//   - Creation of short URLs via JSON API and form submissions
//   - Retrieval and deletion of user's URLs
//   - Redirection from short keys to original URLs
//   - Internal statistics endpoints with IP-based access control
//
// The package follows clean architecture principles, delegating business logic
// to use cases and using middleware for cross-cutting concerns like logging,
// compression, session management, and IP validation.
package handler

import (
	"github.com/skiphead/practicum/internal/audit"
	"github.com/skiphead/practicum/internal/usecase"
)

// URLHandler provides HTTP handlers for URL shortening operations.
// It encapsulates all dependencies required to process HTTP requests
// and delegates business logic to the appropriate use cases.
//
// The handler is designed to be stateless, with all shared state
// managed through the injected dependencies. This enables easier testing
// and ensures clear separation of concerns.
type URLHandler struct {
	storage          usecase.URLUseCase        // Business logic layer for URL operations (create, get, delete)
	auditClient      *audit.Adapter            // Audit logging client for tracking operations
	serverAddr       string                    // Server address for binding (e.g., ":8080")
	baseURL          string                    // Base URL for generating short URLs (e.g., "http://localhost:8080")
	sessionKey       string                    // Secret key for session management and authentication
	ipCheckerUseCase *usecase.IPCheckerUseCase // Use case for IP validation on internal endpoints
}

// NewURLHandler creates a new URLHandler instance with all required dependencies.
//
// Parameters:
//   - storage: Implementation of URLUseCase interface providing URL business logic
//   - ipCheck: IPCheckerUseCase for validating access to internal endpoints
//   - serverAddr: Network address for the HTTP server to bind to (e.g., ":8080" or "localhost:8080")
//   - baseURL: Public base URL used when constructing shortened URLs (e.g., "http://localhost:8080")
//   - sessionKey: Secret key used for signing and validating session cookies
//   - auditClient: Adapter for sending audit logs about URL operations
//
// Returns:
//   - *URLHandler: A fully initialized handler ready to be registered with an HTTP router
//
// Dependencies Explanation:
//   - storage: Core business logic for URL shortening operations
//   - ipCheck: Optional but recommended for internal endpoints requiring subnet validation
//   - serverAddr: Used by the server, passed through for convenience
//   - baseURL: Determines the format of generated short URLs (e.g., "http://localhost:8080/abc123")
//   - sessionKey: Must be kept secret and consistent across service restarts
//   - auditClient: Asynchronous audit logging; can be nil if audit is disabled
//
// Example:
//
//	// Initialize dependencies
//	storage := usecase.NewURLUseCase(repo, baseURL)
//	ipChecker := usecase.NewIPCheckerUseCase(ipRepo)
//	auditor := audit.NewAdapter(auditConfig)
//
//	// Create handler
//	handler := NewURLHandler(
//	    storage,
//	    ipChecker,
//	    ":8080",
//	    "http://localhost:8080",
//	    "your-super-secret-session-key",
//	    auditor,
//	)
//
//	// Register with router
//	r := chi.NewRouter()
//	r.Mount("/", handler.ChiMux())
//
// Note: The sessionKey should be loaded from environment variables or secure
// configuration, never hardcoded or committed to version control.
func NewURLHandler(
	storage usecase.URLUseCase,
	ipCheck *usecase.IPCheckerUseCase,
	serverAddr, baseURL, sessionKey string,
	auditClient *audit.Adapter,
) *URLHandler {
	return &URLHandler{
		storage:          storage,
		serverAddr:       serverAddr,
		baseURL:          baseURL,
		sessionKey:       sessionKey,
		auditClient:      auditClient,
		ipCheckerUseCase: ipCheck,
	}
}
