// Package store defines the persistence layer for VexPay.
//
// The concrete backend (SQLite by default, Postgres optional) is selected at
// runtime from the DATABASE_URL. Callers depend only on the interfaces here, so
// swapping backends never touches business logic.
package store

import (
	"context"
	"fmt"
	"strings"
)

// Store is the top-level persistence handle. Repositories for invoices,
// merchants, webhooks, etc. hang off this as the codebase grows.
type Store interface {
	// Ping verifies the backend is reachable.
	Ping(ctx context.Context) error
	// Close releases any resources held by the backend.
	Close() error
}

// Open selects and initialises a Store from a database URL. Supported forms:
//
//	sqlite:<path>       e.g. sqlite:vexpay.db
//	postgres://...      standard libpq DSN
//
// Phase 0 ships an in-memory backend for the "memory:" scheme and treats
// "sqlite:" as memory-backed until the driver lands in Phase 1.
func Open(databaseURL string) (Store, error) {
	scheme, rest, ok := strings.Cut(databaseURL, ":")
	if !ok {
		return nil, fmt.Errorf("invalid database url %q: missing scheme", databaseURL)
	}
	switch scheme {
	case "memory":
		return newMemory(), nil
	case "sqlite":
		// TODO(phase1): back this with modernc.org/sqlite at path `rest`.
		_ = rest
		return newMemory(), nil
	case "postgres", "postgresql":
		return nil, fmt.Errorf("postgres backend not implemented yet")
	default:
		return nil, fmt.Errorf("unsupported database scheme %q", scheme)
	}
}
