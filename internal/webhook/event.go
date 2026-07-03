package webhook

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/vexarnetwork/vexpay/internal/invoice"
)

// Event is the JSON body delivered to a merchant endpoint.
type Event struct {
	ID      string       `json:"id"`
	Type    string       `json:"type"`
	Created time.Time    `json:"created"`
	Data    invoice.View `json:"data"`
}

// NewEvent builds an event for an invoice status change, e.g. "invoice.paid".
func NewEvent(change invoice.StatusChange, now time.Time) Event {
	return Event{
		ID:      newEventID(),
		Type:    "invoice." + string(change.Invoice.Status),
		Created: now,
		Data:    invoice.NewView(change.Invoice),
	}
}

func newEventID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("vexpay: crypto/rand unavailable: " + err.Error())
	}
	return "evt_" + hex.EncodeToString(b[:])
}
