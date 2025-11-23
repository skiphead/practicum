package repository

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/skiphead/practicum/internal/domain/entity"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/google/uuid"
)

// FileStorage интерфейс для хранения URL
type FileStorage interface {
	Save(userID, correlationID, key, url string) error
	Get(key string) (*URLRecord, bool, error)
	GetByID(id string) (*URLRecord, bool, error)
	FindByOriginalURL(originalURL string) (*URLRecord, error)
	FindByUserID(userID string) ([]URLRecord, error)
	Delete(shortURL string) error
	DeleteByID(id string) error
	SetDeletedByUserIDAndURLs(userID string, shortURLs []string, deleted bool) error
	Stats() map[string]interface{}
	BatchSave(ctx context.Context, in []entity.ShortURL) error // Добавлен в интерфейс
}

// URLRecord представляет структуру хранимых данных URL
type URLRecord struct {
	UUID          string    `json:"uuid"`
	UserID        string    `json:"user_id"`
	CorrelationID string    `json:"correlation_id"`
	ShortURL      string    `json:"short_url"`
	OriginalURL   string    `json:"original_url"`
	CreatedAt     time.Time `json:"created_at"`
	Deleted       bool      `json:"deleted"` // Логическое удаление
}

// CachedFileStorage хранит URL в файле с in-memory кэшем
type cachedFileStorage struct {
	pathDB           string
	mu               sync.RWMutex
	cache            map[string]*URLRecord   // Кэш в памяти: shortURL -> URLRecord
	cacheByID        map[string]*URLRecord   // Кэш в памяти: id -> URLRecord
	originalURLIndex map[string]*URLRecord   // Индекс по оригинальному URL
	userIDIndex      map[string][]*URLRecord // Индекс по user_id
}

// NewCachedFileStorage создает новый экземпляр CachedFileStorage
func NewCachedFileStorage(path string) (FileStorage, error) {
	storage := &cachedFileStorage{
		pathDB:           path,
		cache:            make(map[string]*URLRecord),
		cacheByID:        make(map[string]*URLRecord),
		originalURLIndex: make(map[string]*URLRecord),
		userIDIndex:      make(map[string][]*URLRecord),
	}

	// Восстанавливаем сохраненные данные из файла в кэш при инициализации
	if err := storage.loadCacheFromFile(); err != nil {
		return nil, fmt.Errorf("failed to load cache from file: %w", err)
	}

	return storage, nil
}

// loadCacheFromFile загрузка данных из файла в кэш
func (s *cachedFileStorage) loadCacheFromFile() error {
	file, err := os.OpenFile(s.pathDB, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("error opening file: %w", err)
	}
	defer func(file *os.File) {
		errCloseFile := file.Close()
		if errCloseFile != nil {
			zap.L().Warn("failed to close file", zap.Error(errCloseFile))
		}
	}(file)

	// Проверяем, пуст ли файл
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("error getting file info: %w", err)
	}
	if info.Size() == 0 {
		return nil
	}

	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue // Пропускаем пустые строки
		}

		var record URLRecord
		if err := json.Unmarshal(line, &record); err != nil {
			zap.L().Warn("failed to parse JSON line",
				zap.Int("line", lineNumber),
				zap.Error(err),
				zap.String("content", string(line)))
			continue
		}

		// Пропускаем удаленные записи при загрузке
		if record.Deleted {
			continue
		}

		// Заполняем кэш и индексы
		s.addToCacheAndIndexes(&record)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error scanning file: %w", err)
	}

	return nil
}

// addToCacheAndIndexes добавляет запись во все кэши и индексы
func (s *cachedFileStorage) addToCacheAndIndexes(record *URLRecord) {
	s.cache[record.ShortURL] = record
	s.cacheByID[record.UUID] = record
	s.originalURLIndex[record.OriginalURL] = record

	// Добавляем в индекс по user_id
	if _, exists := s.userIDIndex[record.UserID]; !exists {
		s.userIDIndex[record.UserID] = make([]*URLRecord, 0)
	}
	s.userIDIndex[record.UserID] = append(s.userIDIndex[record.UserID], record)
}

// removeFromIndexes удаляет запись из индексов (но оставляет в основном кэше для логического удаления)
func (s *cachedFileStorage) removeFromIndexes(record *URLRecord) {
	// Удаляем из индекса по оригинальному URL
	delete(s.originalURLIndex, record.OriginalURL)

	// Удаляем из индекса по user_id
	if userRecords, exists := s.userIDIndex[record.UserID]; exists {
		for i, userRecord := range userRecords {
			if userRecord.UUID == record.UUID {
				s.userIDIndex[record.UserID] = append(userRecords[:i], userRecords[i+1:]...)
				break
			}
		}
		// Если у пользователя не осталось записей, удаляем его из индекса
		if len(s.userIDIndex[record.UserID]) == 0 {
			delete(s.userIDIndex, record.UserID)
		}
	}
}

// Save сохраняет URL-маппинг в файл и обновляет кэш
func (s *cachedFileStorage) Save(userID, correlationID, key, url string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Проверяем, не существует ли уже такой оригинальный URL
	if existing, exists := s.originalURLIndex[url]; exists {
		return fmt.Errorf("URL already exists: %s", existing.ShortURL)
	}

	// Генерируем UUID v4
	id := uuid.New().String()

	record := &URLRecord{
		UserID:        userID,
		UUID:          id,
		CorrelationID: correlationID,
		ShortURL:      key,
		OriginalURL:   url,
		CreatedAt:     time.Now(),
		Deleted:       false,
	}

	// Обновляем кэш и индексы
	s.addToCacheAndIndexes(record)

	// Сохраняем в файл в формате JSONL
	return s.appendRecordToFile(record)
}

// BatchSave сохраняет URL-маппинг с ID в файл и обновляет кэш
func (s *cachedFileStorage) BatchSave(ctx context.Context, in []entity.ShortURL) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		for _, i := range in {
			// Проверяем, не существует ли уже такой оригинальный URL
			if existing, exists := s.originalURLIndex[i.OriginalURL]; exists {
				zap.L().Warn("URL already exists, skipping",
					zap.String("original_url", i.OriginalURL),
					zap.String("existing_short", existing.ShortURL))
				continue
			}

			// Генерируем UUID v4
			id := uuid.New().String()
			record := &URLRecord{
				UUID:          id,
				UserID:        i.UserID,
				CorrelationID: i.CorrelationID,
				ShortURL:      i.ShortCode,
				OriginalURL:   i.OriginalURL,
				CreatedAt:     time.Now(),
				Deleted:       false,
			}

			// Обновляем кэш и индексы
			s.addToCacheAndIndexes(record)

			err := s.appendRecordToFile(record)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// appendRecordToFile добавляет запись в конец файла в формате JSONL
func (s *cachedFileStorage) appendRecordToFile(record *URLRecord) error {
	file, err := os.OpenFile(s.pathDB, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("error opening file: %w", err)
	}
	defer func(file *os.File) {
		errCloseFile := file.Close()
		if errCloseFile != nil {
			zap.L().Warn("failed to close file", zap.Error(errCloseFile))
		}
	}(file)

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("error marshaling record: %w", err)
	}

	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("error writing to file: %w", err)
	}

	return nil
}

// Get получает URL по ключу из кэша
func (s *cachedFileStorage) Get(key string) (*URLRecord, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, exists := s.cache[key]
	if !exists || record.Deleted {
		return nil, false, nil
	}
	return record, true, nil
}

// GetByID получает запись по UUID из кэша
func (s *cachedFileStorage) GetByID(id string) (*URLRecord, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, exists := s.cacheByID[id]
	if !exists || record.Deleted {
		return nil, false, nil
	}
	return record, true, nil
}

// Delete помечает запись как удаленную (логическое удаление)
func (s *cachedFileStorage) Delete(shortURL string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, exists := s.cache[shortURL]
	if !exists || record.Deleted {
		return fmt.Errorf("record not found with short URL: %s", shortURL)
	}

	// Логическое удаление
	record.Deleted = true
	s.removeFromIndexes(record)

	// Записываем изменение в файл (append-only)
	return s.appendRecordToFile(record)
}

// DeleteByID помечает запись как удаленную по UUID (логическое удаление)
func (s *cachedFileStorage) DeleteByID(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, exists := s.cacheByID[id]
	if !exists || record.Deleted {
		return fmt.Errorf("record not found with id: %s", id)
	}

	// Логическое удаление
	record.Deleted = true
	s.removeFromIndexes(record)

	// Записываем изменение в файл (append-only)
	return s.appendRecordToFile(record)
}

// FindByOriginalURL ищет запись по оригинальному URL в индексе
func (s *cachedFileStorage) FindByOriginalURL(originalURL string) (*URLRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, exists := s.originalURLIndex[originalURL]
	if !exists {
		return nil, nil // Возвращаем nil вместо ошибки
	}
	return record, nil
}

// FindByUserID ищет все записи по user_id в индексе
func (s *cachedFileStorage) FindByUserID(userID string) ([]URLRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	records, exists := s.userIDIndex[userID]
	if !exists {
		return []URLRecord{}, nil // Возвращаем пустой срез вместо ошибки
	}

	// Создаем копии для безопасного возврата
	result := make([]URLRecord, len(records))
	for i, record := range records {
		result[i] = *record
	}

	return result, nil
}

// SetDeletedByUserIDAndURLs устанавливает флаг deleted для конкретных URL пользователя
func (s *cachedFileStorage) SetDeletedByUserIDAndURLs(userID string, shortURLs []string, deleted bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var updatedRecords []*URLRecord

	// Получаем все записи пользователя
	userRecords, exists := s.userIDIndex[userID]
	if !exists {
		return fmt.Errorf("no records found for user: %s", userID)
	}

	// Создаем карту для быстрого поиска
	urlMap := make(map[string]*URLRecord)
	for _, record := range userRecords {
		urlMap[record.ShortURL] = record
	}

	// Обновляем только указанные URL
	for _, shortURL := range shortURLs {
		record, e := urlMap[shortURL]
		if !e {
			continue // Пропускаем несуществующие URL
		}

		// Если флаг уже установлен в нужное значение, пропускаем
		if record.Deleted == deleted {
			continue
		}

		// Устанавливаем флаг
		record.Deleted = deleted

		// Обновляем индексы
		if deleted {
			s.removeFromIndexes(record)
		} else {
			s.addToCacheAndIndexes(record)
		}

		updatedRecords = append(updatedRecords, record)
	}

	// Записываем изменения в файл
	for _, record := range updatedRecords {
		if err := s.appendRecordToFile(record); err != nil {
			return fmt.Errorf("failed to update record in file: %w", err)
		}
	}

	return nil
}

// CompactFile выполняет сжатие файла, удаляя логически удаленные записи
func (s *cachedFileStorage) CompactFile() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tempPath := s.pathDB + ".tmp"
	tempFile, err := os.OpenFile(tempPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("error creating temp file: %w", err)
	}
	defer func() {
		tempFile.Close()
		os.Remove(tempPath) // Удаляем временный файл в случае ошибки
	}()

	writer := bufio.NewWriter(tempFile)

	// Записываем только активные записи
	for _, record := range s.cache {
		if !record.Deleted {
			data, err := json.Marshal(record)
			if err != nil {
				return fmt.Errorf("error marshaling record: %w", err)
			}
			if _, err := writer.Write(append(data, '\n')); err != nil {
				return fmt.Errorf("error writing to temp file: %w", err)
			}
		}
	}

	if err := writer.Flush(); err != nil {
		return fmt.Errorf("error flushing temp file: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("error closing temp file: %w", err)
	}

	// Заменяем оригинальный файл сжатым
	if err := os.Rename(tempPath, s.pathDB); err != nil {
		return fmt.Errorf("error replacing original file: %w", err)
	}

	return nil
}

// Stats возвращает статистику хранилища
func (s *cachedFileStorage) Stats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	fileInfo, err := os.Stat(s.pathDB)
	fileSize := int64(0)
	if err == nil && fileInfo != nil {
		fileSize = fileInfo.Size()
	}

	// Считаем только активные записи
	activeRecords := 0
	for _, record := range s.cache {
		if !record.Deleted {
			activeRecords++
		}
	}

	return map[string]interface{}{
		"total_records":   len(s.cache),
		"active_records":  activeRecords,
		"deleted_records": len(s.cache) - activeRecords,
		"file_path":       s.pathDB,
		"file_size_bytes": fileSize,
		"unique_users":    len(s.userIDIndex),
	}
}
