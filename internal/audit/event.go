package audit

// Event представляет событие аудита
type Event struct {
	Timestamp int64  `json:"ts"`      // Unix timestamp события
	Action    string `json:"action"`  // действие: shorten или follow
	UserID    string `json:"user_id"` // идентификатор пользователя, если есть
	URL       string `json:"url"`     // оригинальный URL
}
