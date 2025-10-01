package repository

import "sync"

type Storage interface {
	Save(key, url string)
	Get(key string) (string, bool)
}

type MemoryStorage struct {
	links map[string]string
	mu    sync.RWMutex
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		links: make(map[string]string),
	}
}

func (s *MemoryStorage) Save(key, url string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.links[key] = url
}

func (s *MemoryStorage) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	url, exists := s.links[key]
	return url, exists
}
