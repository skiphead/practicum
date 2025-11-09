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

type FileStorage interface {
	Save(correlationID, key, url string) error
	Get(key string) (*URLRecord, bool, error)
	GetByID(id string) (*URLRecord, error)
	FindByOriginalURL(originalURL string) (*URLRecord, error)
	Delete(shortURL string) error
	DeleteByID(id string) error
	Stats() map[string]interface{}
}

// URLRecord представляет структуру хранимых данных URL
type URLRecord struct {
	UUID          string    `json:"uuid"`
	CorrelationId string    `json:"correlation_id"`
	ShortURL      string    `json:"short_url"`
	OriginalURL   string    `json:"original_url"`
	CreatedAt     time.Time `json:"created_at"`
}

// CachedFileStorage хранит URL в файле с in-memory кэшем
type cachedFileStorage struct {
	pathDB    string
	mu        sync.RWMutex
	cache     map[string]*URLRecord // Кэш в памяти: shortURL -> URLRecord
	cacheByID map[string]*URLRecord // Кэш в памяти: id -> URLRecord
}

// NewCachedFileStorage создает новый экземпляр CachedFileStorage
func NewCachedFileStorage(path string) (FileStorage, error) {
	storage := &cachedFileStorage{
		pathDB:    path,
		cache:     make(map[string]*URLRecord),
		cacheByID: make(map[string]*URLRecord),
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

		// Заполняем кэш
		s.cache[record.ShortURL] = &record
		s.cacheByID[record.UUID] = &record
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error scanning file: %w", err)
	}

	return nil
}

// Save сохраняет URL-маппинг в файл и обновляет кэш
func (s *cachedFileStorage) Save(correlationID, key, url string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Генерируем UUID v4
	id := uuid.New().String()

	record := &URLRecord{
		UUID:          id,
		CorrelationId: correlationID,
		ShortURL:      key,
		OriginalURL:   url,
		CreatedAt:     time.Now(),
	}

	// Обновляем кэш
	s.cache[key] = record
	s.cacheByID[id] = record

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
			// Генерируем UUID v4
			id := uuid.New().String()
			record := &URLRecord{
				UUID:          id,
				CorrelationId: i.CorrelationID,
				ShortURL:      i.ShortCode,
				OriginalURL:   i.OriginalURL,
				CreatedAt:     time.Now(),
			}

			// Обновляем кэш
			s.cache[i.ShortCode] = record
			s.cacheByID[id] = record
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
	if !exists {
		return nil, false, nil
	}
	return record, true, nil
}

// GetByID получает запись по UUID из кэша
func (s *cachedFileStorage) GetByID(id string) (*URLRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, exists := s.cacheByID[id]
	if !exists {
		return nil, fmt.Errorf("record not found with id: %s", id)
	}
	return record, nil
}

// Delete удаляет запись по shortURL из кэша и файла
func (s *cachedFileStorage) Delete(shortURL string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, exists := s.cache[shortURL]
	if !exists {
		return fmt.Errorf("record not found with short URL: %s", shortURL)
	}

	// Удаляем из кэша
	delete(s.cache, shortURL)
	delete(s.cacheByID, record.UUID)

	// Обновляем файл
	return s.rewriteFileWithoutDeletedRecords()
}

// DeleteByID удаляет запись по UUID из кэша и файла
func (s *cachedFileStorage) DeleteByID(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, exists := s.cacheByID[id]
	if !exists {
		return fmt.Errorf("record not found with id: %s", id)
	}

	// Удаляем из кэша
	delete(s.cache, record.ShortURL)
	delete(s.cacheByID, id)

	// Обновляем файл
	return s.rewriteFileWithoutDeletedRecords()
}

// rewriteFileWithoutDeletedRecords перезаписывает файл только с активными записями
func (s *cachedFileStorage) rewriteFileWithoutDeletedRecords() error {
	file, err := os.OpenFile(s.pathDB, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("error opening file: %w", err)
	}
	defer func(file *os.File) {
		errCloseFile := file.Close()
		if errCloseFile != nil {
			zap.L().Warn("failed to close file", zap.Error(errCloseFile))
		}
	}(file)

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	// Записываем все активные записи из кэша
	for _, record := range s.cache {
		data, err := json.Marshal(record)
		if err != nil {
			return fmt.Errorf("error marshaling record: %w", err)
		}
		if _, err := writer.Write(append(data, '\n')); err != nil {
			return fmt.Errorf("error writing to file: %w", err)
		}
	}

	return nil
}

// FindByOriginalURL ищет запись по оригинальному URL в кэше
func (s *cachedFileStorage) FindByOriginalURL(originalURL string) (*URLRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, record := range s.cache {
		if record.OriginalURL == originalURL {
			return record, nil
		}
	}

	return nil, fmt.Errorf("record not found with original URL: %s", originalURL)
}

// Stats возвращает статистику хранилища
func (s *cachedFileStorage) Stats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	fileInfo, _ := os.Stat(s.pathDB)
	fileSize := int64(0)
	if fileInfo != nil {
		fileSize = fileInfo.Size()
	}

	return map[string]interface{}{
		"total_records":   len(s.cache),
		"file_path":       s.pathDB,
		"file_size_bytes": fileSize,
	}
}
