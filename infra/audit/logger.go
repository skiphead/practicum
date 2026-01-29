package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogAction represents the type of audit action.
type LogAction string

const (
	ActionShorten LogAction = "shorten" // URL shortening action
	ActionFollow  LogAction = "follow"  // URL following/redirection action
)

// LogEntry represents a single audit log entry.
// Entries are written in JSON Lines format for easy processing.
type LogEntry struct {
	TS     int64     `json:"ts"`      // Unix timestamp of the event
	Action LogAction `json:"action"`  // Action type: shorten or follow
	UserID string    `json:"user_id"` // User identifier (may be empty for anonymous actions)
	URL    string    `json:"url"`     // Original URL involved in the action
}

// Logger provides thread-safe audit logging to a JSON Lines file.
// It implements a singleton pattern per file path to prevent multiple writers.
type Logger struct {
	file     *os.File      // File handle for the audit log
	encoder  *json.Encoder // JSON encoder for writing log entries
	mutex    sync.Mutex    // Mutex for thread-safe file operations
	filePath string        // Path to the audit log file
}

// instances stores singleton logger instances keyed by file path.
var (
	instances = make(map[string]*Logger)
	mu        sync.RWMutex // Mutex for thread-safe instance management
)

// GetInstance returns a singleton logger instance for a specific file path.
// If an instance for the given path doesn't exist, it creates one.
//
// Parameters:
//   - filePath: Path to the audit log file
//
// Returns:
//   - *Logger: Logger instance for the specified path
//   - error: If file cannot be opened or directory cannot be created
//
// The method ensures only one logger instance writes to each file path
// to prevent file corruption from concurrent writes.
func GetInstance(filePath string) (*Logger, error) {
	mu.Lock()
	defer mu.Unlock()

	if instance, exists := instances[filePath]; exists {
		return instance, nil
	}

	// Create directory if needed
	dir := filepath.Dir(filePath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
	}

	// Open file for appending log entries
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

// CloseInstance closes a specific logger instance and removes it from the registry.
// Primarily used for testing cleanup.
//
// Parameters:
//   - filePath: Path of the logger instance to close
//
// Returns:
//   - error: If file cannot be closed (nil if instance doesn't exist)
func CloseInstance(filePath string) error {
	mu.Lock()
	defer mu.Unlock()

	if instance, exists := instances[filePath]; exists {
		delete(instances, filePath)
		return instance.file.Close()
	}
	return nil
}

// ResetInstances closes all logger instances and clears the registry.
// Primarily used for testing cleanup.
func ResetInstances() {
	mu.Lock()
	defer mu.Unlock()

	for filePath, instance := range instances {
		instance.file.Close()
		delete(instances, filePath)
	}
}

// Log writes an audit log entry to the file in JSON format.
// The method is thread-safe and ensures atomic writes.
//
// Parameters:
//   - action: Type of audit action
//   - userID: User identifier (empty string for anonymous)
//   - url: URL involved in the action
//
// Returns:
//   - error: If JSON encoding or file write fails
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

// LogShorten logs a URL shortening action.
// Convenience method for common shortening audit events.
//
// Parameters:
//   - userID: User who created the short URL
//   - url: Original URL that was shortened
//
// Returns:
//   - error: If log write fails
func (l *Logger) LogShorten(userID, url string) error {
	return l.Log(ActionShorten, userID, url)
}

// LogFollow logs a URL following/redirection action.
// Convenience method for common redirection audit events.
//
// Parameters:
//   - userID: User who followed the short URL (empty if anonymous)
//   - url: Original URL that was accessed
//
// Returns:
//   - error: If log write fails
func (l *Logger) LogFollow(userID, url string) error {
	return l.Log(ActionFollow, userID, url)
}

// Close closes the audit log file.
// This should be called during application shutdown to ensure all data is flushed.
//
// Returns:
//   - error: If file cannot be closed
func (l *Logger) Close() error {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// Reopen closes and reopens the log file.
// Useful for log rotation or when files need to be moved/renamed.
//
// Returns:
//   - error: If file cannot be reopened
//
// Note: This method is thread-safe and ensures no log entries are lost
// during the reopening process.
func (l *Logger) Reopen() error {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	// Close old file
	if l.file != nil {
		l.file.Close()
	}

	// Reopen file
	file, err := os.OpenFile(l.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	l.file = file
	l.encoder = json.NewEncoder(file)
	return nil
}
