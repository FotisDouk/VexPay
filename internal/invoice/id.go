package invoice

import (
	"crypto/rand"
	"encoding/hex"
)

// newInvoiceID returns a collision-resistant, URL-safe invoice identifier.
func newInvoiceID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failing is fatal-adjacent; a zero id is never valid and
		// the caller's Create will still store it, but this path is effectively
		// unreachable on supported platforms.
		panic("vexpay: crypto/rand unavailable: " + err.Error())
	}
	return "inv_" + hex.EncodeToString(b[:])
}
