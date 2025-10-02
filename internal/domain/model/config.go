package model

import (
	"net/http"
	"sync"
)

// Config конфигурация сервера.
type Config struct {
	serverAddr string            // Адрес сервера в формате host:port
	router     *http.ServeMux    // HTTP роутер для обработки запросов
	links      map[string]string // Маппинг коротких ключей на оригинальные URL
	mu         sync.RWMutex      // Мьютекс для обеспечения конкурентного доступа к хранилищу ссылок
}
