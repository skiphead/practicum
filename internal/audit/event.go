package audit

import "time"

// Event представляет событие аудита
type Event struct {
	Timestamp int64  `json:"ts"`      // Unix timestamp события
	Action    string `json:"action"`  // действие: shorten или follow
	UserID    string `json:"user_id"` // идентификатор пользователя, если есть
	URL       string `json:"url"`     // оригинальный URL
}

// NewEvent создает новое событие аудита
func NewEvent(action, userID, url string) *Event {
	return &Event{
		Timestamp: time.Now().Unix(),
		Action:    action,
		UserID:    userID,
		URL:       url,
	}
}

// NewShortenEvent создает событие создания короткой ссылки
func NewShortenEvent(userID, originalURL string) *Event {
	return NewEvent("shorten", userID, originalURL)
}

// NewFollowEvent создает событие перехода по короткой ссылке
func NewFollowEvent(userID, originalURL string) *Event {
	return NewEvent("follow", userID, originalURL)
}
