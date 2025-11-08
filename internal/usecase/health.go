package usecase

import (
	"context"
	"github.com/skiphead/practicum/internal/domain/repository"
)

type HealthUseCase struct {
	HealthRepo repository.HealthRepository
}

func NewHealthUseCase(healthRepo repository.HealthRepository) *HealthUseCase {
	return &HealthUseCase{HealthRepo: healthRepo}
}

func (h HealthUseCase) Health(ctx context.Context) bool {

	if h.HealthRepo.Ping(ctx) {
		return true
	}

	return false
}
