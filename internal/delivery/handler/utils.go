package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"go.uber.org/zap"
)

// processAndSaveURL processes a URL for shortening, validates it, and saves it to storage.
// It returns the shortened URL, a boolean indicating if it's a duplicate, and any error.
//
// Parameters:
//   - originalURL: The URL to be shortened
//   - w: HTTP response writer
//   - r: HTTP request
//
// Returns:
//   - string: The shortened URL (empty if error occurred)
//   - bool: true if URL is a duplicate, false otherwise
//   - error: Any error that occurred during processing
//
// The function performs the following steps:
// 1. Validates the URL format and protocol
// 2. Extracts user ID from request context
// 3. Saves the URL to storage with a 5-second timeout
// 4. Handles duplicate URL conflicts
// 5. Builds the full shortened URL
func (h *URLHandler) processAndSaveURL(originalURL string, w http.ResponseWriter, r *http.Request) (string, bool, error) {
	if err := h.validateURL(originalURL, w); err != nil {
		return "", false, err
	}

	userID := h.getUserIDFromContext(r.Context())
	if userID == "" {
		http.Error(w, "User not found", http.StatusUnauthorized)
		return "", false, fmt.Errorf("user not found")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	resp, err := h.storage.Save(ctx, originalURL, userID)
	if err != nil {
		if h.storage.IsDuplicateError(err) {
			return h.buildShortURL(resp.ShortCode), true, nil
		}
		zap.L().Error("save error", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return "", false, err
	}

	return h.buildShortURL(resp.ShortCode), false, nil
}

// readRequestBody reads and returns the entire body of an HTTP request.
// The caller is responsible for closing the request body.
//
// Parameters:
//   - r: HTTP request
//
// Returns:
//   - []byte: The request body content
//   - error: Any error that occurred during reading
func (h *URLHandler) readRequestBody(r *http.Request) ([]byte, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	defer h.closeBody(r.Body)
	return body, nil
}

// validateURL performs comprehensive validation of a URL string.
// It checks for empty URLs, valid format, required components (scheme and host),
// and restricts allowed protocols to HTTP and HTTPS only.
//
// Parameters:
//   - originalURL: The URL string to validate
//   - w: HTTP response writer for sending error responses
//
// Returns:
//   - error: Validation error if URL is invalid, nil otherwise
//
// Validation checks:
// 1. Non-empty URL
// 2. Valid URL format
// 3. Presence of scheme (protocol)
// 4. Presence of host
// 5. Allowed protocols (http or https only)
func (h *URLHandler) validateURL(originalURL string, w http.ResponseWriter) error {
	if originalURL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return fmt.Errorf("URL is required")
	}

	// Parse URL for detailed validation
	u, err := url.Parse(originalURL)
	if err != nil {
		http.Error(w, "Invalid URL format", http.StatusBadRequest)
		return fmt.Errorf("invalid URL format: %v", err)
	}

	// Check for scheme (protocol) presence
	if u.Scheme == "" {
		http.Error(w, "URL scheme (protocol) is required", http.StatusBadRequest)
		return fmt.Errorf("URL scheme is required")
	}

	// Check for host presence
	if u.Host == "" {
		http.Error(w, "URL host is required", http.StatusBadRequest)
		return fmt.Errorf("URL host is required")
	}

	// Check allowed protocols (only HTTP and HTTPS)
	if u.Scheme != "http" && u.Scheme != "https" {
		http.Error(w, "Only HTTP and HTTPS protocols are allowed", http.StatusBadRequest)
		return fmt.Errorf("invalid URL scheme: %s (only http and https are allowed)", u.Scheme)
	}

	return nil
}

// buildShortURL constructs the full shortened URL from a key.
//
// Parameters:
//   - key: The unique identifier/short code for the URL
//
// Returns:
//   - string: The complete shortened URL in format "baseURL/key"
func (h *URLHandler) buildShortURL(key string) string {
	return fmt.Sprintf("%s/%s", h.baseURL, key)
}

// closeBody safely closes an io.ReadCloser and logs any errors.
// This helper function ensures proper resource cleanup.
//
// Parameters:
//   - body: The io.ReadCloser to close
func (h *URLHandler) closeBody(body io.ReadCloser) {
	if err := body.Close(); err != nil {
		zap.L().Error("error close Body", zap.Error(err))
	}
}

// handleConflictError checks if an error is a duplicate URL conflict error.
// If it is, it sends a 409 Conflict response and returns true.
//
// Parameters:
//   - w: HTTP response writer
//   - err: The error to check
//
// Returns:
//   - bool: true if the error is a duplicate conflict, false otherwise
func (h *URLHandler) handleConflictError(w http.ResponseWriter, err error) bool {
	if h.storage.IsDuplicateError(err) {
		http.Error(w, err.Error(), http.StatusConflict)
		return true
	}
	return false
}
