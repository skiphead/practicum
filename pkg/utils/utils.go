package utils

import (
	"math/rand"
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

const (
	keyLength     = 8
	randomCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

func GenerateRandomKey() string {
	buf := make([]byte, keyLength)
	for i := range buf {
		buf[i] = randomCharset[rand.Intn(len(randomCharset))]
	}
	return string(buf)
}
