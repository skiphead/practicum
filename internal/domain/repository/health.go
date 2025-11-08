package repository

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
)

type HealthRepository interface {
	Ping(ctx context.Context) error
}

type healthRepository struct {
	db *pgxpool.Pool
}

func NewHealthRepository(db *pgxpool.Pool) HealthRepository {
	return &healthRepository{db: db}
}

func (h *healthRepository) Ping(ctx context.Context) error {
	if err := h.db.Ping(ctx); err != nil {
		slog.ErrorContext(ctx, "database health check failed", "error", err)
		return err
	}
	return nil
}
