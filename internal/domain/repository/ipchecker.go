package repository

import "github.com/skiphead/practicum/internal/domain/entity"

// IPCheckerRepository определяет контракт для проверки IP
type IPCheckerRepository interface {
	IsIPTrusted(ip *entity.IPValueObject) bool
	GetTrustedSubnet() *entity.SubnetEntity
}

// IPCheckerRepositoryImpl реализует интерфейс IPCheckerRepository
type IPCheckerRepositoryImpl struct {
	trustedSubnet *entity.SubnetEntity
}

// NewIPCheckerRepository создает новый репозиторий
func NewIPCheckerRepository(subnet *entity.SubnetEntity) IPCheckerRepository {
	return &IPCheckerRepositoryImpl{
		trustedSubnet: subnet,
	}
}

// IsIPTrusted проверяет, является ли IP доверенным
func (r *IPCheckerRepositoryImpl) IsIPTrusted(ip *entity.IPValueObject) bool {
	if r.trustedSubnet.IsEmpty() {
		return false
	}
	return r.trustedSubnet.Contains(ip)
}

// GetTrustedSubnet возвращает доверенную подсеть
func (r *IPCheckerRepositoryImpl) GetTrustedSubnet() *entity.SubnetEntity {
	return r.trustedSubnet
}
