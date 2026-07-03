// Package btc implements a watch-only Bitcoin chain adapter. Merchants supply an
// account-level extended public key (xpub/zpub); the adapter derives a fresh
// native-SegWit receive address per invoice at m/0/index and observes payments
// through a block explorer backend. No private keys are ever involved.
package btc

import (
	"context"
	"fmt"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"

	"github.com/vexarnetwork/vexpay/internal/chain"
	"github.com/vexarnetwork/vexpay/internal/money"
)

// Network bundles the chain identity, asset and address parameters for a
// Bitcoin network.
type Network struct {
	ID     chain.ID
	Asset  money.Asset
	Params *chaincfg.Params
}

// Supported networks.
var (
	Mainnet = Network{ID: "bitcoin", Asset: money.BTC, Params: &chaincfg.MainNetParams}
	Testnet = Network{ID: "bitcoin-testnet", Asset: money.TBTC, Params: &chaincfg.TestNet3Params}
)

// Adapter is the Bitcoin chain.Adapter.
type Adapter struct {
	net           Network
	backend       Backend
	requiredConfs int
}

var _ chain.Adapter = (*Adapter)(nil)

// New constructs a Bitcoin adapter. A non-positive requiredConfs defaults to 1
// for testnet and 2 for mainnet.
func New(net Network, backend Backend, requiredConfs int) *Adapter {
	if requiredConfs <= 0 {
		if net.ID == Mainnet.ID {
			requiredConfs = 2
		} else {
			requiredConfs = 1
		}
	}
	return &Adapter{net: net, backend: backend, requiredConfs: requiredConfs}
}

func (a *Adapter) Chain() chain.ID            { return a.net.ID }
func (a *Adapter) Asset() money.Asset         { return a.net.Asset }
func (a *Adapter) RequiredConfirmations() int { return a.requiredConfs }

// DeriveReceiveTarget derives the P2WPKH address at m/0/invoiceIndex from the
// merchant's account xpub.
func (a *Adapter) DeriveReceiveTarget(cfg chain.WalletConfig, invoiceIndex uint32, requested money.Amount) (chain.ReceiveTarget, error) {
	if cfg.XPub == "" {
		return chain.ReceiveTarget{}, fmt.Errorf("bitcoin wallet requires an xpub")
	}
	acct, err := hdkeychain.NewKeyFromString(cfg.XPub)
	if err != nil {
		return chain.ReceiveTarget{}, fmt.Errorf("parse xpub: %w", err)
	}
	external, err := acct.Derive(0)
	if err != nil {
		return chain.ReceiveTarget{}, fmt.Errorf("derive external branch: %w", err)
	}
	child, err := external.Derive(invoiceIndex)
	if err != nil {
		return chain.ReceiveTarget{}, fmt.Errorf("derive index %d: %w", invoiceIndex, err)
	}
	pub, err := child.ECPubKey()
	if err != nil {
		return chain.ReceiveTarget{}, fmt.Errorf("public key: %w", err)
	}
	addr, err := btcutil.NewAddressWitnessPubKeyHash(btcutil.Hash160(pub.SerializeCompressed()), a.net.Params)
	if err != nil {
		return chain.ReceiveTarget{}, fmt.Errorf("build address: %w", err)
	}
	return chain.ReceiveTarget{
		Address:         addr.EncodeAddress(),
		ExactAmount:     requested,
		Strategy:        chain.StrategyAddressPerInvoice,
		DerivationIndex: invoiceIndex,
	}, nil
}

// CheckPayment queries the backend for funds received at the target address.
func (a *Adapter) CheckPayment(ctx context.Context, target chain.ReceiveTarget) (chain.Observation, error) {
	st, err := a.backend.AddressStatus(ctx, target.Address)
	if err != nil {
		return chain.Observation{}, err
	}
	received := money.Zero(a.net.Asset)
	if st.ReceivedSats != nil {
		received = money.FromUnits(a.net.Asset, st.ReceivedSats)
	}
	return chain.Observation{
		Seen:          !received.IsZero(),
		Received:      received,
		Confirmations: st.Confirmations,
		TxHash:        st.TxID,
	}, nil
}

// PaymentURI returns a BIP21 URI, e.g. "bitcoin:bc1q...?amount=0.00250000".
func (a *Adapter) PaymentURI(target chain.ReceiveTarget) string {
	return fmt.Sprintf("bitcoin:%s?amount=%s", target.Address, target.ExactAmount.String())
}
