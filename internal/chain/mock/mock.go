// Package mock provides an in-memory chain adapter for the sandbox. It lets the
// full payment pipeline be exercised — create invoice, receive, confirm, settle,
// webhook — without touching a real blockchain or spending any coins.
package mock

import (
	"context"
	"fmt"
	"sync"

	"github.com/vexarnetwork/vexpay/internal/chain"
	"github.com/vexarnetwork/vexpay/internal/money"
)

// ChainID is the sandbox chain identifier.
const ChainID chain.ID = "mock"

type entry struct {
	received      money.Amount
	confirmations int
	txHash        string
}

// Adapter is a watch-only mock chain backed by an in-memory ledger the sandbox
// simulator writes to. It is safe for concurrent use.
type Adapter struct {
	asset         money.Asset
	requiredConfs int

	mu     sync.RWMutex
	ledger map[string]entry
}

// New returns a mock adapter. requiredConfs controls how many confirmations an
// invoice needs before it settles (kept low for fast sandbox testing).
func New(requiredConfs int) *Adapter {
	if requiredConfs < 1 {
		requiredConfs = 1
	}
	return &Adapter{
		asset:         money.TBTC,
		requiredConfs: requiredConfs,
		ledger:        make(map[string]entry),
	}
}

func (a *Adapter) Chain() chain.ID            { return ChainID }
func (a *Adapter) Asset() money.Asset         { return a.asset }
func (a *Adapter) RequiredConfirmations() int { return a.requiredConfs }

// DeriveReceiveTarget hands out a deterministic sandbox address per invoice.
func (a *Adapter) DeriveReceiveTarget(_ chain.WalletConfig, invoiceIndex uint32, requested money.Amount) (chain.ReceiveTarget, error) {
	return chain.ReceiveTarget{
		Address:         fmt.Sprintf("sbx1q%08d", invoiceIndex),
		ExactAmount:     requested,
		Strategy:        chain.StrategyAddressPerInvoice,
		DerivationIndex: invoiceIndex,
	}, nil
}

// CheckPayment reports whatever the simulator has credited to the address.
func (a *Adapter) CheckPayment(_ context.Context, target chain.ReceiveTarget) (chain.Observation, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	e, ok := a.ledger[target.Address]
	if !ok {
		return chain.Observation{Seen: false, Received: money.Zero(a.asset)}, nil
	}
	return chain.Observation{
		Seen:          true,
		Received:      e.received,
		Confirmations: e.confirmations,
		TxHash:        e.txHash,
	}, nil
}

// PaymentURI returns a sandbox URI for display/QR.
func (a *Adapter) PaymentURI(target chain.ReceiveTarget) string {
	return fmt.Sprintf("sandbox:%s?amount=%s", target.Address, target.ExactAmount.String())
}

// Pay simulates a buyer paying `amount` to `address` with the given number of
// confirmations. Calling it again for the same address replaces the entry,
// which is how the simulator advances confirmations.
func (a *Adapter) Pay(address string, amount money.Amount, confirmations int, txHash string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ledger[address] = entry{
		received:      amount,
		confirmations: confirmations,
		txHash:        txHash,
	}
}
