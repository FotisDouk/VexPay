package rates

import (
	"context"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/vexarnetwork/vexpay/internal/money"
)

type stubProvider struct {
	price *big.Rat
	err   error
	calls int
}

func (s *stubProvider) Price(context.Context, money.Asset, string) (*big.Rat, error) {
	s.calls++
	return s.price, s.err
}

func ratFrom(t *testing.T, s string) *big.Rat {
	t.Helper()
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		t.Fatalf("bad rat %q", s)
	}
	return r
}

func TestCryptoAmountRoundsUp(t *testing.T) {
	// BTC at 60000 EUR. 30 EUR -> 0.0005 BTC exactly.
	o := NewOracle(time.Minute, &stubProvider{price: ratFrom(t, "60000")})
	amt, rate, err := o.CryptoAmount(context.Background(), money.BTC, "EUR", "30")
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if amt.String() != "0.00050000" {
		t.Errorf("amount = %s, want 0.00050000", amt)
	}
	if rate != "60000.00000000" {
		t.Errorf("rate = %s, want 60000.00000000", rate)
	}

	// A price that doesn't divide evenly must round UP to the next satoshi so
	// the merchant is never underpaid.
	o2 := NewOracle(time.Minute, &stubProvider{price: ratFrom(t, "60000")})
	amt2, _, err := o2.CryptoAmount(context.Background(), money.BTC, "EUR", "1")
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	// 1/60000 BTC = 0.00001666... -> ceil to 0.00001667
	if amt2.String() != "0.00001667" {
		t.Errorf("amount = %s, want 0.00001667 (rounded up)", amt2)
	}
}

func TestCryptoAmountRejectsBadFiat(t *testing.T) {
	o := NewOracle(time.Minute, &stubProvider{price: ratFrom(t, "60000")})
	if _, _, err := o.CryptoAmount(context.Background(), money.BTC, "EUR", "-5"); err == nil {
		t.Error("expected error for negative fiat amount")
	}
}

func TestOracleFailsClosed(t *testing.T) {
	o := NewOracle(time.Minute, &stubProvider{err: errors.New("provider down")})
	if _, _, err := o.CryptoAmount(context.Background(), money.BTC, "EUR", "30"); err == nil {
		t.Fatal("expected error when no price is available (must fail closed)")
	}
}

func TestOracleFallsBackToSecondProvider(t *testing.T) {
	down := &stubProvider{err: errors.New("down")}
	up := &stubProvider{price: ratFrom(t, "50000")}
	o := NewOracle(time.Minute, down, up)

	amt, _, err := o.CryptoAmount(context.Background(), money.BTC, "USD", "100")
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if amt.String() != "0.00200000" { // 100/50000
		t.Errorf("amount = %s, want 0.00200000", amt)
	}
	if up.calls == 0 {
		t.Error("secondary provider should have been consulted")
	}
}

func TestOracleCachesWithinTTL(t *testing.T) {
	p := &stubProvider{price: ratFrom(t, "60000")}
	o := NewOracle(time.Minute, p)
	ctx := context.Background()

	_, _, _ = o.CryptoAmount(ctx, money.BTC, "EUR", "30")
	_, _, _ = o.CryptoAmount(ctx, money.BTC, "EUR", "60")
	if p.calls != 1 {
		t.Errorf("provider called %d times, want 1 (cached)", p.calls)
	}
}

func TestCoinGeckoParsesPrice(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("ids") != "bitcoin" || r.URL.Query().Get("vs_currencies") != "eur" {
			http.Error(w, "unexpected query", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"bitcoin":{"eur":60000.5}}`))
	}))
	defer srv.Close()

	cg := NewCoinGecko(srv.URL)
	price, err := cg.Price(context.Background(), money.BTC, "EUR")
	if err != nil {
		t.Fatalf("price: %v", err)
	}
	if price.FloatString(1) != "60000.5" {
		t.Errorf("price = %s, want 60000.5", price.FloatString(1))
	}
}
