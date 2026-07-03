// Package chain defines the adapter interface every cryptocurrency plugs into.
//
// Adding a coin means implementing Adapter (plus a Backend that talks to an
// explorer or node) — the invoice engine, watcher, webhooks and API never
// change. Adapters are strictly watch-only: they derive receive targets from
// merchant-supplied public material and observe the chain. They never hold
// private keys.
package chain

import (
	"context"

	"github.com/vexarnetwork/vexpay/internal/money"
)

// ID identifies a chain/network an adapter serves, e.g. "bitcoin",
// "bitcoin-testnet", "mock".
type ID string

// ReceiveStrategy is how an invoice is matched to an on-chain payment without
// custody.
type ReceiveStrategy string

const (
	// StrategyAddressPerInvoice derives a fresh address per invoice (UTXO chains
	// with an xpub).
	StrategyAddressPerInvoice ReceiveStrategy = "address_per_invoice"
	// StrategyUniqueAmount uses a single merchant address and a unique amount
	// delta per invoice (account-based chains).
	StrategyUniqueAmount ReceiveStrategy = "unique_amount"
)

// WalletConfig is the watch-only material a merchant supplies for a chain.
// Which fields are required depends on the adapter (xpub for BTC, an address
// for account chains, a view key + primary address for Monero).
type WalletConfig struct {
	XPub           string
	Address        string
	ViewKey        string
	PrimaryAddress string
}

// ReceiveTarget is what a specific invoice should be paid to.
type ReceiveTarget struct {
	// Address the buyer sends funds to.
	Address string
	// ExactAmount is the amount the buyer must send. For unique-amount matching
	// this carries the per-invoice delta; otherwise it equals the requested
	// amount.
	ExactAmount money.Amount
	// Strategy used to produce this target (for display/debugging).
	Strategy ReceiveStrategy
	// DerivationIndex used, when applicable.
	DerivationIndex uint32
}

// Observation is what the chain currently shows for a receive target.
type Observation struct {
	// Seen is true once any matching payment has been detected.
	Seen bool
	// Received is the total amount observed at the target so far.
	Received money.Amount
	// Confirmations is the depth of the most relevant payment tx.
	Confirmations int
	// TxHash of the detected payment, when known.
	TxHash string
}

// Adapter is the watch-only interface a cryptocurrency implements.
type Adapter interface {
	// Chain identifies the network this adapter serves.
	Chain() ID
	// Asset is the currency this adapter receives.
	Asset() money.Asset
	// RequiredConfirmations is the confirmation depth before a payment is final.
	RequiredConfirmations() int
	// DeriveReceiveTarget produces the pay-to details for one invoice.
	DeriveReceiveTarget(cfg WalletConfig, invoiceIndex uint32, requested money.Amount) (ReceiveTarget, error)
	// CheckPayment asks the backend what has been received at a target.
	CheckPayment(ctx context.Context, target ReceiveTarget) (Observation, error)
	// PaymentURI builds a wallet-openable URI (BIP21 / EIP-681 / ...).
	PaymentURI(target ReceiveTarget) string
}
