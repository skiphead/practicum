package utils

import (
	"net/url"
	"strings"
)

func IsValidURL(s string) bool {
	// Проверка на пробелы в строке URL
	if strings.ContainsAny(s, " ") {
		return false
	}

	u, err := url.Parse(s)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}

	// Проверка допустимых схем
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}

	return true
}
