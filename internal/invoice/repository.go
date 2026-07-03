package invoice

import "context"

// Repository persists invoices. The concrete implementation is backed by the
// store (SQLite/Postgres); an in-memory version is used for tests and sandbox.
type Repository interface {
	// Create inserts a new invoice.
	Create(ctx context.Context, inv *Invoice) error
	// Get fetches an invoice by ID. Returns ErrNotFound if absent.
	Get(ctx context.Context, id string) (*Invoice, error)
	// Update persists changes to an existing invoice.
	Update(ctx context.Context, inv *Invoice) error
	// ListOpen returns all non-terminal invoices (for the watcher).
	ListOpen(ctx context.Context) ([]*Invoice, error)
	// ListByMerchant returns a merchant's invoices, newest first.
	ListByMerchant(ctx context.Context, merchantID string, limit, offset int) ([]*Invoice, error)
	// NextDerivationIndex returns the next per-merchant derivation index.
	NextDerivationIndex(ctx context.Context, merchantID string) (uint32, error)
}
