package repository

import (
	"testing"

	"github.com/skiphead/practicum/internal/domain/entity"
)

func TestIPCheckerRepositoryImpl(t *testing.T) {
	tests := []struct {
		name        string
		subnetCIDR  string
		testIP      string
		wantTrusted bool
	}{
		{
			name:        "Empty subnet - IP not trusted",
			subnetCIDR:  "",
			testIP:      "192.168.1.100",
			wantTrusted: false,
		},
		{
			name:        "IP in trusted subnet",
			subnetCIDR:  "192.168.1.0/24",
			testIP:      "192.168.1.100",
			wantTrusted: true,
		},
		{
			name:        "IP outside trusted subnet",
			subnetCIDR:  "192.168.1.0/24",
			testIP:      "10.0.0.1",
			wantTrusted: false,
		},
		{
			name:        "IPv6 in trusted subnet",
			subnetCIDR:  "2001:db8::/32",
			testIP:      "2001:db8:1234::1",
			wantTrusted: true,
		},
		{
			name:        "IPv6 outside trusted subnet",
			subnetCIDR:  "2001:db8::/32",
			testIP:      "2002:db8::1",
			wantTrusted: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subnet, _ := entity.NewSubnetEntity(tt.subnetCIDR)
			repo := NewIPCheckerRepository(subnet)

			ip, _ := entity.NewIPValueObject(tt.testIP)
			got := repo.IsIPTrusted(ip)

			if got != tt.wantTrusted {
				t.Errorf("IsIPTrusted() = %v, want %v", got, tt.wantTrusted)
			}

			// Test GetTrustedSubnet
			returnedSubnet := repo.GetTrustedSubnet()
			if returnedSubnet.Cidr != subnet.Cidr {
				t.Errorf("GetTrustedSubnet() returned wrong subnet")
			}
		})
	}
}
