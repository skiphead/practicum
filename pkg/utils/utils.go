// Package utils provides utility functions for common operations such as URL validation
// and secure random key generation.
package utils

import (
	"crypto/rand"
	"math/big"
	"net/url"
	"strings"
)

// IsValidURL validates whether a given string is a valid HTTP or HTTPS URL.
// It performs the following checks:
//   - The string must not contain spaces
//   - Must be parsable as a URL
//   - Must contain both scheme and host
//   - Scheme must be either "http" or "https"
//
// Returns true if all checks pass, false otherwise.
//
// Example:
//   - "https://example.com" returns true
//   - "http://localhost:8080" returns true
//   - "ftp://files.com" returns false (invalid scheme)
//   - "examples_test.com" returns false (missing scheme)
//   - "https://" returns false (missing host)
func IsValidURL(s string) bool {
	// Check for spaces in the URL string
	if strings.ContainsAny(s, " ") {
		return false
	}

	u, err := url.Parse(s)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}

	// Check for valid schemes
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}

	return true
}

const (
	keyLength     = 8
	randomCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

// GenerateRandomKey generates a cryptographically secure random string of fixed length.
// The key consists of alphanumeric characters (both uppercase and lowercase).
// The length of the key is defined by keyLength constant (8 characters).
//
// This function uses crypto/rand for secure random generation, making it suitable
// for generating secret keys, tokens, or identifiers.
//
// Panics if the random number generator fails (which typically indicates a system problem).
func GenerateRandomKey() string {
	buf := make([]byte, keyLength)
	charsetLength := big.NewInt(int64(len(randomCharset)))

	for i := range buf {
		// Generate cryptographically secure random number in range [0, len(charset))
		randIndex, err := rand.Int(rand.Reader, charsetLength)
		if err != nil {
			panic(err) // Handle random generation error
		}
		buf[i] = randomCharset[randIndex.Int64()]
	}
	return string(buf)
}

func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
