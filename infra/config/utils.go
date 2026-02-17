package config

import (
	"os"
	"strings"
)

func IsHTTPSSEnabled() bool {
	val := strings.TrimSpace(os.Getenv("ENABLE_HTTPS"))
	val = strings.ToLower(val)

	switch val {
	case "true", "1", "yes", "on":
		return true
	default:
		return false
	}
}
