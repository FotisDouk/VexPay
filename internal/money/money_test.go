package money

import (
	"math/big"
	"testing"
)

func TestParseDecimalRoundTrip(t *testing.T) {
	cases := []struct {
		asset     Asset
		in        string
		wantUnits int64
		wantStr   string
	}{
		{BTC, "0.0001", 10000, "0.00010000"},
		{BTC, "1", 100000000, "1.00000000"},
		{BTC, "1.23456789", 123456789, "1.23456789"},
		{USDT, "100", 100000000, "100.000000"},
		{USDT, "100.0037", 100003700, "100.003700"},
	}
	for _, c := range cases {
		got, err := ParseDecimal(c.asset, c.in)
		if err != nil {
			t.Fatalf("ParseDecimal(%s, %q): %v", c.asset.Symbol, c.in, err)
		}
		if got.Units().Cmp(big.NewInt(c.wantUnits)) != 0 {
			t.Errorf("ParseDecimal(%s, %q) units = %s, want %d", c.asset.Symbol, c.in, got.Units(), c.wantUnits)
		}
		if got.String() != c.wantStr {
			t.Errorf("ParseDecimal(%s, %q).String() = %q, want %q", c.asset.Symbol, c.in, got.String(), c.wantStr)
		}
	}
}

func TestParseDecimalTooManyPlaces(t *testing.T) {
	if _, err := ParseDecimal(USDT, "1.1234567"); err == nil {
		t.Fatal("expected error: USDT supports only 6 decimals")
	}
}

func TestAmountCompareAndArithmetic(t *testing.T) {
	a, _ := ParseDecimal(BTC, "0.5")
	b, _ := ParseDecimal(BTC, "0.3")

	if a.Cmp(b) != 1 {
		t.Errorf("0.5 should be greater than 0.3")
	}
	sum := a.Add(b)
	if sum.String() != "0.80000000" {
		t.Errorf("0.5 + 0.3 = %s, want 0.80000000", sum)
	}
	diff := a.Sub(b)
	if diff.String() != "0.20000000" {
		t.Errorf("0.5 - 0.3 = %s, want 0.20000000", diff)
	}
	if Zero(BTC).IsZero() != true {
		t.Errorf("Zero(BTC) should be zero")
	}
}
