package utils

import (
	"testing"
)

func TestIsValidURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{"valid HTTP URL", "http://example.com", true},
		{"valid HTTPS URL", "https://example.com", true},
		{"valid URL with path", "https://example.com/path/to/resource", true},
		{"valid URL with query params", "https://example.com?param=value", true},
		{"invalid URL - no scheme", "examples_test.com", false},
		{"invalid URL - bad scheme", "ftp://examples_test.com", false},
		{"invalid URL - empty string", "", false},
		{"invalid URL - malformed", "://examples_test.com", false},
		{"invalid URL - spaces", "https://example.com/path with spaces", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidURL(tt.url)
			if result != tt.expected {
				t.Errorf("IsValidURL(%q) = %v, expected %v", tt.url, result, tt.expected)
			}
		})
	}
}
