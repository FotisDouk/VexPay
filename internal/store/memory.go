package store

import (
	"context"
	"sync"

	"github.com/vexarnetwork/vexpay/internal/invoice"
)

// memory is an in-process Store backend used for tests and the sandbox. It is
// safe for concurrent use.
type memory struct {
	mu     sync.RWMutex
	closed bool
	repo   *invoice.MemRepository
}

func newMemory() *memory { return &memory{repo: invoice.NewMemRepository()} }

func (m *memory) Invoices() invoice.Repository { return m.repo }

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
