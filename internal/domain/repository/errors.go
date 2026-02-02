package repository

import (
	"fmt"
)

// ConflictError represents an application-level conflict error that occurs
// when attempting to create or update a resource that would violate business rules.
// This error type provides structured information about the conflict for
// better error handling and user feedback.
type ConflictError struct {
	Field   string `json:"field"`   // The field that caused the conflict (e.g., "short_code", "original_url")
	Value   any    `json:"value"`   // The conflicting value that was provided
	Message string `json:"message"` // Human-readable description of the conflict
}

// Error implements the error interface for ConflictError.
// It returns a formatted string containing all conflict details.
//
// Returns:
//   - string: Formatted error message with field, value, and conflict description
func (e *ConflictError) Error() string {
	return fmt.Sprintf("conflict on field '%s' with value '%v': %s", e.Field, e.Value, e.Message)
}

// NewConflictError creates and returns a new ConflictError instance.
// This constructor provides a consistent way to create conflict errors throughout the application.
//
// Parameters:
//   - field: The field name where the conflict occurred
//   - value: The conflicting value that was attempted
//   - message: Descriptive message explaining the conflict
//
// Returns:
//   - *ConflictError: New conflict error instance
//
// Example:
//
//	return NewConflictError("short_code", "abc123", "short code already exists")
func NewConflictError(field string, value any, message string) *ConflictError {
	return &ConflictError{
		Field:   field,
		Value:   value,
		Message: message,
	}
}
