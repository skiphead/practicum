package usecase

import (
	"testing"

	"github.com/skiphead/practicum/internal/domain/entity"
	"github.com/skiphead/practicum/internal/domain/repository"
)

func TestIPCheckerUseCase(t *testing.T) {
	tests := []struct {
		name        string
		subnetCIDR  string
		ipAddress   string
		wantTrusted bool
		wantErr     bool
	}{
		{
			name:        "Valid IP in subnet",
			subnetCIDR:  "192.168.1.0/24",
			ipAddress:   "192.168.1.100",
			wantTrusted: true,
			wantErr:     false,
		},
		{
			name:        "Valid IP outside subnet",
			subnetCIDR:  "192.168.1.0/24",
			ipAddress:   "10.0.0.1",
			wantTrusted: false,
			wantErr:     false,
		},
		{
			name:        "Invalid IP address",
			subnetCIDR:  "192.168.1.0/24",
			ipAddress:   "invalid",
			wantTrusted: false,
			wantErr:     true,
		},
		{
			name:        "Empty subnet - always false",
			subnetCIDR:  "",
			ipAddress:   "192.168.1.100",
			wantTrusted: false,
			wantErr:     false,
		},
		{
			name:        "IPv6 valid in subnet",
			subnetCIDR:  "2001:db8::/32",
			ipAddress:   "2001:db8:1234::1",
			wantTrusted: true,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subnet, _ := entity.NewSubnetEntity(tt.subnetCIDR)
			repo := repository.NewIPCheckerRepository(subnet)
			uc := NewIPCheckerUseCase(repo)

			trusted, err := uc.CheckIP(tt.ipAddress)

			if (err != nil) != tt.wantErr {
				t.Errorf("CheckIP() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil && trusted != tt.wantTrusted {
				t.Errorf("CheckIP() = %v, want %v", trusted, tt.wantTrusted)
			}

			// Test GetTrustedSubnetInfo
			info := uc.GetTrustedSubnetInfo()
			if info == "" {
				t.Error("GetTrustedSubnetInfo() returned empty string")
			}
		})
	}
}
