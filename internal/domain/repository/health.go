package repository

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
)

type HealthRepository interface {
	Ping(ctx context.Context) bool
}

type healthRepository struct {
	db *pgxpool.Pool
}

func NewHealthRepository(db *pgxpool.Pool) HealthRepository {
	return &healthRepository{db: db}
}

func (h healthRepository) Ping(ctx context.Context) bool {
	err := h.db.Ping(ctx)
	if err != nil {
		return false
	}

	return true
}
