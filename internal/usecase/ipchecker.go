// Package usecase implements the application's business logic (use cases)
// following the clean architecture principles.
package usecase

import (
	"github.com/skiphead/practicum/internal/domain/entity"
	"github.com/skiphead/practicum/internal/domain/repository"
)

// IPCheckerUseCase implements business logic for IP address validation
// and trusted subnet checking operations.
type IPCheckerUseCase struct {
	ipCheckerRepo repository.IPCheckerRepository
}

// NewIPCheckerUseCase creates a new IP checker use case instance.
//
// Parameters:
//   - repo: Repository implementation that provides access to trusted subnet data
//
// Returns:
//   - *IPCheckerUseCase: Configured use case instance ready for IP checking operations
func NewIPCheckerUseCase(repo repository.IPCheckerRepository) *IPCheckerUseCase {
	return &IPCheckerUseCase{
		ipCheckerRepo: repo,
	}
}

// CheckIP validates an IP address and checks if it belongs to the trusted subnet.
//
// The function performs two main operations:
// 1. Validates the IP address format by creating a domain value object
// 2. Checks if the validated IP belongs to the trusted subnet through the repository
//
// Parameters:
//   - ipAddress: String representation of the IP address to check (IPv4 or IPv6)
//
// Returns:
//   - bool: true if the IP address belongs to the trusted subnet, false otherwise
//   - error: Returns error only if the IP address format is invalid.
//     Returns nil error for valid IPs even if they're not trusted.
//
// Example:
//
//	trusted, err := checker.CheckIP("192.168.1.100")
//	if err != nil {
//	    // Handle invalid IP format
//	}
//	if trusted {
//	    // IP is from trusted subnet
//	}
func (uc *IPCheckerUseCase) CheckIP(ipAddress string) (bool, error) {
	// Create IP value object (validation at domain level)
	ip, err := entity.NewIPValueObject(ipAddress)
	if err != nil {
		return false, err
	}

	// Check IP through repository
	return uc.ipCheckerRepo.IsIPTrusted(ip), nil
}

// GetTrustedSubnetInfo returns the string representation of the currently
// configured trusted subnet.
//
// This method provides access to the trusted subnet information without
// exposing the internal subnet object structure.
//
// Returns:
//   - string: CIDR notation of the trusted subnet (e.g., "192.168.1.0/24").
//     Returns empty string if no trusted subnet is configured.
func (uc *IPCheckerUseCase) GetTrustedSubnetInfo() string {
	subnet := uc.ipCheckerRepo.GetTrustedSubnet()
	return subnet.String()
}
