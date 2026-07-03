package chain

import (
	"fmt"
	"sort"
	"sync"
)

// Registry holds the adapters available to a running gateway, keyed by chain ID.
type Registry struct {
	mu       sync.RWMutex
	adapters map[ID]Adapter
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{adapters: make(map[ID]Adapter)}
}

// Register adds an adapter. It returns an error if the chain is already
// registered.
func (r *Registry) Register(a Adapter) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := a.Chain()
	if _, exists := r.adapters[id]; exists {
		return fmt.Errorf("chain %q already registered", id)
	}
	r.adapters[id] = a
	return nil
}

// Get returns the adapter for a chain ID.
func (r *Registry) Get(id ID) (Adapter, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.adapters[id]
	if !ok {
		return nil, fmt.Errorf("no adapter registered for chain %q", id)
	}
	return a, nil
}

// Chains lists the registered chain IDs in sorted order.
func (r *Registry) Chains() []ID {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]ID, 0, len(r.adapters))
	for id := range r.adapters {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}
