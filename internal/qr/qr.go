// Package qr renders payment URIs as QR code images so buyers can scan an
// invoice with any wallet.
package qr

import (
	"fmt"

	qrcode "github.com/skip2/go-qrcode"
)

const (
	minSize     = 64
	maxSize     = 1024
	defaultSize = 256
)

// PNG encodes content as a PNG QR code of the given pixel size. The size is
// clamped to a sane range. Medium error correction balances density and
// scan reliability for payment URIs.
func PNG(content string, size int) ([]byte, error) {
	if content == "" {
		return nil, fmt.Errorf("qr: empty content")
	}
	switch {
	case size <= 0:
		size = defaultSize
	case size < minSize:
		size = minSize
	case size > maxSize:
		size = maxSize
	}
	return qrcode.Encode(content, qrcode.Medium, size)
}
