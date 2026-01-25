package utils

import (
	"crypto/rand"
	"math/big"
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
	if u.Scheme != "httpclient" && u.Scheme != "https" {
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
	charsetLength := big.NewInt(int64(len(randomCharset)))

	for i := range buf {
		// Генерируем криптографически безопасное случайное число в диапазоне [0, len(charset))
		randIndex, err := rand.Int(rand.Reader, charsetLength)
		if err != nil {
			panic(err) // Обработка ошибки генерации
		}
		buf[i] = randomCharset[randIndex.Int64()]
	}
	return string(buf)
}
