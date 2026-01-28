package domain

// AuditEvent - событие аудита
type AuditEvent struct {
	Timestamp int64  `json:"ts"`          // unix timestamp события
	Action    string `json:"action"`      // действие
	UserID    string `json:"user_id"`     // идентификатор пользователя, если есть
	URL       string `json:"url"`         // URL
	Method    string `json:"method"`      // HTTP метод
	Path      string `json:"path"`        // Путь запроса
	Status    int    `json:"status"`      // HTTP статус
	ClientIP  string `json:"client_ip"`   // IP клиента
	UserAgent string `json:"user_agent"`  // User Agent
	Duration  int64  `json:"duration_ms"` // Длительность обработки в мс
	BytesSent int64  `json:"bytes_sent"`  // Отправлено байт
}

// Subscriber - интерфейс подписчика
type Subscriber interface {
	Update(event AuditEvent)
	ID() string
	Type() string
}

// Publisher - интерфейс издателя
type Publisher interface {
	Subscribe(subscriber Subscriber)
	Unsubscribe(subscriberID string)
	Notify(event AuditEvent)
	GetSubscribers() []Subscriber
}
