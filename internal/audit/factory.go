package audit

import (
	"sync"
)

// Factory создает и управляет адаптерами аудита
type Factory struct {
	config   Config
	adapters map[string]*Adapter
	mutex    sync.RWMutex
}

// GetAdapter получает или создает адаптер
func (f *Factory) GetAdapter() (*Adapter, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	// Используем ключ на основе конфигурации
	key := f.config.FilePath + "|" + f.config.HTTPEndpoint
	if adapter, exists := f.adapters[key]; exists {
		return adapter, nil
	}

	// Создаем новый адаптер
	adapter, err := NewAdapter(f.config)
	if err != nil {
		return nil, err
	}

	f.adapters[key] = adapter
	return adapter, nil
}

// GetOrCreateAdapter получает существующий или создает новый адаптер
func (f *Factory) GetOrCreateAdapter(cfg Config) (*Adapter, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	key := cfg.FilePath + "|" + cfg.HTTPEndpoint
	if adapter, exists := f.adapters[key]; exists {
		return adapter, nil
	}

	adapter, err := NewAdapter(cfg)
	if err != nil {
		return nil, err
	}

	f.adapters[key] = adapter
	return adapter, nil
}

// CloseAdapter закрывает адаптер по ключу
func (f *Factory) CloseAdapter(key string) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if adapter, exists := f.adapters[key]; exists {
		delete(f.adapters, key)
		return adapter.Close()
	}

	return nil
}

// CloseAll закрывает все адаптеры
func (f *Factory) CloseAll() error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	var lastErr error
	for key, adapter := range f.adapters {
		if err := adapter.Close(); err != nil {
			lastErr = err
		}
		delete(f.adapters, key)
	}

	return lastErr
}

// AdapterCount возвращает количество адаптеров
func (f *Factory) AdapterCount() int {
	f.mutex.RLock()
	defer f.mutex.RUnlock()

	return len(f.adapters)
}
