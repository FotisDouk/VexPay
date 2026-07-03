package money

import "fmt"

// Well-known assets. This list grows as chain adapters are added.
var (
	BTC  = Asset{Symbol: "BTC", Decimals: 8}
	TBTC = Asset{Symbol: "tBTC", Decimals: 8} // Bitcoin testnet, priced as BTC
	ETH  = Asset{Symbol: "ETH", Decimals: 18}
	USDT = Asset{Symbol: "USDT", Decimals: 6}
	USDC = Asset{Symbol: "USDC", Decimals: 6}
)

var registry = map[string]Asset{
	BTC.Symbol:  BTC,
	TBTC.Symbol: TBTC,
	ETH.Symbol:  ETH,
	USDT.Symbol: USDT,
	USDC.Symbol: USDC,
}

// Lookup returns the asset for a symbol.
func Lookup(symbol string) (Asset, error) {
	a, ok := registry[symbol]
	if !ok {
		return Asset{}, fmt.Errorf("unknown asset %q", symbol)
	}
	return a, nil
}
