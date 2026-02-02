package audit

import (
	"sync"
)

// Factory manages the creation and lifecycle of audit adapters.
// It implements a singleton-like pattern for adapter reuse based on configuration.
type Factory struct {
	config   Config              // Default configuration for adapters
	adapters map[string]*Adapter // Map of adapters keyed by configuration
	mutex    sync.RWMutex        // Mutex for thread-safe operations
}

// GetAdapter retrieves or creates an audit adapter using the factory's default configuration.
// This method reuses existing adapters when possible to conserve resources.
//
// Returns:
//   - *Adapter: Audit adapter instance
//   - error: If adapter creation fails
//
// Adapters are keyed by a combination of FilePath and HTTPEndpoint,
// ensuring unique adapters for different receiver configurations.
func (f *Factory) GetAdapter() (*Adapter, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	// Use key based on configuration
	key := f.config.FilePath + "|" + f.config.HTTPEndpoint
	if adapter, exists := f.adapters[key]; exists {
		return adapter, nil
	}

	// Create new adapter
	adapter, err := NewAdapter(f.config)
	if err != nil {
		return nil, err
	}

	f.adapters[key] = adapter
	return adapter, nil
}

// GetOrCreateAdapter retrieves an existing adapter or creates a new one with the given configuration.
// This method allows creating adapters with different configurations than the factory default.
//
// Parameters:
//   - cfg: Configuration for the adapter to create or retrieve
//
// Returns:
//   - *Adapter: Audit adapter instance
//   - error: If adapter creation fails
//
// The adapter is stored and reused if the same configuration is requested again.
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

// CloseAdapter closes and removes an adapter by its configuration key.
// This releases resources associated with the adapter.
//
// Parameters:
//   - key: Configuration key identifying the adapter to close
//
// Returns:
//   - error: If adapter fails to close properly
//
// The key should be in the format "FilePath|HTTPEndpoint" as used internally.
func (f *Factory) CloseAdapter(key string) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if adapter, exists := f.adapters[key]; exists {
		delete(f.adapters, key)
		return adapter.Close()
	}

	return nil
}

// CloseAll closes all adapters managed by the factory.
// This should be called during application shutdown to release resources.
//
// Returns:
//   - error: The last error encountered during closing, if any
//
// If multiple adapters fail to close, only the last error is returned.
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

// AdapterCount returns the number of adapters currently managed by the factory.
// This is useful for monitoring and debugging.
//
// Returns:
//   - int: Number of active adapters
func (f *Factory) AdapterCount() int {
	f.mutex.RLock()
	defer f.mutex.RUnlock()

	return len(f.adapters)
}
