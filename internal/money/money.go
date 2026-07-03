// Package money represents cryptocurrency amounts as exact integers in an
// asset's smallest unit (satoshi, wei, ...). Floating point is never used for
// balances or comparisons.
package money

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
)

// Asset describes a payable cryptocurrency unit.
type Asset struct {
	// Symbol is the ticker, e.g. "BTC", "USDT".
	Symbol string
	// Decimals is how many fractional digits map to one whole unit
	// (BTC: 8, ETH: 18, USDT: 6).
	Decimals int
}

// Amount is an exact quantity of an Asset, stored in the smallest unit.
// The zero value is not usable; construct with New, FromUnits, or ParseDecimal.
type Amount struct {
	asset Asset
	units *big.Int
}

// FromUnits builds an Amount from a count of smallest units.
func FromUnits(asset Asset, units *big.Int) Amount {
	return Amount{asset: asset, units: new(big.Int).Set(units)}
}

// Zero returns a zero Amount for the given asset.
func Zero(asset Asset) Amount {
	return Amount{asset: asset, units: new(big.Int)}
}

// ParseDecimal parses a human decimal string ("1.5", "0.00010000") into an
// Amount, scaling by the asset's decimals. It rejects values with more
// fractional digits than the asset supports rather than silently truncating.
func ParseDecimal(asset Asset, s string) (Amount, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Amount{}, errors.New("empty amount")
	}
	neg := false
	if strings.HasPrefix(s, "-") {
		neg = true
		s = s[1:]
	}

	intPart, fracPart, hasFrac := strings.Cut(s, ".")
	if hasFrac && len(fracPart) > asset.Decimals {
		return Amount{}, fmt.Errorf("amount %q has more than %d decimal places for %s", s, asset.Decimals, asset.Symbol)
	}

	digits := intPart + fracPart
	// Right-pad the fractional part up to the asset's precision.
	digits += strings.Repeat("0", asset.Decimals-len(fracPart))
	if digits == "" {
		return Amount{}, fmt.Errorf("invalid amount %q", s)
	}

	units, ok := new(big.Int).SetString(digits, 10)
	if !ok {
		return Amount{}, fmt.Errorf("invalid amount %q", s)
	}
	if neg {
		units.Neg(units)
	}
	return Amount{asset: asset, units: units}, nil
}

// Asset returns the amount's asset.
func (a Amount) Asset() Asset { return a.asset }

// Units returns a copy of the amount in smallest units.
func (a Amount) Units() *big.Int {
	if a.units == nil {
		return new(big.Int)
	}
	return new(big.Int).Set(a.units)
}

// IsZero reports whether the amount is exactly zero.
func (a Amount) IsZero() bool { return a.units == nil || a.units.Sign() == 0 }

// Cmp compares a and b, which must share an asset. It returns -1, 0, or +1.
func (a Amount) Cmp(b Amount) int {
	return a.mustUnits().Cmp(b.mustUnits())
}

// Add returns a+b. Both must share an asset.
func (a Amount) Add(b Amount) Amount {
	return Amount{asset: a.asset, units: new(big.Int).Add(a.mustUnits(), b.mustUnits())}
}

// Sub returns a-b. Both must share an asset.
func (a Amount) Sub(b Amount) Amount {
	return Amount{asset: a.asset, units: new(big.Int).Sub(a.mustUnits(), b.mustUnits())}
}

// String renders the amount as a decimal string with the asset's precision,
// e.g. "0.00010000" for 10000 satoshi.
func (a Amount) String() string {
	units := a.mustUnits()
	neg := units.Sign() < 0
	abs := new(big.Int).Abs(units)

	if a.asset.Decimals == 0 {
		s := abs.String()
		if neg {
			return "-" + s
		}
		return s
	}

	s := abs.String()
	if len(s) <= a.asset.Decimals {
		s = strings.Repeat("0", a.asset.Decimals-len(s)+1) + s
	}
	split := len(s) - a.asset.Decimals
	out := s[:split] + "." + s[split:]
	if neg {
		out = "-" + out
	}
	return out
}

func (a Amount) mustUnits() *big.Int {
	if a.units == nil {
		return new(big.Int)
	}
	return a.units
}
