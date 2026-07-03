// Package rates converts fiat prices into exact crypto amounts using pluggable
// price providers. It fails closed: if no fresh price is available, invoice
// creation errors rather than guessing a rate.
package rates

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/vexarnetwork/vexpay/internal/money"
)

// Provider returns the price of one unit of asset expressed in the given fiat
// currency (e.g. BTC -> 60000 EUR).
type Provider interface {
	Price(ctx context.Context, asset money.Asset, fiat string) (*big.Rat, error)
}

type cacheEntry struct {
	price *big.Rat
	at    time.Time
}

// Oracle caches prices for a TTL and consults providers in order, so a
// secondary provider covers a primary outage. It implements invoice pricing.
type Oracle struct {
	providers []Provider
	ttl       time.Duration
	now       func() time.Time

	mu    sync.Mutex
	cache map[string]cacheEntry
}

// NewOracle builds an Oracle. Providers are tried in order until one succeeds.
func NewOracle(ttl time.Duration, providers ...Provider) *Oracle {
	if ttl <= 0 {
		ttl = time.Minute
	}
	return &Oracle{
		providers: providers,
		ttl:       ttl,
		now:       time.Now,
		cache:     make(map[string]cacheEntry),
	}
}

// price returns a fresh (cached-within-TTL) price for asset/fiat.
func (o *Oracle) price(ctx context.Context, asset money.Asset, fiat string) (*big.Rat, error) {
	fiat = strings.ToLower(fiat)
	key := asset.Symbol + "/" + fiat

	o.mu.Lock()
	if e, ok := o.cache[key]; ok && o.now().Sub(e.at) < o.ttl {
		o.mu.Unlock()
		return new(big.Rat).Set(e.price), nil
	}
	o.mu.Unlock()

	var lastErr error
	for _, p := range o.providers {
		price, err := p.Price(ctx, asset, fiat)
		if err != nil {
			lastErr = err
			continue
		}
		if price.Sign() <= 0 {
			lastErr = fmt.Errorf("provider returned non-positive price")
			continue
		}
		o.mu.Lock()
		o.cache[key] = cacheEntry{price: new(big.Rat).Set(price), at: o.now()}
		o.mu.Unlock()
		return price, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no price providers configured")
	}
	return nil, fmt.Errorf("no rate available for %s/%s: %w", asset.Symbol, strings.ToUpper(fiat), lastErr)
}

// CryptoAmount converts a fiat amount into the crypto asset, rounding up to the
// asset's smallest unit so the merchant is never underpaid due to rounding. It
// returns the amount and the locked rate (fiat per one crypto unit) as a string.
func (o *Oracle) CryptoAmount(ctx context.Context, asset money.Asset, fiatCurrency, fiatAmount string) (money.Amount, string, error) {
	price, err := o.price(ctx, asset, fiatCurrency)
	if err != nil {
		return money.Amount{}, "", err
	}
	fiat, ok := new(big.Rat).SetString(strings.TrimSpace(fiatAmount))
	if !ok || fiat.Sign() <= 0 {
		return money.Amount{}, "", fmt.Errorf("invalid fiat amount %q", fiatAmount)
	}

	// units = ceil( (fiat / price) * 10^decimals )
	scale := new(big.Rat).SetInt(pow10(asset.Decimals))
	value := new(big.Rat).Quo(fiat, price)
	value.Mul(value, scale)
	units := ceilRat(value)

	return money.FromUnits(asset, units), price.FloatString(8), nil
}

func pow10(n int) *big.Int {
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil)
}

// ceilRat returns the smallest integer >= r.
func ceilRat(r *big.Rat) *big.Int {
	num := new(big.Int).Set(r.Num())
	den := r.Denom()
	q, m := new(big.Int).QuoRem(num, den, new(big.Int))
	if m.Sign() > 0 {
		q.Add(q, big.NewInt(1))
	}
	return q
}
