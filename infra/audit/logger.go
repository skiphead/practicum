package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogAction - тип действия
type LogAction string

const (
	ActionShorten LogAction = "shorten"
	ActionFollow  LogAction = "follow"
)

// LogEntry - структура записи лога
type LogEntry struct {
	TS     int64     `json:"ts"`      // Unix timestamp события
	Action LogAction `json:"action"`  // Действие: shorten или follow
	UserID string    `json:"user_id"` // Идентификатор пользователя (может быть пустым)
	URL    string    `json:"url"`     // Оригинальный URL
}

// Logger - логгер
type Logger struct {
	file     *os.File
	encoder  *json.Encoder
	mutex    sync.Mutex
	filePath string
}

// instanceMap хранит экземпляры для разных путей файлов
var (
	instances = make(map[string]*Logger)
	mu        sync.RWMutex
)

// GetInstance - возвращает экземпляр синглтона логгера для конкретного пути
func GetInstance(filePath string) (*Logger, error) {
	mu.Lock()
	defer mu.Unlock()

	if instance, exists := instances[filePath]; exists {
		return instance, nil
	}

	// Создаем директорию, если нужно
	dir := filepath.Dir(filePath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
	}

	// Открываем файл для добавления записей
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	instance := &Logger{
		file:     file,
		encoder:  json.NewEncoder(file),
		filePath: filePath,
	}

	instances[filePath] = instance
	return instance, nil
}

// CloseInstance - закрывает экземпляр логгера для конкретного пути (для тестов)
func CloseInstance(filePath string) error {
	mu.Lock()
	defer mu.Unlock()

	if instance, exists := instances[filePath]; exists {
		delete(instances, filePath)
		return instance.file.Close()
	}
	return nil
}

// ResetInstances - удаляет все экземпляры (для тестов)
func ResetInstances() {
	mu.Lock()
	defer mu.Unlock()

	for filePath, instance := range instances {
		instance.file.Close()
		delete(instances, filePath)
	}
}

// Log - записывает запись в лог
func (l *Logger) Log(action LogAction, userID, url string) error {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	entry := LogEntry{
		TS:     time.Now().Unix(),
		Action: action,
		UserID: userID,
		URL:    url,
	}

	return l.encoder.Encode(entry)
}

// LogShorten - логирует создание короткой ссылки
func (l *Logger) LogShorten(userID, url string) error {
	return l.Log(ActionShorten, userID, url)
}

// LogFollow - логирует переход по короткой ссылке
func (l *Logger) LogFollow(userID, url string) error {
	return l.Log(ActionFollow, userID, url)
}

// Close - закрывает файл лога
func (l *Logger) Close() error {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// Reopen - переоткрывает файл (полезно при ротации логов)
func (l *Logger) Reopen() error {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	// Закрываем старый файл
	if l.file != nil {
		l.file.Close()
	}

	// Открываем файл заново
	file, err := os.OpenFile(l.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	l.file = file
	l.encoder = json.NewEncoder(file)
	return nil
}
