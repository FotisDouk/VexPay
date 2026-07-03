package invoice

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/vexarnetwork/vexpay/internal/chain"
	"github.com/vexarnetwork/vexpay/internal/money"
)

// Emitter receives status-change notifications (implemented by the webhook
// dispatcher). A nil Emitter disables notifications.
type Emitter interface {
	Emit(ctx context.Context, change StatusChange)
}

// Service creates invoices and advances their lifecycle from chain
// observations.
type Service struct {
	repo     Repository
	registry *chain.Registry
	emitter  Emitter

	now    func() time.Time
	newID  func() string
	expiry time.Duration
}

// Options configures a Service.
type Options struct {
	Repo     Repository
	Registry *chain.Registry
	Emitter  Emitter
	Expiry   time.Duration
	// Now and NewID are injectable for tests; sensible defaults are used if nil.
	Now   func() time.Time
	NewID func() string
}

// NewService constructs a Service.
func NewService(opts Options) *Service {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	newID := opts.NewID
	if newID == nil {
		newID = func() string { return newInvoiceID() }
	}
	expiry := opts.Expiry
	if expiry <= 0 {
		expiry = 15 * time.Minute
	}
	return &Service{
		repo:     opts.Repo,
		registry: opts.Registry,
		emitter:  opts.Emitter,
		now:      now,
		newID:    newID,
		expiry:   expiry,
	}
}

// CreateParams describes a new invoice.
type CreateParams struct {
	MerchantID string
	Chain      chain.ID
	Wallet     chain.WalletConfig
	Amount     money.Amount

	// Optional fiat pricing captured for display (rate lock).
	FiatCurrency string
	FiatAmount   string
	Rate         string

	Metadata map[string]string
}

// Create derives a receive target and persists a new pending invoice.
func (s *Service) Create(ctx context.Context, p CreateParams) (*Invoice, error) {
	if p.MerchantID == "" {
		return nil, fmt.Errorf("merchant id is required")
	}
	adapter, err := s.registry.Get(p.Chain)
	if err != nil {
		return nil, err
	}
	if p.Amount.Asset() != adapter.Asset() {
		return nil, fmt.Errorf("amount asset %s does not match chain asset %s", p.Amount.Asset().Symbol, adapter.Asset().Symbol)
	}
	if p.Amount.IsZero() || p.Amount.Cmp(money.Zero(p.Amount.Asset())) < 0 {
		return nil, fmt.Errorf("amount must be positive")
	}

	idx, err := s.repo.NextDerivationIndex(ctx, p.MerchantID)
	if err != nil {
		return nil, err
	}
	target, err := adapter.DeriveReceiveTarget(p.Wallet, idx, p.Amount)
	if err != nil {
		return nil, fmt.Errorf("derive receive target: %w", err)
	}

	now := s.now()
	inv := &Invoice{
		ID:              s.newID(),
		MerchantID:      p.MerchantID,
		Chain:           p.Chain,
		Asset:           adapter.Asset(),
		Amount:          p.Amount,
		FiatCurrency:    p.FiatCurrency,
		FiatAmount:      p.FiatAmount,
		Rate:            p.Rate,
		ReceiveAddress:  target.Address,
		PaymentURI:      adapter.PaymentURI(target),
		Strategy:        target.Strategy,
		DerivationIndex: target.DerivationIndex,
		Status:          StatusPending,
		Received:        money.Zero(adapter.Asset()),
		RequiredConfs:   adapter.RequiredConfirmations(),
		Metadata:        p.Metadata,
		CreatedAt:       now,
		ExpiresAt:       now.Add(s.expiry),
		UpdatedAt:       now,
	}
	if err := s.repo.Create(ctx, inv); err != nil {
		return nil, err
	}
	return inv, nil
}

// Get returns an invoice by ID.
func (s *Service) Get(ctx context.Context, id string) (*Invoice, error) {
	return s.repo.Get(ctx, id)
}

// List returns a merchant's invoices, newest first.
func (s *Service) List(ctx context.Context, merchantID string, limit, offset int) ([]*Invoice, error) {
	return s.repo.ListByMerchant(ctx, merchantID, limit, offset)
}

// Sync fetches the latest chain observation for one invoice, applies the state
// machine, persists any change, and emits a status-change event. It is
// idempotent: with no on-chain change, nothing is emitted.
func (s *Service) Sync(ctx context.Context, inv *Invoice) (bool, error) {
	if inv.Status.IsTerminal() {
		return false, nil
	}

	adapter, err := s.registry.Get(inv.Chain)
	if err != nil {
		return false, err
	}
	target := chain.ReceiveTarget{
		Address:         inv.ReceiveAddress,
		ExactAmount:     inv.Amount,
		Strategy:        inv.Strategy,
		DerivationIndex: inv.DerivationIndex,
	}
	obs, err := adapter.CheckPayment(ctx, target)
	if err != nil {
		return false, fmt.Errorf("check payment: %w", err)
	}

	now := s.now()
	prev := inv.Status
	next := s.decide(inv, obs, now)

	// Record what we observed regardless of whether status moved.
	if obs.Seen {
		inv.Received = obs.Received
		inv.Confirmations = obs.Confirmations
		if obs.TxHash != "" {
			inv.TxHash = obs.TxHash
		}
	}
	inv.UpdatedAt = now

	changed := next != prev
	if changed {
		inv.Status = next
		if next.IsSettled() && inv.PaidAt == nil {
			t := now
			inv.PaidAt = &t
		}
	}

	if err := s.repo.Update(ctx, inv); err != nil {
		return false, err
	}
	if changed && s.emitter != nil {
		// Emit an immutable snapshot: the caller keeps mutating inv on later
		// syncs, and webhook delivery may be asynchronous.
		snapshot := *inv
		s.emitter.Emit(ctx, StatusChange{Invoice: &snapshot, Previous: prev})
	}
	return changed, nil
}

// decide computes the next status from the current invoice and a fresh
// observation. It fails closed: uncertainty never yields a paid state.
func (s *Service) decide(inv *Invoice, obs chain.Observation, now time.Time) Status {
	requested := inv.Amount
	expired := now.After(inv.ExpiresAt)

	if !obs.Seen || obs.Received.IsZero() {
		if expired {
			return StatusExpired
		}
		return StatusPending
	}

	switch cmp := obs.Received.Cmp(requested); {
	case cmp < 0: // underpaid
		if expired {
			return StatusExpired
		}
		return StatusUnderpaid
	default: // met or exceeded the requested amount
		if obs.Confirmations < inv.RequiredConfs {
			return StatusConfirming
		}
		if cmp > 0 {
			return StatusOverpaid
		}
		return StatusPaid
	}
}

// ProcessOpen syncs every open invoice once. Used by the watcher on each tick.
func (s *Service) ProcessOpen(ctx context.Context) error {
	open, err := s.repo.ListOpen(ctx)
	if err != nil {
		return err
	}
	for _, inv := range open {
		if _, err := s.Sync(ctx, inv); err != nil {
			// One bad invoice must not stall the rest.
			log.Printf("invoice sync failed id=%s: %v", inv.ID, err)
		}
	}
	return nil
}
