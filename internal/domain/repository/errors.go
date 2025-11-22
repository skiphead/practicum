package repository

import (
	"fmt"
)

type ConflictError struct {
	Field   string `json:"field"`
	Value   any    `json:"value"`
	Message string `json:"message"`
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("conflict on field '%s' with value '%v': %s", e.Field, e.Value, e.Message)
}

func NewConflictError(field string, value any, message string) *ConflictError {
	return &ConflictError{
		Field:   field,
		Value:   value,
		Message: message,
	}
}
