package invoice

import (
	"context"
	"sort"
	"sync"
)

// MemRepository is an in-memory Repository for tests and the sandbox. Invoices
// are deep-enough copied on the way in and out so callers can't mutate stored
// state by accident.
type MemRepository struct {
	mu    sync.RWMutex
	byID  map[string]Invoice
	seq   map[string]uint32 // per-merchant derivation counter
	order []string          // insertion order for stable listing
}

// NewMemRepository returns an empty in-memory repository.
func NewMemRepository() *MemRepository {
	return &MemRepository{
		byID: make(map[string]Invoice),
		seq:  make(map[string]uint32),
	}
}

func (r *MemRepository) Create(_ context.Context, inv *Invoice) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[inv.ID] = *inv
	r.order = append(r.order, inv.ID)
	return nil
}

func (r *MemRepository) Get(_ context.Context, id string) (*Invoice, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	inv, ok := r.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := inv
	return &cp, nil
}

func (r *MemRepository) Update(_ context.Context, inv *Invoice) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[inv.ID]; !ok {
		return ErrNotFound
	}
	r.byID[inv.ID] = *inv
	return nil
}

func (r *MemRepository) ListOpen(_ context.Context) ([]*Invoice, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*Invoice
	for _, id := range r.order {
		inv := r.byID[id]
		if !inv.Status.IsTerminal() {
			cp := inv
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *MemRepository) ListByMerchant(_ context.Context, merchantID string, limit, offset int) ([]*Invoice, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var all []Invoice
	for _, id := range r.order {
		if inv := r.byID[id]; inv.MerchantID == merchantID {
			all = append(all, inv)
		}
	}
	// Newest first.
	sort.SliceStable(all, func(i, j int) bool { return all[i].CreatedAt.After(all[j].CreatedAt) })

	if offset > len(all) {
		offset = len(all)
	}
	all = all[offset:]
	if limit > 0 && limit < len(all) {
		all = all[:limit]
	}
	out := make([]*Invoice, len(all))
	for i := range all {
		cp := all[i]
		out[i] = &cp
	}
	return out, nil
}

func (r *MemRepository) NextDerivationIndex(_ context.Context, merchantID string) (uint32, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	idx := r.seq[merchantID]
	r.seq[merchantID] = idx + 1
	return idx, nil
}
