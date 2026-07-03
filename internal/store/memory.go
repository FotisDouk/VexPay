package store

import (
	"context"
	"sync"
)

// memory is an in-process Store backend. It is used for tests, the sandbox, and
// as the Phase 0 placeholder until the SQLite driver is wired in. It is safe for
// concurrent use.
type memory struct {
	mu     sync.RWMutex
	closed bool
}

func newMemory() *memory { return &memory{} }

func (m *memory) Ping(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return context.Canceled
	}
	return nil
}

func (m *memory) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}
