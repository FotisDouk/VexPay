package rates

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/vexarnetwork/vexpay/internal/money"
)

// coinGeckoIDs maps asset symbols to CoinGecko coin IDs.
var coinGeckoIDs = map[string]string{
	"BTC":  "bitcoin",
	"tBTC": "bitcoin",
	"ETH":  "ethereum",
	"USDT": "tether",
	"USDC": "usd-coin",
}

// CoinGecko is a Provider backed by the public CoinGecko simple-price API.
type CoinGecko struct {
	BaseURL string
	Client  *http.Client
}

// NewCoinGecko returns a CoinGecko provider. An empty baseURL uses the public
// API endpoint.
func NewCoinGecko(baseURL string) *CoinGecko {
	if baseURL == "" {
		baseURL = "https://api.coingecko.com"
	}
	return &CoinGecko{BaseURL: baseURL, Client: &http.Client{Timeout: 10 * time.Second}}
}

// Price implements Provider.
func (c *CoinGecko) Price(ctx context.Context, asset money.Asset, fiat string) (*big.Rat, error) {
	id, ok := coinGeckoIDs[asset.Symbol]
	if !ok {
		return nil, fmt.Errorf("no CoinGecko id for asset %s", asset.Symbol)
	}
	fiat = strings.ToLower(fiat)

	u := fmt.Sprintf("%s/api/v3/simple/price?ids=%s&vs_currencies=%s",
		c.BaseURL, url.QueryEscape(id), url.QueryEscape(fiat))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("coingecko: status %d", resp.StatusCode)
	}

	// Response: {"bitcoin":{"eur":60000.5}}. Use json.Number for exact parsing.
	var body map[string]map[string]json.Number
	dec := json.NewDecoder(resp.Body)
	dec.UseNumber()
	if err := dec.Decode(&body); err != nil {
		return nil, fmt.Errorf("coingecko decode: %w", err)
	}
	priceNum, ok := body[id][fiat]
	if !ok {
		return nil, fmt.Errorf("coingecko: no price for %s/%s", id, fiat)
	}
	price, ok := new(big.Rat).SetString(priceNum.String())
	if !ok {
		return nil, fmt.Errorf("coingecko: invalid price %q", priceNum.String())
	}
	return price, nil
}

func (c *CoinGecko) client() *http.Client {
	if c.Client != nil {
		return c.Client
	}
	return http.DefaultClient
}
