package webhook

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vexarnetwork/vexpay/internal/invoice"
	"github.com/vexarnetwork/vexpay/internal/money"
)

func TestSignVerifyRoundTrip(t *testing.T) {
	secret := "whsec_test"
	body := []byte(`{"hello":"world"}`)
	ts := time.Now()

	header := Sign(secret, ts, body)
	if err := Verify(secret, header, body, time.Minute); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := Verify("wrong-secret", header, body, time.Minute); err == nil {
		t.Error("expected failure with wrong secret")
	}
	if err := Verify(secret, header, []byte(`tampered`), time.Minute); err == nil {
		t.Error("expected failure with tampered body")
	}
}

func TestVerifyRejectsOldTimestamp(t *testing.T) {
	secret := "whsec_test"
	body := []byte(`{}`)
	old := time.Now().Add(-10 * time.Minute)
	header := Sign(secret, old, body)
	if err := Verify(secret, header, body, time.Minute); err == nil {
		t.Error("expected failure for stale timestamp")
	}
}

func paidChange(t *testing.T) invoice.StatusChange {
	t.Helper()
	amt, _ := money.ParseDecimal(money.TBTC, "0.001")
	return invoice.StatusChange{
		Previous: invoice.StatusConfirming,
		Invoice: &invoice.Invoice{
			ID:         "inv_test",
			MerchantID: "m1",
			Chain:      "mock",
			Asset:      money.TBTC,
			Amount:     amt,
			Received:   amt,
			Status:     invoice.StatusPaid,
		},
	}
}

func TestDispatcherDeliversSignedEvent(t *testing.T) {
	secret := "whsec_abc"
	var got struct {
		verified atomic.Bool
		typ      atomic.Value
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := Verify(secret, r.Header.Get(SignatureHeader), body, time.Minute); err == nil {
			got.verified.Store(true)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := NewDispatcher(Options{
		Resolver: ResolverFunc(func(string) (Endpoint, bool) {
			return Endpoint{URL: srv.URL, Secret: secret}, true
		}),
	})
	d.Emit(context.Background(), paidChange(t))
	d.Close()

	if !got.verified.Load() {
		t.Fatal("endpoint did not receive a verifiable signed event")
	}
	_ = got.typ
}

func TestDispatcherRetriesUntilSuccess(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := NewDispatcher(Options{
		Resolver: ResolverFunc(func(string) (Endpoint, bool) {
			return Endpoint{URL: srv.URL, Secret: "s"}, true
		}),
		MaxAttempts: 5,
		Backoff:     func(int) time.Duration { return time.Millisecond },
	})
	d.Emit(context.Background(), paidChange(t))
	d.Close()

	if attempts.Load() < 3 {
		t.Fatalf("expected at least 3 attempts, got %d", attempts.Load())
	}
}
