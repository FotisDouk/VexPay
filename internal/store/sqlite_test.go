package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/vexarnetwork/vexpay/internal/chain"
	"github.com/vexarnetwork/vexpay/internal/invoice"
	"github.com/vexarnetwork/vexpay/internal/money"
)

func sampleInvoice(id string) *invoice.Invoice {
	amount, _ := money.ParseDecimal(money.BTC, "0.01")
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	return &invoice.Invoice{
		ID:             id,
		MerchantID:     "m1",
		Chain:          chain.ID("bitcoin"),
		Asset:          money.BTC,
		Amount:         amount,
		ReceiveAddress: "bc1qexample",
		PaymentURI:     "bitcoin:bc1qexample?amount=0.01000000",
		Strategy:       chain.StrategyAddressPerInvoice,
		Status:         invoice.StatusPending,
		Received:       money.Zero(money.BTC),
		RequiredConfs:  2,
		Metadata:       map[string]string{"order": "42"},
		CreatedAt:      now,
		ExpiresAt:      now.Add(15 * time.Minute),
		UpdatedAt:      now,
	}
}

func TestSQLitePersistsAcrossReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "vexpay.db")

	st, err := Open("sqlite:" + path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	repo := st.Invoices()

	inv := sampleInvoice("inv_persist")
	if err := repo.Create(ctx, inv); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Mutate it the way the service would.
	paid, _ := money.ParseDecimal(money.BTC, "0.01")
	inv.Status = invoice.StatusPaid
	inv.Received = paid
	inv.Confirmations = 2
	inv.TxHash = "deadbeef"
	pt := inv.CreatedAt.Add(time.Minute)
	inv.PaidAt = &pt
	inv.UpdatedAt = pt
	if err := repo.Update(ctx, inv); err != nil {
		t.Fatalf("update: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Reopen a fresh store over the same file.
	st2, err := Open("sqlite:" + path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer st2.Close()

	got, err := st2.Invoices().Get(ctx, "inv_persist")
	if err != nil {
		t.Fatalf("get after reopen: %v", err)
	}
	if got.Status != invoice.StatusPaid {
		t.Errorf("status = %s, want paid", got.Status)
	}
	if got.Received.String() != "0.01000000" {
		t.Errorf("received = %s, want 0.01000000", got.Received)
	}
	if got.TxHash != "deadbeef" {
		t.Errorf("txhash = %s, want deadbeef", got.TxHash)
	}
	if got.PaidAt == nil {
		t.Error("paid_at should be set")
	}
	if got.Metadata["order"] != "42" {
		t.Errorf("metadata not preserved: %+v", got.Metadata)
	}
	if got.Amount.String() != "0.01000000" {
		t.Errorf("amount = %s, want 0.01000000", got.Amount)
	}
}

func TestSQLiteDerivationIndexMonotonic(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "seq.db")
	st, err := Open("sqlite:" + path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer st.Close()
	repo := st.Invoices()

	for want := uint32(0); want < 3; want++ {
		got, err := repo.NextDerivationIndex(ctx, "m1")
		if err != nil {
			t.Fatalf("next index: %v", err)
		}
		if got != want {
			t.Fatalf("index = %d, want %d", got, want)
		}
	}
	// Separate merchant starts fresh.
	if got, _ := repo.NextDerivationIndex(ctx, "m2"); got != 0 {
		t.Fatalf("m2 first index = %d, want 0", got)
	}
}

func TestSQLiteListOpenExcludesTerminal(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "open.db")
	st, _ := Open("sqlite:" + path)
	defer st.Close()
	repo := st.Invoices()

	open := sampleInvoice("inv_open")
	_ = repo.Create(ctx, open)

	done := sampleInvoice("inv_done")
	done.Status = invoice.StatusPaid
	_ = repo.Create(ctx, done)

	list, err := repo.ListOpen(ctx)
	if err != nil {
		t.Fatalf("list open: %v", err)
	}
	if len(list) != 1 || list[0].ID != "inv_open" {
		t.Fatalf("ListOpen = %+v, want only inv_open", list)
	}
}
