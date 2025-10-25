package storage

import (
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Storage interface {
	Save(key, url string) error
	Get(key string) (*URLRecord, bool, error)
	GetByID(id string) (*URLRecord, error)
	FindByOriginalURL(originalURL string) (*URLRecord, error)
	Delete(shortURL string) error
	DeleteByID(id string) error
	Stats() map[string]interface{}
}

// URLRecord представляет структуру хранимых данных URL
type URLRecord struct {
	UUID        string    `json:"uuid"`
	ShortURL    string    `json:"short_url"`
	OriginalURL string    `json:"original_url"`
	CreatedAt   time.Time `json:"created_at"`
}

// CachedFileStorage хранит URL в файле с in-memory кэшем
type cachedFileStorage struct {
	pathDB    string
	mu        sync.RWMutex
	cache     map[string]*URLRecord // Кэш в памяти: shortURL -> URLRecord
	cacheByID map[string]*URLRecord // Кэш в памяти: id -> URLRecord
}

// NewCachedFileStorage создает новый экземпляр CachedFileStorage
func NewCachedFileStorage(path string) (Storage, error) {
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
	records, err := s.readAllRecordsFromFile()
	if err != nil {
		return err
	}

	// Заполняем кэш
	for i := range records {
		record := &records[i]
		s.cache[record.ShortURL] = record
		s.cacheByID[record.UUID] = record
	}

	return nil
}

// Save сохраняет URL-маппинг в файл и обновляет кэш
func (s *cachedFileStorage) Save(key, url string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Генерируем UUID v4
	id := uuid.New().String()

	record := &URLRecord{
		UUID:        id,
		ShortURL:    key,
		OriginalURL: url,
		CreatedAt:   time.Now(),
	}

	// Обновляем кэш
	s.cache[key] = record
	s.cacheByID[id] = record

	// Сохраняем в файл
	return s.saveRecordToFile(record)
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

// saveRecordToFile сохраняет/обновляет запись в файле
func (s *cachedFileStorage) saveRecordToFile(record *URLRecord) error {
	// Читаем все записи из файла
	records, err := s.readAllRecordsFromFile()
	if err != nil {
		return err
	}

	// Проверяем, существует ли уже такой shortURL
	found := false
	for i, r := range records {
		if r.ShortURL == record.ShortURL {
			// Обновляем существующую запись
			records[i] = *record
			found = true
			break
		}
	}

	// Если не нашли, добавляем новую запись
	if !found {
		records = append(records, *record)
	}

	// Записываем обратно в файл
	return s.writeAllRecordsToFile(records)
}

// readAllRecordsFromFile читает все записи из файла
func (s *cachedFileStorage) readAllRecordsFromFile() ([]URLRecord, error) {
	file, err := os.OpenFile(s.pathDB, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}
	defer func(file *os.File) {
		errCloseFile := file.Close()
		if errCloseFile != nil {
			zap.L().Warn("failed to close file", zap.Error(errCloseFile))
		}
	}(file)

	// Проверяем, пуст ли файл
	info, errStat := file.Stat()
	if errStat != nil {
		return nil, fmt.Errorf("error getting file info: %w", errStat)
	}
	if info.Size() == 0 {
		return []URLRecord{}, nil
	}

	var records []URLRecord
	decoder := json.NewDecoder(file)
	if errDecode := decoder.Decode(&records); errDecode != nil {
		return nil, fmt.Errorf("error decoding JSON: %w", errDecode)
	}

	return records, nil
}

// writeAllRecordsToFile записывает все записи в файл
func (s *cachedFileStorage) writeAllRecordsToFile(records []URLRecord) error {
	file, err := os.OpenFile(s.pathDB, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("error opening file: %w", err)
	}
	defer func(file *os.File) {
		errCloseFile := file.Close()
		if errCloseFile != nil {
			zap.L().Warn("failed to close file", zap.Error(errCloseFile))
		}
	}(file)

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if errEncode := encoder.Encode(records); errEncode != nil {
		return fmt.Errorf("error encoding records: %w", errEncode)
	}

	return nil
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
	return s.rewriteFileWithoutRecord(shortURL)
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
	return s.rewriteFileWithoutRecord(record.ShortURL)
}

// rewriteFileWithoutRecord перезаписывает файл без указанной записи
func (s *cachedFileStorage) rewriteFileWithoutRecord(shortURL string) error {
	records, err := s.readAllRecordsFromFile()
	if err != nil {
		return fmt.Errorf("error reading from file: %w", err)
	}

	// Создаем новый срез без удаляемой записи
	var newRecords []URLRecord
	for _, record := range records {
		if record.ShortURL != shortURL {
			newRecords = append(newRecords, record)
		}
	}

	return s.writeAllRecordsToFile(newRecords)
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
