package config

import (
	"os"
	"testing"
)

// TestIsHTTPSSEnabled validates all truthy/falsy interpretations of ENABLE_HTTPS.
func TestIsHTTPSSEnabled(t *testing.T) {
	tests := []struct {
		name     string
		envValue string // "" means unset
		want     bool
	}{
		// Truthy values
		{"explicit true", "true", true},
		{"uppercase TRUE", "TRUE", true},
		{"numeric 1", "1", true},
		{"yes", "yes", true},
		{"YES with spaces", "  YES  ", true},
		{"on", "on", true},
		{"ON mixed case", "On", true},

		// Falsy values
		{"explicit false", "false", false},
		{"numeric 0", "0", false},
		{"no", "no", false},
		{"off", "off", false},
		{"unknown value", "maybe", false},
		{"empty string", "", false},

		// Edge cases
		{"only whitespace", "   ", false},
		{"unset variable", "", false}, // Handled via t.Setenv with empty value
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue == "" && tt.name == "unset variable" {
				// Unset the variable completely
				t.Setenv("ENABLE_HTTPS", "") // Go 1.17+ automatically unsets when value is empty
				os.Unsetenv("ENABLE_HTTPS")  // Explicit unset for older Go versions
			} else {
				t.Setenv("ENABLE_HTTPS", tt.envValue)
			}

			got := IsHTTPSSEnabled()
			if got != tt.want {
				t.Errorf("IsHTTPSSEnabled() = %v, want %v (env=%q)", got, tt.want, tt.envValue)
			}
		})
	}
}

// TestIsHTTPSSEnabled_NoSideEffects ensures the function doesn't modify environment state.
func TestIsHTTPSSEnabled_NoSideEffects(t *testing.T) {
	const original = "original-value"
	t.Setenv("ENABLE_HTTPS", original)

	_ = IsHTTPSSEnabled()

	if got := os.Getenv("ENABLE_HTTPS"); got != original {
		t.Errorf("environment variable modified: got %q, want %q", got, original)
	}
}

// BenchmarkIsHTTPSSEnabled measures performance of the config check.
func BenchmarkIsHTTPSSEnabled(b *testing.B) {
	benchCases := []struct {
		name  string
		value string
	}{
		{"truthy", "true"},
		{"falsy", "false"},
		{"unset", ""},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			if bc.value != "" {
				b.Setenv("ENABLE_HTTPS", bc.value)
			} else {
				os.Unsetenv("ENABLE_HTTPS")
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = IsHTTPSSEnabled()
			}
		})
	}
}
