package repository

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"
)

// IsDuplicateError checks if an error is a PostgreSQL duplicate key violation.
// It identifies unique constraint violations (error code 23505) which occur when
// trying to insert a record that would violate a UNIQUE or PRIMARY KEY constraint.
//
// Parameters:
//   - err: The error to check for duplicate violations
//
// Returns:
//   - bool: true if the error is a PostgreSQL duplicate key violation, false otherwise
//
// PostgreSQL Error Code 23505 corresponds to "unique_violation" - when an insert or update
// violates a unique constraint. This commonly occurs with:
//   - Duplicate short codes
//   - Duplicate original URLs (if URL uniqueness is enforced)
//   - Duplicate correlation IDs in batch operations
//
// The method logs debug information about the constraint violation including:
//   - Constraint name that was violated
//   - Detailed error message from PostgreSQL
func (r *storageRepository) IsDuplicateError(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		zap.L().Debug("Duplicate key violation detected",
			zap.String("constraint", pgErr.ConstraintName),
			zap.String("detail", pgErr.Detail))
		return true
	}
	return false
}
