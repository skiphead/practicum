package entity

import (
	"encoding/json"
	"testing"
)

func TestShortenRequest_JSON(t *testing.T) {
	tests := []struct {
		name    string
		request ShortenRequest
		want    string
	}{
		{
			name:    "Regular URL",
			request: ShortenRequest{URL: "https://example.com"},
			want:    `{"url":"https://example.com"}`,
		},
		{
			name:    "Empty URL",
			request: ShortenRequest{URL: ""},
			want:    `{"url":""}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Тестируем маршалинг
			got, err := json.Marshal(tt.request)
			if err != nil {
				t.Errorf("Marshal() error = %v", err)
				return
			}
			if string(got) != tt.want {
				t.Errorf("Marshal() got = %s, want %s", got, tt.want)
			}

			// Тестируем демаршалинг
			var unmarshalled ShortenRequest
			if err := json.Unmarshal(got, &unmarshalled); err != nil {
				t.Errorf("Unmarshal() error = %v", err)
				return
			}
			if unmarshalled != tt.request {
				t.Errorf("Unmarshal() got = %v, want %v", unmarshalled, tt.request)
			}
		})
	}
}

func TestShortenResponse_JSON(t *testing.T) {
	tests := []struct {
		name     string
		response ShortenResponse
		want     string
	}{
		{
			name:     "Regular result",
			response: ShortenResponse{Result: "abc123"},
			want:     `{"result":"abc123"}`,
		},
		{
			name:     "Empty result",
			response: ShortenResponse{Result: ""},
			want:     `{"result":""}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Тестируем маршалинг
			got, err := json.Marshal(tt.response)
			if err != nil {
				t.Errorf("Marshal() error = %v", err)
				return
			}
			if string(got) != tt.want {
				t.Errorf("Marshal() got = %s, want %s", got, tt.want)
			}

			// Тестируем демаршалинг
			var unmarshalled ShortenResponse
			if err := json.Unmarshal(got, &unmarshalled); err != nil {
				t.Errorf("Unmarshal() error = %v", err)
				return
			}
			if unmarshalled != tt.response {
				t.Errorf("Unmarshal() got = %v, want %v", unmarshalled, tt.response)
			}
		})
	}
}

func TestIPValueObject(t *testing.T) {
	tests := []struct {
		name    string
		address string
		wantErr bool
	}{
		{"Valid IPv4", "192.168.1.1", false},
		{"Valid IPv6", "2001:0db8:85a3:0000:0000:8a2e:0370:7334", false},
		{"Valid localhost", "127.0.0.1", false},
		{"Invalid IP", "256.256.256.256", true},
		{"Empty string", "", true},
		{"Invalid format", "not-an-ip", true},
		{"Partial IP", "192.168", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip, err := NewIPValueObject(tt.address)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewIPValueObject() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if ip.String() != tt.address {
					t.Errorf("IP.String() = %v, want %v", ip.String(), tt.address)
				}
			}
		})
	}
}

func TestSubnetEntity(t *testing.T) {
	tests := []struct {
		name         string
		cidr         string
		wantErr      bool
		wantAllowAll bool
		wantEmpty    bool
		testIP       string
		wantContains bool
	}{

		{
			name:         "Valid IPv4 subnet",
			cidr:         "192.168.1.0/24",
			wantErr:      false,
			wantAllowAll: false,
			wantEmpty:    false,
			testIP:       "192.168.1.100",
			wantContains: true,
		},
		{
			name:         "IP outside subnet",
			cidr:         "192.168.1.0/24",
			wantErr:      false,
			wantAllowAll: false,
			wantEmpty:    false,
			testIP:       "10.0.0.1",
			wantContains: false,
		},
		{
			name:         "Valid IPv6 subnet",
			cidr:         "2001:db8::/32",
			wantErr:      false,
			wantAllowAll: false,
			wantEmpty:    false,
			testIP:       "2001:db8:1234::1",
			wantContains: true,
		},
		{
			name:         "Invalid CIDR",
			cidr:         "invalid",
			wantErr:      true,
			wantAllowAll: false,
			wantEmpty:    false,
		},
		{
			name:         "CIDR without network",
			cidr:         "192.168.1.1/24",
			wantErr:      false, // ParseCIDR doesn't validate network part strictly
			wantAllowAll: false,
			wantEmpty:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subnet, err := NewSubnetEntity(tt.cidr)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewSubnetEntity() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if subnet.IsEmpty() != tt.wantEmpty {
					t.Errorf("IsEmpty() = %v, want %v", subnet.IsEmpty(), tt.wantEmpty)
				}

				if tt.testIP != "" {
					ip, _ := NewIPValueObject(tt.testIP)
					if subnet.Contains(ip) != tt.wantContains {
						t.Errorf("Contains(%v) = %v, want %v", tt.testIP, subnet.Contains(ip), tt.wantContains)
					}
				}

				// Test String() method
				str := subnet.String()
				if tt.wantEmpty && str != "not configured (all internal requests will be denied)" {
					t.Errorf("String() for empty = %v, want expected message", str)
				}
				if tt.wantAllowAll && str != "0.0.0.0/0 (all IPs allowed)" {
					t.Errorf("String() for allowAll = %v, want expected message", str)
				}
			}
		})
	}
}
