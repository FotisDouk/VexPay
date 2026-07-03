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

	"github.com/vexarnetwork/vexpay/internal/invoice"
)

// Store is the top-level persistence handle. Repositories for invoices,
// merchants, webhooks, etc. hang off this as the codebase grows.
type Store interface {
	// Ping verifies the backend is reachable.
	Ping(ctx context.Context) error
	// Invoices returns the invoice repository backed by this store.
	Invoices() invoice.Repository
	// Close releases any resources held by the backend.
	Close() error
}

// Open selects and initialises a Store from a database URL. Supported forms:
//
//	memory:             in-process, non-persistent (tests, sandbox)
//	sqlite:<path>       file-backed SQLite (default); use ":memory:" for RAM
//	postgres://...      standard libpq DSN (not yet implemented)
func Open(databaseURL string) (Store, error) {
	scheme, rest, ok := strings.Cut(databaseURL, ":")
	if !ok {
		return nil, fmt.Errorf("invalid database url %q: missing scheme", databaseURL)
	}
	switch scheme {
	case "memory":
		return newMemory(), nil
	case "sqlite":
		if rest == "" {
			return nil, fmt.Errorf("sqlite url requires a path, e.g. sqlite:vexpay.db")
		}
		return openSQLite(rest)
	case "postgres", "postgresql":
		return nil, fmt.Errorf("postgres backend not implemented yet")
	default:
		return nil, fmt.Errorf("unsupported database scheme %q", scheme)
	}
}
