package repository

import (
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"
)

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
