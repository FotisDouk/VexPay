package invoice

import (
	"context"
	"testing"
	"time"

	"github.com/vexarnetwork/vexpay/internal/chain"
	"github.com/vexarnetwork/vexpay/internal/chain/mock"
	"github.com/vexarnetwork/vexpay/internal/money"
)

type captureEmitter struct {
	changes []StatusChange
}

func (c *captureEmitter) Emit(_ context.Context, ch StatusChange) {
	c.changes = append(c.changes, ch)
}

func newTestService(t *testing.T, requiredConfs int, now func() time.Time) (*Service, *mock.Adapter, *captureEmitter) {
	t.Helper()
	reg := chain.NewRegistry()
	adapter := mock.New(requiredConfs)
	if err := reg.Register(adapter); err != nil {
		t.Fatalf("register: %v", err)
	}
	em := &captureEmitter{}
	svc := NewService(Options{
		Repo:     NewMemRepository(),
		Registry: reg,
		Emitter:  em,
		Expiry:   15 * time.Minute,
		Now:      now,
	})
	return svc, adapter, em
}

func mustAmount(t *testing.T, s string) money.Amount {
	t.Helper()
	a, err := money.ParseDecimal(money.TBTC, s)
	if err != nil {
		t.Fatalf("amount: %v", err)
	}
	return a
}

func TestLifecycleUnderpaidToConfirmingToPaid(t *testing.T) {
	ctx := context.Background()
	clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	svc, adapter, em := newTestService(t, 2, func() time.Time { return clock })

	inv, err := svc.Create(ctx, CreateParams{
		MerchantID: "m1",
		Chain:      mock.ChainID,
		Amount:     mustAmount(t, "0.001"),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if inv.Status != StatusPending {
		t.Fatalf("initial status = %s, want pending", inv.Status)
	}
	addr := inv.ReceiveAddress

	// No payment yet.
	assertSync(t, svc, ctx, inv, StatusPending, false)

	// Underpaid.
	adapter.Pay(addr, mustAmount(t, "0.0005"), 1, "tx1")
	assertSync(t, svc, ctx, inv, StatusUnderpaid, true)

	// Full amount but not enough confirmations.
	adapter.Pay(addr, mustAmount(t, "0.001"), 1, "tx2")
	assertSync(t, svc, ctx, inv, StatusConfirming, true)

	// Enough confirmations -> paid.
	adapter.Pay(addr, mustAmount(t, "0.001"), 2, "tx2")
	assertSync(t, svc, ctx, inv, StatusPaid, true)
	if inv.PaidAt == nil {
		t.Error("PaidAt should be set once paid")
	}

	// Idempotent: syncing a terminal invoice does nothing.
	changed, err := svc.Sync(ctx, inv)
	if err != nil {
		t.Fatalf("sync terminal: %v", err)
	}
	if changed {
		t.Error("terminal invoice should not change")
	}

	wantStatuses := []Status{StatusUnderpaid, StatusConfirming, StatusPaid}
	if len(em.changes) != len(wantStatuses) {
		t.Fatalf("emitted %d changes, want %d", len(em.changes), len(wantStatuses))
	}
	for i, want := range wantStatuses {
		if em.changes[i].Invoice.Status != want {
			t.Errorf("change %d status = %s, want %s", i, em.changes[i].Invoice.Status, want)
		}
	}
}

type stubPricer struct {
	amount money.Amount
	rate   string
}

func (s stubPricer) CryptoAmount(_ context.Context, _ money.Asset, _, _ string) (money.Amount, string, error) {
	return s.amount, s.rate, nil
}

func TestFiatPricedInvoice(t *testing.T) {
	ctx := context.Background()
	clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	reg := chain.NewRegistry()
	if err := reg.Register(mock.New(1)); err != nil {
		t.Fatalf("register: %v", err)
	}
	svc := NewService(Options{
		Repo:     NewMemRepository(),
		Registry: reg,
		Pricer:   stubPricer{amount: mustAmount(t, "0.0005"), rate: "60000.00"},
		Now:      func() time.Time { return clock },
	})

	inv, err := svc.Create(ctx, CreateParams{
		MerchantID:   "m1",
		Chain:        mock.ChainID,
		FiatCurrency: "EUR",
		FiatAmount:   "30",
	})
	if err != nil {
		t.Fatalf("create fiat invoice: %v", err)
	}
	if inv.Amount.String() != "0.00050000" {
		t.Errorf("amount = %s, want 0.00050000", inv.Amount)
	}
	if inv.Rate != "60000.00" {
		t.Errorf("rate = %s, want 60000.00", inv.Rate)
	}
	if inv.FiatCurrency != "EUR" || inv.FiatAmount != "30" {
		t.Errorf("fiat fields not preserved: %+v", inv)
	}
}

func TestCreateRequiresAmountOrFiat(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newTestService(t, 1, func() time.Time { return time.Now() })
	if _, err := svc.Create(ctx, CreateParams{MerchantID: "m1", Chain: mock.ChainID}); err == nil {
		t.Fatal("expected error when neither amount nor fiat price is given")
	}
}

func TestOverpaidSettles(t *testing.T) {
	ctx := context.Background()
	clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	svc, adapter, _ := newTestService(t, 1, func() time.Time { return clock })

	inv, _ := svc.Create(ctx, CreateParams{MerchantID: "m1", Chain: mock.ChainID, Amount: mustAmount(t, "0.001")})
	adapter.Pay(inv.ReceiveAddress, mustAmount(t, "0.0015"), 1, "tx")
	assertSync(t, svc, ctx, inv, StatusOverpaid, true)
	if !inv.Status.IsSettled() {
		t.Error("overpaid should be settled")
	}
}

func TestExpiryWithoutPayment(t *testing.T) {
	ctx := context.Background()
	clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	svc, _, _ := newTestService(t, 1, func() time.Time { return clock })

	inv, _ := svc.Create(ctx, CreateParams{MerchantID: "m1", Chain: mock.ChainID, Amount: mustAmount(t, "0.001")})

	// Advance the clock past expiry.
	clock = clock.Add(20 * time.Minute)
	assertSync(t, svc, ctx, inv, StatusExpired, true)
	if !inv.Status.IsTerminal() {
		t.Error("expired should be terminal")
	}
}

func assertSync(t *testing.T, svc *Service, ctx context.Context, inv *Invoice, want Status, wantChanged bool) {
	t.Helper()
	changed, err := svc.Sync(ctx, inv)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if changed != wantChanged {
		t.Errorf("changed = %v, want %v (status now %s)", changed, wantChanged, inv.Status)
	}
	if inv.Status != want {
		t.Errorf("status = %s, want %s", inv.Status, want)
	}
}
