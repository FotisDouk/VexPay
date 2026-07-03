package btc

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"time"
)

// AddressStatus is what a backend reports for a watched address.
type AddressStatus struct {
	// ReceivedSats is the total received (confirmed + mempool), in satoshi.
	ReceivedSats *big.Int
	// Confirmations of the most recent paying transaction (0 if unconfirmed).
	Confirmations int
	// TxID of the most recent paying transaction.
	TxID string
}

// Backend observes Bitcoin addresses. The default implementation talks to a
// mempool.space-compatible REST API; a self-hosted instance or Electrs/Esplora
// server can be substituted by pointing BaseURL at it.
type Backend interface {
	AddressStatus(ctx context.Context, address string) (AddressStatus, error)
}

// MempoolBackend implements Backend against a mempool.space / Esplora API.
type MempoolBackend struct {
	BaseURL string
	Client  *http.Client
}

// NewMempoolBackend returns a backend for the given API base URL, e.g.
// "https://mempool.space" (mainnet) or "https://mempool.space/testnet".
func NewMempoolBackend(baseURL string) *MempoolBackend {
	return &MempoolBackend{
		BaseURL: baseURL,
		Client:  &http.Client{Timeout: 15 * time.Second},
	}
}

type esploraStats struct {
	FundedTxoSum int64 `json:"funded_txo_sum"`
}

type esploraAddress struct {
	ChainStats   esploraStats `json:"chain_stats"`
	MempoolStats esploraStats `json:"mempool_stats"`
}

type esploraTxVout struct {
	ScriptpubkeyAddress string `json:"scriptpubkey_address"`
}

type esploraTxStatus struct {
	Confirmed   bool  `json:"confirmed"`
	BlockHeight int64 `json:"block_height"`
}

type esploraTx struct {
	TxID   string          `json:"txid"`
	Vout   []esploraTxVout `json:"vout"`
	Status esploraTxStatus `json:"status"`
}

// AddressStatus fetches the funded total, then resolves confirmations from the
// most recent transaction paying the address.
func (b *MempoolBackend) AddressStatus(ctx context.Context, address string) (AddressStatus, error) {
	var addr esploraAddress
	if err := b.getJSON(ctx, "/api/address/"+address, &addr); err != nil {
		return AddressStatus{}, err
	}
	total := addr.ChainStats.FundedTxoSum + addr.MempoolStats.FundedTxoSum
	status := AddressStatus{ReceivedSats: big.NewInt(total)}
	if total == 0 {
		return status, nil
	}

	tip, err := b.tipHeight(ctx)
	if err != nil {
		return status, err
	}

	var txs []esploraTx
	if err := b.getJSON(ctx, "/api/address/"+address+"/txs", &txs); err != nil {
		return status, err
	}
	// txs are newest-first; take the most recent that pays this address.
	for _, tx := range txs {
		if !paysAddress(tx, address) {
			continue
		}
		status.TxID = tx.TxID
		if tx.Status.Confirmed {
			status.Confirmations = int(tip - tx.Status.BlockHeight + 1)
			if status.Confirmations < 0 {
				status.Confirmations = 0
			}
		}
		break
	}
	return status, nil
}

func paysAddress(tx esploraTx, address string) bool {
	for _, v := range tx.Vout {
		if v.ScriptpubkeyAddress == address {
			return true
		}
	}
	return false
}

func (b *MempoolBackend) tipHeight(ctx context.Context) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.BaseURL+"/api/blocks/tip/height", nil)
	if err != nil {
		return 0, err
	}
	resp, err := b.client().Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("tip height: status %d", resp.StatusCode)
	}
	var height int64
	if err := json.NewDecoder(resp.Body).Decode(&height); err != nil {
		return 0, fmt.Errorf("tip height decode: %w", err)
	}
	return height, nil
}

func (b *MempoolBackend) getJSON(ctx context.Context, path string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.BaseURL+path, nil)
	if err != nil {
		return err
	}
	resp, err := b.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: status %d", path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

func (b *MempoolBackend) client() *http.Client {
	if b.Client != nil {
		return b.Client
	}
	return http.DefaultClient
}
