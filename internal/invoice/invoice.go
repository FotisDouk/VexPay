// Package invoice implements the payment lifecycle: creating invoices, locking a
// receive target, and advancing each invoice's status from on-chain
// observations. It is chain-agnostic — all coin specifics live behind the
// chain.Adapter interface.
package invoice

import (
	"time"

	"github.com/vexarnetwork/vexpay/internal/chain"
	"github.com/vexarnetwork/vexpay/internal/money"
)

// Status is the lifecycle state of an invoice.
type Status string

const (
	// StatusPending: created, awaiting payment.
	StatusPending Status = "pending"
	// StatusUnderpaid: a payment arrived but for less than requested.
	StatusUnderpaid Status = "underpaid"
	// StatusConfirming: sufficient funds seen, waiting for confirmations.
	StatusConfirming Status = "confirming"
	// StatusPaid: fully paid and confirmed.
	StatusPaid Status = "paid"
	// StatusOverpaid: confirmed, but more than requested was sent.
	StatusOverpaid Status = "overpaid"
	// StatusExpired: the payment window closed before settlement.
	StatusExpired Status = "expired"
)

// IsTerminal reports whether a status will never change again.
func (s Status) IsTerminal() bool {
	switch s {
	case StatusPaid, StatusOverpaid, StatusExpired:
		return true
	default:
		return false
	}
}

// IsSettled reports whether the invoice can be considered successfully paid.
func (s Status) IsSettled() bool {
	return s == StatusPaid || s == StatusOverpaid
}

// Invoice is a single request for payment.
type Invoice struct {
	ID         string
	MerchantID string

	Chain chain.ID
	Asset money.Asset

	// Amount is the crypto amount requested.
	Amount money.Amount

	// Optional fiat pricing captured at creation (rate locked for the window).
	FiatCurrency string
	FiatAmount   string
	Rate         string

	// Receive target details.
	ReceiveAddress  string
	PaymentURI      string
	Strategy        chain.ReceiveStrategy
	DerivationIndex uint32

	Status        Status
	Received      money.Amount
	Confirmations int
	RequiredConfs int
	TxHash        string

	Metadata map[string]string

	CreatedAt time.Time
	ExpiresAt time.Time
	PaidAt    *time.Time
	UpdatedAt time.Time
}

// StatusChange describes a transition, emitted to subscribers (e.g. webhooks).
type StatusChange struct {
	Invoice  *Invoice
	Previous Status
}
