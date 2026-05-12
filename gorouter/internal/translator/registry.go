package translator

import (
	"sync"
)

type registry struct {
	adapters map[string]ProviderAdapter
	mu       sync.RWMutex
}

var defaultRegistry = &registry{
	adapters: make(map[string]ProviderAdapter),
}

func Register(adapter ProviderAdapter) {
	defaultRegistry.mu.Lock()
	defer defaultRegistry.mu.Unlock()
	defaultRegistry.adapters[adapter.Provider()] = adapter
}

func Get(provider string) (ProviderAdapter, error) {
	defaultRegistry.mu.RLock()
	defer defaultRegistry.mu.RUnlock()

	adapter, ok := defaultRegistry.adapters[provider]
	if !ok {
		return nil, ErrAdapterNotFound{Provider: provider}
	}
	return adapter, nil
}

func List() []ProviderAdapter {
	defaultRegistry.mu.RLock()
	defer defaultRegistry.mu.RUnlock()

	adapters := make([]ProviderAdapter, 0, len(defaultRegistry.adapters))
	for _, a := range defaultRegistry.adapters {
		adapters = append(adapters, a)
	}
	return adapters
}

type ErrAdapterNotFound struct {
	Provider string
}

func (e ErrAdapterNotFound) Error() string {
	return "adapter not found for provider: " + e.Provider
}